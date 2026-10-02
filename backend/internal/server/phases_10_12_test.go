package server

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/commands"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/config"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/database"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/ingest"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/mqtt"
)

type memPublisher struct {
	mu      sync.Mutex
	topic   string
	qos     byte
	payload []byte
	err     error
}

func (m *memPublisher) Publish(topic string, qos byte, _ bool, payload []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.topic = topic
	m.qos = qos
	m.payload = append([]byte(nil), payload...)
	return m.err
}

func TestCommandsControlsAlertsAndEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pool := openPool(t)
	defer pool.Close()
	if err := database.Migrate(poolURL(t)); err != nil {
		t.Fatal(err)
	}
	pub := &memPublisher{}
	router := New(Deps{
		Config:     config.Config{AppURL: "http://localhost:5173", DeviceOfflineTimeout: time.Minute},
		Pool:       pool,
		MQTTUp:     func() bool { return true },
		BrokerHost: "localhost",
		BrokerPort: 1883,
		Publisher:  pub,
	})

	suffix := time.Now().UTC().Format("20060102150405.000")
	emailA := "phase10-" + suffix + "@example.com"
	emailB := "phase10-view-" + suffix + "@example.com"
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

	team := postJSON(t, router, "/api/v1/teams", tokenA, map[string]string{"name": "Controls " + suffix}, http.StatusCreated)
	teamID = team["team"].(map[string]any)["id"].(string)
	project := postJSON(t, router, "/api/v1/teams/"+teamID+"/projects", tokenA, map[string]string{"name": "Bench"}, http.StatusCreated)
	projectID := project["project"].(map[string]any)["id"].(string)
	created := postJSON(t, router, "/api/v1/projects/"+projectID+"/devices", tokenA, map[string]string{"name": "Bed"}, http.StatusCreated)
	device := created["device"].(map[string]any)
	deviceID := device["id"].(string)
	deviceKey := device["deviceKey"].(string)
	postJSON(t, router, "/api/v1/teams/"+teamID+"/members", tokenA, map[string]string{"email": emailB, "role": "viewer"}, http.StatusCreated)

	if got := postStatus(t, router, "/api/v1/projects/"+projectID+"/controls", tokenB, map[string]any{
		"key": "pump", "name": "Pump", "controlType": "button", "command": "set_pump",
	}); got != http.StatusForbidden {
		t.Fatalf("viewer control status %d", got)
	}
	if got := postStatus(t, router, "/api/v1/projects/"+projectID+"/controls", tokenA, map[string]any{
		"key": "speed", "name": "Speed", "controlType": "slider", "command": "set_speed",
		"configuration": map[string]any{"min": 10, "max": 0, "step": 1, "payloadKey": "speed"},
	}); got != http.StatusBadRequest {
		t.Fatalf("bad slider status %d", got)
	}

	button := postJSON(t, router, "/api/v1/projects/"+projectID+"/controls", tokenA, map[string]any{
		"key": "open_gate", "name": "Open gate", "controlType": "button", "command": "open_gate",
	}, http.StatusCreated)
	if button["control"].(map[string]any)["command"] != "open_gate" {
		t.Fatalf("button %#v", button)
	}
	postJSON(t, router, "/api/v1/projects/"+projectID+"/controls", tokenA, map[string]any{
		"key": "fan", "name": "Fan", "controlType": "toggle", "command": "set_fan",
		"configuration": map[string]any{
			"onPayload":  map[string]any{"enabled": true},
			"offPayload": map[string]any{"enabled": false},
		},
	}, http.StatusCreated)
	slider := postJSON(t, router, "/api/v1/projects/"+projectID+"/controls", tokenA, map[string]any{
		"key": "speed", "name": "Speed", "controlType": "slider", "command": "set_speed",
		"configuration": map[string]any{"min": 0, "max": 100, "step": 5, "payloadKey": "speed"},
	}, http.StatusCreated)
	controlID := slider["control"].(map[string]any)["id"].(string)
	listed := getBody(t, router, "/api/v1/projects/"+projectID+"/controls", tokenA)
	if !containsAll(listed, "Open gate", "Fan", "set_speed") {
		t.Fatalf("controls %s", listed)
	}

	if got := postStatus(t, router, "/api/v1/devices/"+deviceID+"/commands", tokenB, map[string]any{
		"command": "set_pump", "payload": map[string]any{"enabled": true},
	}); got != http.StatusForbidden {
		t.Fatalf("viewer command status %d", got)
	}
	sent := postJSON(t, router, "/api/v1/devices/"+deviceID+"/commands", tokenA, map[string]any{
		"command": "set_pump", "payload": map[string]any{"enabled": true},
	}, http.StatusCreated)
	cmd := sent["command"].(map[string]any)
	if cmd["status"] != "pending" {
		t.Fatalf("create did not return immediately: %#v", cmd)
	}
	commandID := cmd["id"].(string)
	correlation := cmd["correlationId"].(string)
	published := waitCommandStatus(t, router, commandID, tokenA, "published")
	if published["correlationId"] != correlation {
		t.Fatalf("published %#v", published)
	}
	pub.mu.Lock()
	topic, qos, body := pub.topic, pub.qos, string(pub.payload)
	pub.mu.Unlock()
	if qos != 1 || topic != mqtt.DeviceTopic(projectID, deviceKey, "commands") || !containsAll(body, correlation, "set_pump", `"enabled":true`) {
		t.Fatalf("publish qos %d topic %s body %s", qos, topic, body)
	}

	ctx := context.Background()
	updated, err := commands.ApplyAck(ctx, pool, deviceID, commands.Ack{ID: correlation, Success: true}, time.Now().UTC())
	if err != nil || updated == nil || updated.Status != commands.StatusAcknowledged {
		t.Fatalf("ack %#v err %v", updated, err)
	}
	if got := getBody(t, router, "/api/v1/commands/"+commandID, tokenA); !containsAll(got, "acknowledged") {
		t.Fatalf("get %s", got)
	}
	history := getBody(t, router, "/api/v1/devices/"+deviceID+"/commands", tokenA)
	if !containsAll(history, "set_pump", "acknowledged") {
		t.Fatalf("history %s", history)
	}

	failed := postJSON(t, router, "/api/v1/devices/"+deviceID+"/commands", tokenA, map[string]any{
		"command": "set_fan", "payload": map[string]any{"enabled": true},
	}, http.StatusCreated)
	failedID := failed["command"].(map[string]any)["correlationId"].(string)
	waitCommandStatus(t, router, failed["command"].(map[string]any)["id"].(string), tokenA, "published")
	rejected, err := commands.ApplyAck(ctx, pool, deviceID, commands.Ack{ID: failedID, Success: false, Error: "invalid fan mode"}, time.Now().UTC())
	if err != nil || rejected == nil || rejected.Status != commands.StatusFailed || rejected.ErrorMessage != "invalid fan mode" {
		t.Fatalf("reject %#v err %v", rejected, err)
	}

	stale := postJSON(t, router, "/api/v1/devices/"+deviceID+"/commands", tokenA, map[string]any{
		"command": "open_gate", "payload": map[string]any{},
	}, http.StatusCreated)
	staleID := stale["command"].(map[string]any)["id"].(string)
	if _, err := pool.Exec(ctx, `UPDATE device_commands SET requested_at = now() - interval '2 minutes' WHERE id = $1`, database.ID(staleID)); err != nil {
		t.Fatal(err)
	}
	if _, err := commands.Expire(ctx, pool, time.Now().UTC(), 30*time.Second, nil); err != nil {
		t.Fatal(err)
	}
	if got := getBody(t, router, "/api/v1/commands/"+staleID, tokenA); !containsAll(got, "timed_out") {
		t.Fatalf("timeout %s", got)
	}

	postJSON(t, router, "/api/v1/devices/"+deviceID+"/disable", tokenA, map[string]any{}, http.StatusOK)
	if got := postStatus(t, router, "/api/v1/devices/"+deviceID+"/commands", tokenA, map[string]any{
		"command": "set_pump", "payload": map[string]any{"enabled": false},
	}); got != http.StatusConflict {
		t.Fatalf("disabled command status %d", got)
	}

	res := do(t, router, http.MethodDelete, "/api/v1/controls/"+controlID, tokenA, nil)
	if res.Code != http.StatusNoContent {
		t.Fatalf("delete control %d %s", res.Code, res.Body.String())
	}

	early := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	later := early.Add(time.Hour)
	if _, err := pool.Exec(ctx, `
		INSERT INTO device_events (project_id, device_id, event_type, topic, payload, created_at)
		VALUES ($1, $2, 'telemetry', 'topic', '{"metrics":{"temperature":21}}', $3)
	`, database.ID(projectID), database.ID(deviceID), early); err != nil {
		t.Fatal(err)
	}
	filtered := getBody(t, router, "/api/v1/projects/"+projectID+"/events?event_type=telemetry&device_id="+deviceID+"&from="+early.Add(-time.Minute).Format(time.RFC3339)+"&to="+later.Format(time.RFC3339), tokenA)
	if !containsAll(filtered, "telemetry", deviceKey) {
		t.Fatalf("filtered %s", filtered)
	}
	if got := getStatus(t, router, "/api/v1/projects/"+projectID+"/events?from=yesterday", tokenA); got != http.StatusBadRequest {
		t.Fatalf("bad from status %d", got)
	}
	outside := getBody(t, router, "/api/v1/projects/"+projectID+"/events?from="+later.Add(time.Hour).Format(time.RFC3339), tokenA)
	if containsAll(outside, `"createdAt":"`+early.Format(time.RFC3339)) {
		t.Fatalf("from filter kept the old event %s", outside)
	}

	second := postJSON(t, router, "/api/v1/projects/"+projectID+"/devices", tokenA, map[string]string{"name": "Probe"}, http.StatusCreated)
	probeID := second["device"].(map[string]any)["id"].(string)
	probeKey := second["device"].(map[string]any)["deviceKey"].(string)

	if got := postStatus(t, router, "/api/v1/projects/"+projectID+"/alerts", tokenB, map[string]any{
		"name": "Hot", "ruleType": "metric_threshold", "metricKey": "temperature", "operator": ">", "thresholdValue": 40,
	}); got != http.StatusForbidden {
		t.Fatalf("viewer alert status %d", got)
	}
	rule := postJSON(t, router, "/api/v1/projects/"+projectID+"/alerts", tokenA, map[string]any{
		"name": "Hot", "ruleType": "metric_threshold", "metricKey": "temperature", "operator": ">",
		"thresholdValue": 40, "durationSeconds": 0,
	}, http.StatusCreated)
	alertID := rule["alert"].(map[string]any)["id"].(string)
	held := postJSON(t, router, "/api/v1/projects/"+projectID+"/alerts", tokenA, map[string]any{
		"name": "Sustained", "ruleType": "metric_threshold", "metricKey": "temperature", "operator": ">",
		"thresholdValue": 30, "durationSeconds": 60, "deviceId": probeID,
	}, http.StatusCreated)
	if held["alert"].(map[string]any)["deviceId"] != probeID {
		t.Fatalf("held %#v", held)
	}
	offline := postJSON(t, router, "/api/v1/projects/"+projectID+"/alerts", tokenA, map[string]any{
		"name": "Down", "ruleType": "device_offline", "durationSeconds": 0, "deviceId": probeID,
	}, http.StatusCreated)
	if offline["alert"].(map[string]any)["ruleType"] != "device_offline" {
		t.Fatalf("offline %#v", offline)
	}

	when := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	svc := ingest.New(pool, ingest.Options{Now: func() time.Time { return when }, TelemetryPerSecond: 10})
	hot := []byte(`{"metrics":{"temperature":41.5}}`)
	if err := svc.Handle(ctx, mqtt.DeviceTopic(projectID, probeKey, "telemetry"), hot); err != nil {
		t.Fatal(err)
	}
	if err := svc.Handle(ctx, mqtt.DeviceTopic(projectID, probeKey, "telemetry"), hot); err != nil {
		t.Fatal(err)
	}
	events := getBody(t, router, "/api/v1/projects/"+projectID+"/alert-events", tokenA)
	if !containsAll(events, "triggered", "Hot", "temperature") {
		t.Fatalf("triggered %s", events)
	}
	if countStatus(t, events, "triggered") != 1 {
		t.Fatalf("duplicate active alerts %s", events)
	}
	if containsAll(events, "Sustained") {
		t.Fatalf("duration fired immediately %s", events)
	}
	overview := getBody(t, router, "/api/v1/projects/"+projectID+"/overview", tokenA)
	if !containsAll(overview, `"activeAlerts":1`) {
		t.Fatalf("overview %s", overview)
	}

	if err := svc.Handle(ctx, mqtt.DeviceTopic(projectID, probeKey, "telemetry"), []byte(`{"metrics":{"temperature":20}}`)); err != nil {
		t.Fatal(err)
	}
	events = getBody(t, router, "/api/v1/projects/"+projectID+"/alert-events", tokenA)
	if !containsAll(events, "resolved") || countStatus(t, events, "triggered") != 0 {
		t.Fatalf("resolved %s", events)
	}

	if _, err := pool.Exec(ctx, `UPDATE devices SET status = 'offline', updated_at = now() WHERE id = $1`, database.ID(probeID)); err != nil {
		t.Fatal(err)
	}
	if err := svc.Handle(ctx, mqtt.DeviceTopic(projectID, probeKey, "status"), []byte(`{"status":"offline"}`)); err != nil {
		t.Fatal(err)
	}
	events = getBody(t, router, "/api/v1/projects/"+projectID+"/alert-events", tokenA)
	if !containsAll(events, "Down", "offline", "triggered") {
		t.Fatalf("offline alert %s", events)
	}
	if err := svc.Handle(ctx, mqtt.DeviceTopic(projectID, probeKey, "status"), []byte(`{"status":"online"}`)); err != nil {
		t.Fatal(err)
	}
	events = getBody(t, router, "/api/v1/projects/"+projectID+"/alert-events", tokenA)
	if countStatus(t, events, "triggered") != 0 {
		t.Fatalf("offline stayed open %s", events)
	}

	patched := patchJSON(t, router, "/api/v1/alerts/"+alertID, tokenA, map[string]any{
		"name": "Hotter", "ruleType": "metric_threshold", "metricKey": "temperature", "operator": ">=",
		"thresholdValue": 50, "durationSeconds": 0, "enabled": false,
	})
	if patched["alert"].(map[string]any)["enabled"] != false || patched["alert"].(map[string]any)["name"] != "Hotter" {
		t.Fatalf("patched %#v", patched)
	}
}

func waitCommandStatus(t *testing.T, router http.Handler, id, token, status string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	var last string
	for time.Now().Before(deadline) {
		body := getBody(t, router, "/api/v1/commands/"+id, token)
		var parsed map[string]any
		if err := json.Unmarshal([]byte(body), &parsed); err != nil {
			t.Fatal(err)
		}
		cmd := parsed["command"].(map[string]any)
		if cmd["status"] == status {
			return cmd
		}
		last = body
		time.Sleep(15 * time.Millisecond)
	}
	t.Fatalf("command %s never became %s: %s", id, status, last)
	return nil
}

func countStatus(t *testing.T, body, status string) int {
	t.Helper()
	var parsed struct {
		Events []struct {
			Status string `json:"status"`
		} `json:"events"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, event := range parsed.Events {
		if event.Status == status {
			n++
		}
	}
	return n
}
