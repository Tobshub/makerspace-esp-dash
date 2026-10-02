package ingest

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/config"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/database"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/devices"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/mqtt"
)

func TestIngestPipeline(t *testing.T) {
	pool := openPool(t)
	defer pool.Close()
	if err := database.Migrate(poolURL(t)); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	fixed := time.Date(2026, 10, 2, 1, 0, 0, 0, time.UTC)
	projectID, cleanup := seedProject(t, pool)
	t.Cleanup(cleanup)

	hash, err := bcrypt.GenerateFromPassword([]byte("test-secret"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("telemetry", func(t *testing.T) {
		deviceID, key := insertDevice(t, pool, projectID, string(hash), "unknown", nil)
		svc := New(pool, Options{Now: func() time.Time { return fixed }, TelemetryPerSecond: 10})
		topic := mqtt.DeviceTopic(projectID, key, "telemetry")
		body := []byte(`{"timestamp":1700000000000,"metrics":{"temperature":28.4,"humidity":0,"pump_active":false,"mode":"automatic","soil_moisture":40}}`)
		if err := svc.Handle(ctx, topic, body); err != nil {
			t.Fatal(err)
		}
		if got := metricCount(t, pool, deviceID); got != 5 {
			t.Fatalf("metrics %d", got)
		}
		status, firmware, seen := devicePresence(t, pool, deviceID)
		if status != "online" || firmware != "" || seen == nil || !seen.Equal(fixed) {
			t.Fatalf("status %s firmware %s seen %v", status, firmware, seen)
		}
		var recorded time.Time
		if err := pool.QueryRow(ctx, `SELECT recorded_at FROM telemetry WHERE device_id = $1 AND metric_key = 'temperature'`, database.ID(deviceID)).Scan(&recorded); err != nil {
			t.Fatal(err)
		}
		if !recorded.Equal(time.UnixMilli(1700000000000).UTC()) {
			t.Fatal(recorded)
		}
		if eventTypes(t, pool, deviceID)[0] != "telemetry" {
			t.Fatal(eventTypes(t, pool, deviceID))
		}
	})

	t.Run("rejects nested and keeps status", func(t *testing.T) {
		deviceID, key := insertDevice(t, pool, projectID, string(hash), "unknown", nil)
		svc := New(pool, Options{Now: func() time.Time { return fixed }})
		topic := mqtt.DeviceTopic(projectID, key, "telemetry")
		if err := svc.Handle(ctx, topic, []byte(`{"metrics":{"temperature":{"c":1}}}`)); err != nil {
			t.Fatal(err)
		}
		if metricCount(t, pool, deviceID) != 0 {
			t.Fatal("nested metrics were stored")
		}
		status, _, seen := devicePresence(t, pool, deviceID)
		if status != "unknown" || seen != nil {
			t.Fatalf("status %s seen %v", status, seen)
		}
		var reason string
		if err := pool.QueryRow(ctx, `SELECT payload->>'reason' FROM device_events WHERE device_id = $1`, database.ID(deviceID)).Scan(&reason); err != nil {
			t.Fatal(err)
		}
		if reason != "nested metric" {
			t.Fatal(reason)
		}
	})

	t.Run("rate limit", func(t *testing.T) {
		deviceID, key := insertDevice(t, pool, projectID, string(hash), "unknown", nil)
		svc := New(pool, Options{Now: func() time.Time { return fixed }, TelemetryPerSecond: 1})
		topic := mqtt.DeviceTopic(projectID, key, "telemetry")
		body := []byte(`{"metrics":{"temperature":1}}`)
		if err := svc.Handle(ctx, topic, body); err != nil {
			t.Fatal(err)
		}
		if err := svc.Handle(ctx, topic, []byte(`{"metrics":{"temperature":2}}`)); err != nil {
			t.Fatal(err)
		}
		if metricCount(t, pool, deviceID) != 1 {
			t.Fatalf("metrics %d", metricCount(t, pool, deviceID))
		}
	})

	t.Run("wrong project and disabled", func(t *testing.T) {
		deviceID, key := insertDevice(t, pool, projectID, string(hash), "disabled", nil)
		svc := New(pool, Options{Now: func() time.Time { return fixed }})
		if err := svc.Handle(ctx, mqtt.DeviceTopic(projectID, key, "telemetry"), []byte(`{"metrics":{"temperature":1}}`)); err != nil {
			t.Fatal(err)
		}
		other := "22222222-2222-4222-8222-222222222222"
		liveID, liveKey := insertDevice(t, pool, projectID, string(hash), "unknown", nil)
		if err := svc.Handle(ctx, mqtt.DeviceTopic(other, liveKey, "telemetry"), []byte(`{"metrics":{"temperature":1}}`)); err != nil {
			t.Fatal(err)
		}
		if metricCount(t, pool, deviceID) != 0 || metricCount(t, pool, liveID) != 0 {
			t.Fatal("rejected devices stored telemetry")
		}
		status, _, _ := devicePresence(t, pool, deviceID)
		if status != "disabled" {
			t.Fatal(status)
		}
	})

	t.Run("state", func(t *testing.T) {
		deviceID, key := insertDevice(t, pool, projectID, string(hash), "unknown", nil)
		svc := New(pool, Options{Now: func() time.Time { return fixed }})
		body := []byte(`{"firmwareVersion":"1.0.3","wifiRssi":-61,"mode":"automatic","relay":true}`)
		if err := svc.Handle(ctx, mqtt.DeviceTopic(projectID, key, "state"), body); err != nil {
			t.Fatal(err)
		}
		status, firmware, seen := devicePresence(t, pool, deviceID)
		if status != "online" || firmware != "1.0.3" || seen == nil || !seen.Equal(fixed) {
			t.Fatalf("status %s firmware %s seen %v", status, firmware, seen)
		}
		var state string
		if err := pool.QueryRow(ctx, `SELECT state->>'mode' FROM device_state WHERE device_id = $1`, database.ID(deviceID)).Scan(&state); err != nil {
			t.Fatal(err)
		}
		if state != "automatic" {
			t.Fatal(state)
		}
	})

	t.Run("presence and timeout", func(t *testing.T) {
		old := fixed.Add(-time.Hour)
		recent := fixed.Add(-10 * time.Second)
		boundary := fixed.Add(-60 * time.Second)
		stale := fixed.Add(-61 * time.Second)
		offlineID, offlineKey := insertDevice(t, pool, projectID, string(hash), "online", &old)
		freshID, freshKey := insertDevice(t, pool, projectID, string(hash), "offline", &recent)
		holdID, _ := insertDevice(t, pool, projectID, string(hash), "online", &boundary)
		dropID, _ := insertDevice(t, pool, projectID, string(hash), "online", &stale)
		svc := New(pool, Options{Now: func() time.Time { return fixed }, OfflineTimeout: time.Minute})

		if err := svc.HandleMessage(ctx, mqtt.DeviceTopic(projectID, offlineKey, "status"), []byte(`{"status":"offline"}`), true); err != nil {
			t.Fatal(err)
		}
		status, _, seen := devicePresence(t, pool, offlineID)
		if status != "offline" || seen == nil || !seen.Equal(old) {
			t.Fatalf("retained offline status %s seen %v", status, seen)
		}
		if err := svc.HandleMessage(ctx, mqtt.DeviceTopic(projectID, offlineKey, "status"), []byte(`{"status":"online"}`), true); err != nil {
			t.Fatal(err)
		}
		status, _, seen = devicePresence(t, pool, offlineID)
		if status != "offline" || seen == nil || !seen.Equal(old) {
			t.Fatalf("stale retained online status %s seen %v", status, seen)
		}
		if err := svc.HandleMessage(ctx, mqtt.DeviceTopic(projectID, freshKey, "status"), []byte(`{"status":"online"}`), true); err != nil {
			t.Fatal(err)
		}
		status, _, seen = devicePresence(t, pool, freshID)
		if status != "online" || seen == nil || !seen.Equal(recent) {
			t.Fatalf("fresh retained online status %s seen %v", status, seen)
		}
		if err := svc.Handle(ctx, mqtt.DeviceTopic(projectID, freshKey, "telemetry"), []byte(`{"metrics":{}}`)); err != nil {
			t.Fatal(err)
		}
		_, _, seen = devicePresence(t, pool, freshID)
		if seen == nil || !seen.Equal(fixed) {
			t.Fatalf("live telemetry seen %v", seen)
		}

		if _, err := svc.Sweep(ctx); err != nil {
			t.Fatal(err)
		}
		holdStatus, _, _ := devicePresence(t, pool, holdID)
		dropStatus, _, _ := devicePresence(t, pool, dropID)
		if holdStatus != "online" || dropStatus != "offline" {
			t.Fatalf("boundary %s stale %s", holdStatus, dropStatus)
		}
		var reason string
		if err := pool.QueryRow(ctx, `
			SELECT payload->>'reason' FROM device_events
			WHERE device_id = $1 AND event_type = 'disconnected'
		`, database.ID(dropID)).Scan(&reason); err != nil {
			t.Fatal(err)
		}
		if reason != "timeout" {
			t.Fatal(reason)
		}
	})

	t.Run("retained telemetry is ignored", func(t *testing.T) {
		deviceID, key := insertDevice(t, pool, projectID, string(hash), "unknown", nil)
		svc := New(pool, Options{Now: func() time.Time { return fixed }})
		if err := svc.HandleMessage(ctx, mqtt.DeviceTopic(projectID, key, "telemetry"), []byte(`{"metrics":{"temperature":1}}`), true); err != nil {
			t.Fatal(err)
		}
		if metricCount(t, pool, deviceID) != 0 {
			t.Fatal("retained telemetry stored")
		}
	})

	t.Run("ack and command topic", func(t *testing.T) {
		deviceID, key := insertDevice(t, pool, projectID, string(hash), "unknown", nil)
		svc := New(pool, Options{Now: func() time.Time { return fixed }})
		if err := svc.Handle(ctx, mqtt.DeviceTopic(projectID, key, "commands"), []byte(`{"command":"set_led"}`)); err != nil {
			t.Fatal(err)
		}
		if err := svc.Handle(ctx, mqtt.DeviceTopic(projectID, key, "commands/ack"), []byte(`{"id":"cmd_123","success":true}`)); err != nil {
			t.Fatal(err)
		}
		status, _, _ := devicePresence(t, pool, deviceID)
		if status != "online" {
			t.Fatal(status)
		}
		types := eventTypes(t, pool, deviceID)
		if len(types) != 1 || types[0] != "command_ack" {
			t.Fatalf("%v", types)
		}
	})
}

func seedProject(t *testing.T, pool *pgxpool.Pool) (string, func()) {
	t.Helper()
	ctx := context.Background()
	suffix := time.Now().UTC().Format("20060102150405.000000000")
	var teamID, projectID string
	if err := pool.QueryRow(ctx, `INSERT INTO teams (name, slug) VALUES ($1, $2) RETURNING id::text`, "Ingest "+suffix, "ingest-"+suffix).Scan(&teamID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO projects (team_id, name, slug) VALUES ($1, 'Bed', 'bed') RETURNING id::text`, database.ID(teamID)).Scan(&projectID); err != nil {
		t.Fatal(err)
	}
	return projectID, func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM teams WHERE id = $1`, database.ID(teamID))
	}
}

func insertDevice(t *testing.T, pool *pgxpool.Pool, projectID, hash, status string, lastSeen *time.Time) (string, string) {
	t.Helper()
	key, err := devices.GenerateDeviceKey()
	if err != nil {
		t.Fatal(err)
	}
	var seen any
	if lastSeen != nil {
		seen = *lastSeen
	}
	var id string
	err = pool.QueryRow(context.Background(), `
		INSERT INTO devices (project_id, name, device_key, secret_hash, status, last_seen_at)
		VALUES ($1, 'Probe', $2, $3, $4, $5)
		RETURNING id::text
	`, database.ID(projectID), key, hash, status, seen).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id, key
}

func metricCount(t *testing.T, pool *pgxpool.Pool, deviceID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM telemetry WHERE device_id = $1`, database.ID(deviceID)).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func devicePresence(t *testing.T, pool *pgxpool.Pool, deviceID string) (string, string, *time.Time) {
	t.Helper()
	var status, firmware string
	var seen *time.Time
	err := pool.QueryRow(context.Background(), `SELECT status, firmware_version, last_seen_at FROM devices WHERE id = $1`, database.ID(deviceID)).Scan(&status, &firmware, &seen)
	if err != nil {
		t.Fatal(err)
	}
	return status, firmware, seen
}

func eventTypes(t *testing.T, pool *pgxpool.Pool, deviceID string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT event_type FROM device_events WHERE device_id = $1 ORDER BY created_at, event_type`, database.ID(deviceID))
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var eventType string
		if err := rows.Scan(&eventType); err != nil {
			t.Fatal(err)
		}
		out = append(out, eventType)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
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
