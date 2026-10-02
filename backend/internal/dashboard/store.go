package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/access"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/database"
)

type Counts struct {
	Total    int `json:"total"`
	Online   int `json:"online"`
	Offline  int `json:"offline"`
	Unknown  int `json:"unknown"`
	Disabled int `json:"disabled"`
}

type Overview struct {
	ProjectID     string     `json:"projectId"`
	Devices       Counts     `json:"devices"`
	MessagesToday int        `json:"messagesToday"`
	LastMessageAt *time.Time `json:"lastMessageAt"`
	ActiveAlerts  int        `json:"activeAlerts"`
}

type Activity struct {
	ID         string          `json:"id"`
	DeviceID   string          `json:"deviceId"`
	DeviceKey  string          `json:"deviceKey"`
	DeviceName string          `json:"deviceName"`
	EventType  string          `json:"eventType"`
	Topic      string          `json:"topic"`
	Payload    json.RawMessage `json:"payload"`
	CreatedAt  time.Time       `json:"createdAt"`
}

type State struct {
	DeviceID  string          `json:"deviceId"`
	State     json.RawMessage `json:"state"`
	UpdatedAt *time.Time      `json:"updatedAt"`
}

func overview(ctx context.Context, pool *pgxpool.Pool, projectID string) (Overview, error) {
	out := Overview{ProjectID: projectID}
	err := pool.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE status <> 'disabled'),
			count(*) FILTER (WHERE status = 'online'),
			count(*) FILTER (WHERE status = 'offline'),
			count(*) FILTER (WHERE status = 'unknown'),
			count(*) FILTER (WHERE status = 'disabled')
		FROM devices
		WHERE project_id = $1
	`, database.ID(projectID)).Scan(&out.Devices.Total, &out.Devices.Online, &out.Devices.Offline, &out.Devices.Unknown, &out.Devices.Disabled)
	if err != nil {
		return Overview{}, err
	}
	err = pool.QueryRow(ctx, `
		SELECT count(*)
		FROM device_events
		WHERE project_id = $1
		  AND event_type = 'telemetry'
		  AND created_at >= date_trunc('day', (now() AT TIME ZONE 'utc')) AT TIME ZONE 'utc'
	`, database.ID(projectID)).Scan(&out.MessagesToday)
	if err != nil {
		return Overview{}, err
	}
	err = pool.QueryRow(ctx, `
		SELECT max(received_at) FROM telemetry WHERE project_id = $1
	`, database.ID(projectID)).Scan(&out.LastMessageAt)
	return out, err
}

func projectEvents(ctx context.Context, pool *pgxpool.Pool, projectID, eventType string, limit int) ([]Activity, error) {
	rows, err := pool.Query(ctx, `
		SELECT e.id::text, e.device_id::text, d.device_key, d.name, e.event_type, e.topic, e.payload, e.created_at
		FROM device_events e
		JOIN devices d ON d.id = e.device_id
		WHERE e.project_id = $1
		  AND ($2 = '' OR e.event_type = $2)
		ORDER BY e.created_at DESC, e.id DESC
		LIMIT $3
	`, database.ID(projectID), eventType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanActivity(rows)
}

func deviceEvents(ctx context.Context, pool *pgxpool.Pool, deviceID, eventType string, limit int) ([]Activity, error) {
	rows, err := pool.Query(ctx, `
		SELECT e.id::text, e.device_id::text, d.device_key, d.name, e.event_type, e.topic, e.payload, e.created_at
		FROM device_events e
		JOIN devices d ON d.id = e.device_id
		WHERE e.device_id = $1
		  AND ($2 = '' OR e.event_type = $2)
		ORDER BY e.created_at DESC, e.id DESC
		LIMIT $3
	`, database.ID(deviceID), eventType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanActivity(rows)
}

func deviceProject(ctx context.Context, pool *pgxpool.Pool, deviceID string) (string, error) {
	var projectID string
	err := pool.QueryRow(ctx, `SELECT project_id::text FROM devices WHERE id = $1`, database.ID(deviceID)).Scan(&projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", access.ErrNotFound
	}
	return projectID, err
}

func deviceState(ctx context.Context, pool *pgxpool.Pool, deviceID string) (State, error) {
	out := State{DeviceID: deviceID}
	err := pool.QueryRow(ctx, `
		SELECT state, updated_at FROM device_state WHERE device_id = $1
	`, database.ID(deviceID)).Scan(&out.State, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	return out, err
}

type activityRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func scanActivity(rows activityRows) ([]Activity, error) {
	out := []Activity{}
	for rows.Next() {
		var item Activity
		if err := rows.Scan(&item.ID, &item.DeviceID, &item.DeviceKey, &item.DeviceName, &item.EventType, &item.Topic, &item.Payload, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
