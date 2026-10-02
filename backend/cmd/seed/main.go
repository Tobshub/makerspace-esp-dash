package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/config"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/database"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/server"
)

const (
	demoEmail    = "demo@makerspace.local"
	demoPassword = "demo-password"
	demoTeam     = "Makerspace Demo"
	demoProject  = "Smart Greenhouse"
	demoDevice   = "Greenhouse ESP32"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	cfg := config.Load()
	if err := database.Migrate(cfg.DatabaseURL); err != nil {
		slog.Error("migrate failed", "err", err)
		os.Exit(1)
	}
	pool, err := database.Open(context.Background(), cfg.DatabaseURL)
	if err != nil || pool == nil {
		slog.Error("database unavailable", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	router := server.New(server.Deps{
		Config:     cfg,
		Pool:       pool,
		MQTTUp:     func() bool { return true },
		BrokerHost: "localhost",
		BrokerPort: 1883,
	})
	ts := httptest.NewServer(router)
	defer ts.Close()

	token := signupOrLogin(ts.URL)
	teamID := ensureTeam(ts.URL, token)
	projectID := ensureProject(ts.URL, token, teamID)
	if deviceExists(ts.URL, token, projectID) {
		fmt.Println("Demo data is already present.")
		fmt.Printf("Sign in as %s / %s\n", demoEmail, demoPassword)
		fmt.Println("The device secret was shown on the first seed and is not stored.")
		return
	}
	issued := post(ts.URL+"/api/v1/projects/"+projectID+"/devices", token, map[string]string{
		"name":        demoDevice,
		"description": "Bench greenhouse controller",
	})
	secret := issued["credentials"].(map[string]any)["secret"].(string)
	deviceKey := issued["device"].(map[string]any)["deviceKey"].(string)
	fmt.Println("Demo data created.")
	fmt.Printf("Sign in as %s / %s\n", demoEmail, demoPassword)
	fmt.Printf("Device key: %s\n", deviceKey)
	fmt.Printf("Device secret (shown once): %s\n", secret)
}

func signupOrLogin(base string) string {
	res, body := request(http.MethodPost, base+"/api/v1/auth/register", "", map[string]string{
		"email":    demoEmail,
		"name":     "Demo",
		"password": demoPassword,
	})
	if res.StatusCode == http.StatusCreated {
		return body["token"].(string)
	}
	res, body = request(http.MethodPost, base+"/api/v1/auth/login", "", map[string]string{
		"email":    demoEmail,
		"password": demoPassword,
	})
	if res.StatusCode != http.StatusOK {
		slog.Error("demo login failed", "status", res.StatusCode)
		os.Exit(1)
	}
	return body["token"].(string)
}

func ensureTeam(base, token string) string {
	_, body := request(http.MethodGet, base+"/api/v1/teams", token, nil)
	for _, item := range body["teams"].([]any) {
		team := item.(map[string]any)
		if team["name"] == demoTeam {
			return team["id"].(string)
		}
	}
	created := post(base+"/api/v1/teams", token, map[string]string{"name": demoTeam})
	return created["team"].(map[string]any)["id"].(string)
}

func ensureProject(base, token, teamID string) string {
	_, body := request(http.MethodGet, base+"/api/v1/teams/"+teamID+"/projects", token, nil)
	for _, item := range body["projects"].([]any) {
		project := item.(map[string]any)
		if project["name"] == demoProject {
			return project["id"].(string)
		}
	}
	created := post(base+"/api/v1/teams/"+teamID+"/projects", token, map[string]string{
		"name":        demoProject,
		"description": "Sample project from plan.md",
	})
	return created["project"].(map[string]any)["id"].(string)
}

func deviceExists(base, token, projectID string) bool {
	_, body := request(http.MethodGet, base+"/api/v1/projects/"+projectID+"/devices", token, nil)
	for _, item := range body["devices"].([]any) {
		if item.(map[string]any)["name"] == demoDevice {
			return true
		}
	}
	return false
}

func post(url, token string, payload any) map[string]any {
	res, body := request(http.MethodPost, url, token, payload)
	if res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusOK {
		slog.Error("seed request failed", "url", url, "status", res.StatusCode, "body", body)
		os.Exit(1)
	}
	return body
}

func request(method, url, token string, payload any) (*http.Response, map[string]any) {
	var reader io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			slog.Error("encode", "err", err)
			os.Exit(1)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		slog.Error("request", "err", err)
		os.Exit(1)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Error("request", "err", err)
		os.Exit(1)
	}
	defer res.Body.Close()
	var body map[string]any
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil && err != io.EOF {
		slog.Error("decode", "err", err)
		os.Exit(1)
	}
	return res, body
}
