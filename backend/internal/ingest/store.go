package ingest

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/database"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/events"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/mqtt"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/realtime"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/telemetry"
)

type deviceRow struct {
	ID         string
	ProjectID  string
	DeviceKey  string
	Status     string
	LastSeenAt *time.Time
}

type change struct {
	status        string
	touchLastSeen bool
	eventType     string
	topic         string
	payload       []byte
	firmware      string
	state         []byte
	points        []telemetry.Point
	recorded      time.Time
	received      time.Time
}

func (s *Service) lookup(ctx context.Context, projectID, deviceKey string) (deviceRow, bool, error) {
	var row deviceRow
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, project_id::text, device_key, status, last_seen_at
		FROM devices
		WHERE device_key = $1
	`, deviceKey).Scan(&row.ID, &row.ProjectID, &row.DeviceKey, &row.Status, &row.LastSeenAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return deviceRow{}, false, nil
	}
	if err != nil {
		return deviceRow{}, false, err
	}
	if !strings.EqualFold(row.ProjectID, projectID) {
		return deviceRow{}, false, nil
	}
	return row, true, nil
}

func (s *Service) writeError(ctx context.Context, device deviceRow, topic, reason string) error {
	payload, err := json.Marshal(map[string]string{"reason": reason})
	if err != nil {
		return err
	}
	return events.Insert(ctx, s.pool, device.ProjectID, device.ID, "error", topic, payload)
}

func (s *Service) persist(ctx context.Context, device deviceRow, ch change) error {
	when := ch.received.UTC()
	if when.IsZero() {
		when = s.now().UTC()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, point := range ch.points {
		numeric, boolean, text := pointValues(point)
		if _, err := tx.Exec(ctx, `
			INSERT INTO telemetry (
				project_id, device_id, metric_key, numeric_value, boolean_value, string_value, recorded_at, received_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, database.ID(device.ProjectID), database.ID(device.ID), point.Key, numeric, boolean, text, ch.recorded.UTC(), when); err != nil {
			return err
		}
	}
	if len(ch.state) > 0 {
		if _, err := tx.Exec(ctx, `
			INSERT INTO device_state (device_id, state, updated_at)
			VALUES ($1, $2, $3)
			ON CONFLICT (device_id) DO UPDATE
			SET state = EXCLUDED.state, updated_at = EXCLUDED.updated_at
		`, database.ID(device.ID), ch.state, when); err != nil {
			return err
		}
	}
	tag, err := tx.Exec(ctx, `
		UPDATE devices
		SET status = CASE WHEN $2 = '' THEN status ELSE $2 END,
		    last_seen_at = CASE WHEN $3 THEN $4 ELSE last_seen_at END,
		    firmware_version = CASE WHEN $5 = '' THEN firmware_version ELSE $5 END,
		    updated_at = $4
		WHERE id = $1 AND status <> 'disabled'
	`, database.ID(device.ID), ch.status, ch.touchLastSeen, when, ch.firmware)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	if ch.eventType != "" {
		body := ch.payload
		if len(body) == 0 {
			body = []byte(`{}`)
		}
		if err := events.Insert(ctx, tx, device.ProjectID, device.ID, ch.eventType, ch.topic, body); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.notifyChange(device, ch, when)
	return nil
}

func pointValues(point telemetry.Point) (numeric, boolean, text any) {
	if point.Numeric != nil {
		numeric = *point.Numeric
	}
	if point.Boolean != nil {
		boolean = *point.Boolean
	}
	if point.Text != nil {
		text = *point.Text
	}
	return numeric, boolean, text
}

// Sweep marks online devices offline when last_seen_at is older than the timeout.
func (s *Service) Sweep(ctx context.Context) (int, error) {
	if s == nil || s.pool == nil {
		return 0, nil
	}
	now := s.now().UTC()
	cutoff := now.Add(-s.offlineTimeout)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `
		UPDATE devices
		SET status = 'offline', updated_at = $1
		WHERE status = 'online'
		  AND last_seen_at IS NOT NULL
		  AND last_seen_at < $2
		RETURNING id::text, project_id::text, device_key
	`, now, cutoff)
	if err != nil {
		return 0, err
	}
	type stale struct {
		id      string
		project string
		key     string
	}
	found := []stale{}
	for rows.Next() {
		var item stale
		if err := rows.Scan(&item.id, &item.project, &item.key); err != nil {
			rows.Close()
			return 0, err
		}
		found = append(found, item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, item := range found {
		if err := events.Insert(ctx, tx, item.project, item.id, "disconnected", mqtt.DeviceTopic(item.project, item.key, "status"), []byte(`{"reason":"timeout"}`)); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	if s.notify != nil {
		for _, item := range found {
			s.notify(item.project, realtime.Event{
				Type:      realtime.DeviceOffline,
				DeviceID:  item.id,
				Timestamp: now,
				Data:      map[string]string{"reason": "timeout"},
			})
		}
	}
	return len(found), nil
}
