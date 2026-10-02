package server

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/config"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/database"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/realtime"
)

func TestProjectStreamAndOverview(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pool := openPool(t)
	defer pool.Close()
	if err := database.Migrate(poolURL(t)); err != nil {
		t.Fatal(err)
	}
	hub := realtime.New()
	router := New(Deps{
		Config:     config.Config{AppURL: "http://localhost:5173", DeviceOfflineTimeout: time.Minute, MQTTBrokerURL: "tcp://localhost:1883"},
		Pool:       pool,
		MQTTUp:     func() bool { return true },
		BrokerHost: "localhost",
		BrokerPort: 1883,
		Hub:        hub,
	})

	suffix := time.Now().UTC().Format("20060102150405.000")
	emailA := "stream-a-" + suffix + "@example.com"
	emailB := "stream-b-" + suffix + "@example.com"
	tokenA, _ := register(t, router, emailA, "Ada")
	var teamID string
	t.Cleanup(func() {
		ctx := context.Background()
		if teamID != "" {
			_, _ = pool.Exec(ctx, `DELETE FROM teams WHERE id = $1`, database.ID(teamID))
		}
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE email = $1 OR email = $2`, emailA, emailB)
	})

	team := postJSON(t, router, "/api/v1/teams", tokenA, map[string]string{"name": "Stream Team " + suffix}, http.StatusCreated)
	teamID = team["team"].(map[string]any)["id"].(string)
	project := postJSON(t, router, "/api/v1/teams/"+teamID+"/projects", tokenA, map[string]string{"name": "Beds"}, http.StatusCreated)
	projectID := project["project"].(map[string]any)["id"].(string)
	created := postJSON(t, router, "/api/v1/projects/"+projectID+"/devices", tokenA, map[string]string{"name": "Bench"}, http.StatusCreated)
	deviceID := created["device"].(map[string]any)["id"].(string)

	other := postJSON(t, router, "/api/v1/auth/register", "", map[string]string{
		"email":    emailB,
		"name":     "Grace",
		"password": "test-password",
	}, http.StatusCreated)
	tokenB := other["token"].(string)

	if got := getStatus(t, router, "/api/v1/projects/"+projectID+"/stream", ""); got != http.StatusUnauthorized {
		t.Fatalf("anonymous stream %d", got)
	}
	if got := getStatus(t, router, "/api/v1/projects/"+projectID+"/stream?access_token=nope", ""); got != http.StatusUnauthorized {
		t.Fatalf("bad token %d", got)
	}
	if got := getStatus(t, router, "/api/v1/projects/"+projectID+"/stream", tokenB); got != http.StatusNotFound {
		t.Fatalf("other stream %d", got)
	}
	if got := getStatus(t, router, "/api/v1/projects/"+projectID+"/overview", tokenB); got != http.StatusNotFound {
		t.Fatalf("other overview %d", got)
	}
	if got := getStatus(t, router, "/api/v1/projects/"+projectID+"/events", tokenB); got != http.StatusNotFound {
		t.Fatalf("other events %d", got)
	}
	if got := getStatus(t, router, "/api/v1/devices/"+deviceID+"/state", tokenB); got != http.StatusNotFound {
		t.Fatalf("other state %d", got)
	}

	now := time.Now().UTC().Truncate(time.Millisecond)
	insertTelemetry(t, pool, projectID, deviceID, "temperature", f64(21.5), nil, nil, now)
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO device_events (project_id, device_id, event_type, topic, payload, created_at)
		VALUES ($1, $2, 'telemetry', 'makerspace/v1/telemetry', '{"metrics":{"temperature":21.5}}', $3)
	`, database.ID(projectID), database.ID(deviceID), now); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO device_state (device_id, state, updated_at)
		VALUES ($1, '{"wifi_rssi":-48}', $2)
	`, database.ID(deviceID), now); err != nil {
		t.Fatal(err)
	}

	overview := getJSON(t, router, "/api/v1/projects/"+projectID+"/overview", tokenA)
	counts := overview["devices"].(map[string]any)
	if counts["total"] != float64(1) || counts["unknown"] != float64(1) || overview["messagesToday"] != float64(1) {
		t.Fatalf("overview %#v", overview)
	}
	if overview["lastMessageAt"] == nil || overview["activeAlerts"] != float64(0) {
		t.Fatalf("overview %#v", overview)
	}
	events := getJSON(t, router, "/api/v1/projects/"+projectID+"/events?limit=5", tokenA)
	if len(events["events"].([]any)) != 1 {
		t.Fatalf("events %#v", events)
	}
	state := getJSON(t, router, "/api/v1/devices/"+deviceID+"/state", tokenA)
	rssi := state["state"].(map[string]any)["wifi_rssi"]
	if rssi != float64(-48) {
		t.Fatalf("state %#v", state)
	}
	if got := getStatus(t, router, "/api/v1/projects/"+projectID+"/events?limit=500", tokenA); got != http.StatusBadRequest {
		t.Fatalf("limit status %d", got)
	}

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/v1/projects/"+projectID+"/stream?access_token="+tokenA, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK || !strings.Contains(res.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("stream status %d type %s", res.StatusCode, res.Header.Get("Content-Type"))
	}

	go func() {
		time.Sleep(150 * time.Millisecond)
		hub.Publish(projectID, realtime.Event{
			Type:      realtime.TelemetryReceived,
			DeviceID:  deviceID,
			Timestamp: now,
			Data:      map[string]any{"temperature": 28.4},
		})
	}()

	scanner := bufio.NewScanner(res.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err != nil {
			t.Fatal(err)
		}
		if event["type"] != realtime.TelemetryReceived || event["deviceId"] != deviceID {
			t.Fatalf("event %#v", event)
		}
		data := event["data"].(map[string]any)
		if data["temperature"] != 28.4 {
			t.Fatalf("event %#v", event)
		}
		return
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		t.Fatal(err)
	}
	t.Fatal("stream closed before the event")
}
