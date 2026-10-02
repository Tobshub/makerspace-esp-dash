package server

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/config"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/database"
)

func TestTelemetryQueries(t *testing.T) {
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
	emailA := "telemetry-a-" + suffix + "@example.com"
	emailB := "telemetry-b-" + suffix + "@example.com"
	tokenA, _ := register(t, router, emailA, "Ada")
	tokenB, _ := register(t, router, emailB, "Grace")
	var teamID string
	t.Cleanup(func() {
		ctx := context.Background()
		if teamID != "" {
			_, _ = pool.Exec(ctx, `DELETE FROM teams WHERE id = $1`, database.ID(teamID))
		}
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE email = $1 OR email = $2`, emailA, emailB)
	})

	team := postJSON(t, router, "/api/v1/teams", tokenA, map[string]string{"name": "Telemetry Team " + suffix}, http.StatusCreated)
	teamID = team["team"].(map[string]any)["id"].(string)
	project := postJSON(t, router, "/api/v1/teams/"+teamID+"/projects", tokenA, map[string]string{"name": "Beds"}, http.StatusCreated)
	projectID := project["project"].(map[string]any)["id"].(string)
	created := postJSON(t, router, "/api/v1/projects/"+projectID+"/devices", tokenA, map[string]string{"name": "Bench"}, http.StatusCreated)
	deviceID := created["device"].(map[string]any)["id"].(string)
	quiet := postJSON(t, router, "/api/v1/projects/"+projectID+"/devices", tokenA, map[string]string{"name": "Spare"}, http.StatusCreated)
	quietID := quiet["device"].(map[string]any)["id"].(string)

	postJSON(t, router, "/api/v1/teams/"+teamID+"/members", tokenA, map[string]string{
		"email": emailB,
		"role":  "viewer",
	}, http.StatusCreated)

	now := time.Now().UTC().Truncate(time.Millisecond)
	insertTelemetry(t, pool, projectID, deviceID, "temperature", f64(20), nil, nil, now.Add(-2*time.Hour))
	insertTelemetry(t, pool, projectID, deviceID, "temperature", f64(28.4), nil, nil, now.Add(-time.Minute))
	insertTelemetry(t, pool, projectID, deviceID, "temperature", f64(1), nil, nil, now.Add(-48*time.Hour))
	insertTelemetry(t, pool, projectID, deviceID, "pump_active", nil, boolp(true), nil, now.Add(-time.Minute))
	insertTelemetry(t, pool, projectID, deviceID, "mode", nil, nil, strp("automatic"), now.Add(-time.Minute))

	if got := getStatus(t, router, "/api/v1/devices/"+deviceID+"/telemetry/latest", ""); got != http.StatusUnauthorized {
		t.Fatalf("anonymous status %d", got)
	}
	other := postJSON(t, router, "/api/v1/auth/register", "", map[string]string{
		"email":    "telemetry-c-" + suffix + "@example.com",
		"name":     "Other",
		"password": "test-password",
	}, http.StatusCreated)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE email = $1`, "telemetry-c-"+suffix+"@example.com")
	})
	tokenC := other["token"].(string)
	if got := getStatus(t, router, "/api/v1/devices/"+deviceID+"/telemetry/latest", tokenC); got != http.StatusNotFound {
		t.Fatalf("other user latest %d", got)
	}
	if got := getStatus(t, router, "/api/v1/projects/"+projectID+"/telemetry/latest", tokenC); got != http.StatusNotFound {
		t.Fatalf("other user project %d", got)
	}

	latest := getJSON(t, router, "/api/v1/devices/"+deviceID+"/telemetry/latest", tokenB)
	metrics := latest["metrics"].([]any)
	if len(metrics) != 3 {
		t.Fatalf("latest metrics %#v", latest)
	}
	byKey := map[string]map[string]any{}
	for _, item := range metrics {
		row := item.(map[string]any)
		byKey[row["metric"].(string)] = row
	}
	if math.Abs(asFloat(t, byKey["temperature"]["numericValue"])-28.4) > 0.001 {
		t.Fatalf("temperature %#v", byKey["temperature"])
	}
	if byKey["pump_active"]["booleanValue"] != true {
		t.Fatalf("pump %#v", byKey["pump_active"])
	}
	if byKey["mode"]["stringValue"] != "automatic" {
		t.Fatalf("mode %#v", byKey["mode"])
	}

	series := getJSON(t, router, "/api/v1/devices/"+deviceID+"/telemetry?metric=temperature", tokenA)
	points := series["points"].([]any)
	if series["resolution"] != "raw" || series["truncated"] != false || len(points) != 2 {
		t.Fatalf("default series %#v", series)
	}
	if math.Abs(asFloat(t, points[0].(map[string]any)["numericValue"])-20) > 0.001 ||
		math.Abs(asFloat(t, points[1].(map[string]any)["numericValue"])-28.4) > 0.001 {
		t.Fatalf("order %#v", points)
	}

	from := url.QueryEscape(now.Add(-72 * time.Hour).Format(time.RFC3339))
	to := url.QueryEscape(now.Format(time.RFC3339))
	capped := getJSON(t, router, "/api/v1/devices/"+deviceID+"/telemetry?metric=temperature&from="+from+"&to="+to+"&limit=1&resolution=raw", tokenA)
	cappedPoints := capped["points"].([]any)
	if capped["truncated"] != true || len(cappedPoints) != 1 || math.Abs(asFloat(t, cappedPoints[0].(map[string]any)["numericValue"])-28.4) > 0.001 {
		t.Fatalf("capped %#v", capped)
	}

	pump := getJSON(t, router, "/api/v1/devices/"+deviceID+"/telemetry?metric=pump_active&from="+from+"&to="+to, tokenA)
	if pump["points"].([]any)[0].(map[string]any)["booleanValue"] != true {
		t.Fatalf("pump series %#v", pump)
	}

	bad := do(t, router, http.MethodGet, "/api/v1/devices/"+deviceID+"/telemetry?metric=temperature&resolution=1h", tokenA, nil)
	if bad.Code != http.StatusBadRequest || !json.Valid(bad.Body.Bytes()) {
		t.Fatalf("resolution status %d %s", bad.Code, bad.Body.String())
	}

	projectLatest := getJSON(t, router, "/api/v1/projects/"+projectID+"/telemetry/latest", tokenB)
	devices := projectLatest["devices"].([]any)
	if len(devices) != 2 {
		t.Fatalf("project devices %#v", projectLatest)
	}
	foundQuiet := false
	foundBench := false
	for _, item := range devices {
		row := item.(map[string]any)
		switch row["deviceId"] {
		case quietID:
			foundQuiet = true
			if len(row["metrics"].([]any)) != 0 {
				t.Fatalf("quiet device %#v", row)
			}
		case deviceID:
			foundBench = true
			if len(row["metrics"].([]any)) != 3 {
				t.Fatalf("bench %#v", row)
			}
		}
	}
	if !foundQuiet || !foundBench {
		t.Fatalf("project devices %#v", projectLatest)
	}
}

func insertTelemetry(t *testing.T, pool *pgxpool.Pool, projectID, deviceID, metric string, numeric *float64, boolean *bool, text *string, recorded time.Time) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		INSERT INTO telemetry (project_id, device_id, metric_key, numeric_value, boolean_value, string_value, recorded_at, received_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
	`, database.ID(projectID), database.ID(deviceID), metric, numeric, boolean, text, recorded)
	if err != nil {
		t.Fatal(err)
	}
}

func asFloat(t *testing.T, v any) float64 {
	t.Helper()
	n, ok := v.(float64)
	if !ok {
		t.Fatalf("numeric value %#v", v)
	}
	return n
}

func f64(v float64) *float64 { return &v }
func boolp(v bool) *bool     { return &v }
func strp(v string) *string  { return &v }

func getJSON(t *testing.T, router http.Handler, path, token string) map[string]any {
	t.Helper()
	body := getBody(t, router, path, token)
	var out map[string]any
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatal(err)
	}
	return out
}
