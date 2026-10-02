package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/config"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/database"
)

func TestRegistryIsolation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pool := openPool(t)
	defer pool.Close()
	if err := database.Migrate(poolURL(t)); err != nil {
		t.Fatal(err)
	}

	router := New(Deps{
		Config:     config.Config{AppURL: "http://localhost:5173", DeviceOfflineTimeout: time.Minute, MQTTBrokerURL: "tcp://localhost:1883"},
		Pool:       pool,
		MQTTUp:     func() bool { return true },
		BrokerHost: "localhost",
		BrokerPort: 1883,
	})

	suffix := time.Now().UTC().Format("20060102150405.000")
	emailA := "phase-a-" + suffix + "@example.com"
	emailB := "phase-b-" + suffix + "@example.com"

	tokenA, userA := register(t, router, emailA, "Ada")
	tokenB, _ := register(t, router, emailB, "Grace")
	var teamID string
	t.Cleanup(func() {
		ctx := context.Background()
		if teamID != "" {
			_, _ = pool.Exec(ctx, `DELETE FROM teams WHERE id = $1`, database.ID(teamID))
		}
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE email = $1 OR email = $2`, emailA, emailB)
	})

	team := postJSON(t, router, "/api/v1/teams", tokenA, map[string]string{"name": "Phase Team " + suffix}, http.StatusCreated)
	teamID = team["team"].(map[string]any)["id"].(string)

	missing := getStatus(t, router, "/api/v1/teams/"+teamID, tokenB)
	if missing != http.StatusNotFound {
		t.Fatalf("other user team status %d", missing)
	}

	project := postJSON(t, router, "/api/v1/teams/"+teamID+"/projects", tokenA, map[string]string{
		"name":        "Greenhouse",
		"description": "Demo beds",
	}, http.StatusCreated)
	projectID := project["project"].(map[string]any)["id"].(string)

	if got := getStatus(t, router, "/api/v1/projects/"+projectID, tokenB); got != http.StatusNotFound {
		t.Fatalf("other user project status %d", got)
	}

	member := postJSON(t, router, "/api/v1/teams/"+teamID+"/members", tokenA, map[string]string{
		"email": emailB,
		"role":  "viewer",
	}, http.StatusCreated)
	if member["member"].(map[string]any)["role"] != "viewer" {
		t.Fatalf("member %#v", member)
	}

	if got := postStatus(t, router, "/api/v1/projects/"+projectID+"/devices", tokenB, map[string]string{"name": "Nope"}); got != http.StatusForbidden {
		t.Fatalf("viewer create status %d", got)
	}

	created := postJSON(t, router, "/api/v1/projects/"+projectID+"/devices", tokenA, map[string]string{
		"name":        "Greenhouse ESP32",
		"description": "Bench unit",
	}, http.StatusCreated)
	secret := created["credentials"].(map[string]any)["secret"].(string)
	device := created["device"].(map[string]any)
	deviceID := device["id"].(string)
	if secret == "" || device["deviceKey"] == "" {
		t.Fatal("credentials missing on create")
	}
	if strings.Contains(string(mustJSON(t, device)), secret) {
		t.Fatal("device object included the secret")
	}

	body := getBody(t, router, "/api/v1/devices/"+deviceID, tokenA)
	if strings.Contains(body, secret) {
		t.Fatal("get device returned the plaintext secret")
	}
	var hash string
	if err := pool.QueryRow(context.Background(), `SELECT secret_hash FROM devices WHERE id = $1`, database.ID(deviceID)).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if hash == secret || bcrypt.CompareHashAndPassword([]byte(hash), []byte(secret)) != nil {
		t.Fatal("secret was not stored as a matching hash")
	}

	rotated := postJSON(t, router, "/api/v1/devices/"+deviceID+"/rotate-secret", tokenA, map[string]string{}, http.StatusOK)
	next := rotated["credentials"].(map[string]any)["secret"].(string)
	if next == "" || next == secret {
		t.Fatal("rotation did not issue a new secret")
	}
	if strings.Contains(getBody(t, router, "/api/v1/devices/"+deviceID, tokenA), next) {
		t.Fatal("get device returned the rotated secret")
	}
	if err := pool.QueryRow(context.Background(), `SELECT secret_hash FROM devices WHERE id = $1`, database.ID(deviceID)).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(secret)) == nil {
		t.Fatal("old secret still matches")
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(next)) != nil {
		t.Fatal("new secret does not match the stored hash")
	}

	disabled := postJSON(t, router, "/api/v1/devices/"+deviceID+"/disable", tokenA, map[string]string{}, http.StatusOK)
	if disabled["device"].(map[string]any)["status"] != "disabled" {
		t.Fatalf("disable %#v", disabled)
	}

	ready := getBody(t, router, "/ready", "")
	if !strings.Contains(ready, `"database":"ok"`) || !strings.Contains(ready, `"mqtt":"ok"`) {
		t.Fatalf("ready %s", ready)
	}

	if userA == "" {
		t.Fatal("user id missing")
	}
}

func openPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, poolURL(t))
	if err != nil {
		t.Skip(err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skip(err)
	}
	return pool
}

func poolURL(t *testing.T) string {
	t.Helper()
	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		t.Skip("DATABASE_URL is empty")
	}
	return cfg.DatabaseURL
}

func register(t *testing.T, router http.Handler, email, name string) (string, string) {
	t.Helper()
	body := postJSON(t, router, "/api/v1/auth/register", "", map[string]string{
		"email":    email,
		"name":     name,
		"password": "test-password",
	}, http.StatusCreated)
	user := body["user"].(map[string]any)
	return body["token"].(string), user["id"].(string)
}

func postJSON(t *testing.T, router http.Handler, path, token string, payload any, want int) map[string]any {
	t.Helper()
	res := do(t, router, http.MethodPost, path, token, payload)
	if res.Code != want {
		t.Fatalf("%s %s: status %d body %s", http.MethodPost, path, res.Code, res.Body.String())
	}
	if res.Body.Len() == 0 {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func postStatus(t *testing.T, router http.Handler, path, token string, payload any) int {
	t.Helper()
	return do(t, router, http.MethodPost, path, token, payload).Code
}

func getStatus(t *testing.T, router http.Handler, path, token string) int {
	t.Helper()
	return do(t, router, http.MethodGet, path, token, nil).Code
}

func getBody(t *testing.T, router http.Handler, path, token string) string {
	t.Helper()
	res := do(t, router, http.MethodGet, path, token, nil)
	if res.Code != http.StatusOK {
		t.Fatalf("GET %s status %d body %s", path, res.Code, res.Body.String())
	}
	return res.Body.String()
}

func do(t *testing.T, router http.Handler, method, path, token string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	return res
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
