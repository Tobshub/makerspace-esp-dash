package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/config"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/database"
)

func TestMetricDefinitions(t *testing.T) {
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
	emailA := "metrics-a-" + suffix + "@example.com"
	emailB := "metrics-b-" + suffix + "@example.com"
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

	team := postJSON(t, router, "/api/v1/teams", tokenA, map[string]string{"name": "Metrics Team " + suffix}, http.StatusCreated)
	teamID = team["team"].(map[string]any)["id"].(string)
	project := postJSON(t, router, "/api/v1/teams/"+teamID+"/projects", tokenA, map[string]string{"name": "Beds"}, http.StatusCreated)
	projectID := project["project"].(map[string]any)["id"].(string)
	created := postJSON(t, router, "/api/v1/projects/"+projectID+"/devices", tokenA, map[string]string{"name": "Bench"}, http.StatusCreated)
	deviceID := created["device"].(map[string]any)["id"].(string)

	ctx := context.Background()
	if _, err := pool.Exec(ctx, `
		INSERT INTO telemetry (project_id, device_id, metric_key, numeric_value, recorded_at)
		VALUES ($1, $2, 'temperature', 21.5, now())
	`, database.ID(projectID), database.ID(deviceID)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO telemetry (project_id, device_id, metric_key, boolean_value, recorded_at)
		VALUES ($1, $2, 'pump_active', true, now())
	`, database.ID(projectID), database.ID(deviceID)); err != nil {
		t.Fatal(err)
	}

	latest := getBody(t, router, "/api/v1/devices/"+deviceID+"/telemetry/latest", tokenA)
	if !containsAll(latest, "temperature", "pump_active") {
		t.Fatalf("telemetry arrived without definitions: %s", latest)
	}

	if got := getStatus(t, router, "/api/v1/projects/"+projectID+"/metrics/discovered", tokenB); got != http.StatusNotFound {
		t.Fatalf("other user discovered status %d", got)
	}

	discovered := getBody(t, router, "/api/v1/projects/"+projectID+"/metrics/discovered", tokenA)
	if !containsAll(discovered, "temperature", "pump_active", `"dataType":"number"`, `"dataType":"boolean"`) {
		t.Fatalf("discovered %s", discovered)
	}

	member := postJSON(t, router, "/api/v1/teams/"+teamID+"/members", tokenA, map[string]string{
		"email": emailB,
		"role":  "viewer",
	}, http.StatusCreated)
	if member["member"].(map[string]any)["role"] != "viewer" {
		t.Fatalf("member %#v", member)
	}
	if got := postStatus(t, router, "/api/v1/projects/"+projectID+"/metrics", tokenB, map[string]any{
		"key": "temperature", "name": "Temperature", "dataType": "number", "displayType": "number",
	}); got != http.StatusForbidden {
		t.Fatalf("viewer create status %d", got)
	}

	if got := postStatus(t, router, "/api/v1/projects/"+projectID+"/metrics", tokenA, map[string]any{
		"key": "soil", "name": "Soil", "dataType": "number", "displayType": "gauge",
	}); got != http.StatusBadRequest {
		t.Fatalf("gauge without range status %d", got)
	}

	min := 0.0
	max := 50.0
	saved := postJSON(t, router, "/api/v1/projects/"+projectID+"/metrics", tokenA, map[string]any{
		"key":         "temperature",
		"name":        "Temperature",
		"description": "Air",
		"dataType":    "number",
		"unit":        "°C",
		"displayType": "gauge",
		"minValue":    min,
		"maxValue":    max,
	}, http.StatusCreated)
	metric := saved["metric"].(map[string]any)
	metricID := metric["id"].(string)
	if metric["unit"] != "°C" || metric["displayType"] != "gauge" || metric["key"] != "temperature" || metric["hidden"] != false {
		t.Fatalf("created %#v", metric)
	}

	if got := postStatus(t, router, "/api/v1/projects/"+projectID+"/metrics", tokenA, map[string]any{
		"key": "temperature", "name": "Again", "dataType": "number", "displayType": "number",
	}); got != http.StatusConflict {
		t.Fatalf("duplicate status %d", got)
	}

	discovered = getBody(t, router, "/api/v1/projects/"+projectID+"/metrics/discovered", tokenA)
	if containsAll(discovered, "temperature") || !containsAll(discovered, "pump_active") {
		t.Fatalf("discovered after define %s", discovered)
	}

	patched := patchJSON(t, router, "/api/v1/metrics/"+metricID, tokenA, map[string]any{
		"name":        "Bench temperature",
		"description": "Air near the bed",
		"dataType":    "number",
		"unit":        "°C",
		"displayType": "line",
	})
	if patched["metric"].(map[string]any)["name"] != "Bench temperature" {
		t.Fatalf("patched %#v", patched)
	}
	if patched["metric"].(map[string]any)["key"] != "temperature" {
		t.Fatalf("key changed %#v", patched)
	}

	hidden := patchJSON(t, router, "/api/v1/metrics/"+metricID, tokenA, map[string]any{
		"name":        "Bench temperature",
		"description": "Air near the bed",
		"dataType":    "number",
		"unit":        "°C",
		"displayType": "line",
		"hidden":      true,
	})
	if hidden["metric"].(map[string]any)["hidden"] != true {
		t.Fatalf("hidden %#v", hidden)
	}

	listed := getBody(t, router, "/api/v1/projects/"+projectID+"/metrics", tokenA)
	if !containsAll(listed, "Bench temperature", "temperature") {
		t.Fatalf("list %s", listed)
	}
	if got := getStatus(t, router, "/api/v1/metrics/"+metricID, tokenB); got != http.StatusOK {
		t.Fatalf("viewer get status %d", got)
	}

	res := do(t, router, http.MethodDelete, "/api/v1/metrics/"+metricID, tokenA, nil)
	if res.Code != http.StatusNoContent {
		t.Fatalf("delete status %d body %s", res.Code, res.Body.String())
	}
	discovered = getBody(t, router, "/api/v1/projects/"+projectID+"/metrics/discovered", tokenA)
	if !containsAll(discovered, "temperature", "pump_active") {
		t.Fatalf("discovered after delete %s", discovered)
	}
	latest = getBody(t, router, "/api/v1/devices/"+deviceID+"/telemetry/latest", tokenA)
	if !containsAll(latest, "temperature", "pump_active") {
		t.Fatalf("telemetry after delete %s", latest)
	}
}

func patchJSON(t *testing.T, router http.Handler, path, token string, payload any) map[string]any {
	t.Helper()
	res := do(t, router, http.MethodPatch, path, token, payload)
	if res.Code != http.StatusOK {
		t.Fatalf("PATCH %s status %d body %s", path, res.Code, res.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func containsAll(body string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(body, part) {
			return false
		}
	}
	return true
}
