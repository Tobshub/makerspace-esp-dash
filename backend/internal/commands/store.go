package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/access"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/database"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/events"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/mqtt"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/realtime"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/telemetry"
)

var (
	ErrNotFound = access.ErrNotFound
	ErrDisabled = errors.New("device disabled")
)

const (
	maxPayloadBytes = 8192
	listLimit       = 50
)

// Input is a create body after validation.
type Input struct {
	Command string
	Payload json.RawMessage
}

type deviceTarget struct {
	ProjectID string
	DeviceKey string
	Status    string
}

func validateInput(command string, payload json.RawMessage) (Input, map[string]string) {
	fields := map[string]string{}
	command = strings.TrimSpace(command)
	if !telemetry.ValidMetricKey(command) {
		fields["command"] = "Command must start with a letter and use letters, digits, or underscores"
	}
	payload = bytes.TrimSpace(payload)
	if len(payload) == 0 {
		payload = []byte(`{}`)
	}
	if !json.Valid(payload) || payload[0] != '{' {
		fields["payload"] = "Payload must be a JSON object"
	} else if len(payload) > maxPayloadBytes {
		fields["payload"] = "Payload is too large"
	}
	if len(fields) > 0 {
		return Input{}, fields
	}
	return Input{Command: command, Payload: append([]byte(nil), payload...)}, nil
}

const commandSelect = `
	id::text, project_id::text, device_id::text, command, payload, status,
	COALESCE(requested_by::text, ''), requested_at, published_at, acknowledged_at, failed_at,
	correlation_id, error_message
`

func scanCommand(row pgx.Row) (Command, error) {
	var cmd Command
	err := row.Scan(
		&cmd.ID, &cmd.ProjectID, &cmd.DeviceID, &cmd.Command, &cmd.Payload, &cmd.Status,
		&cmd.RequestedBy, &cmd.RequestedAt, &cmd.PublishedAt, &cmd.AcknowledgedAt, &cmd.FailedAt,
		&cmd.CorrelationID, &cmd.ErrorMessage,
	)
	return cmd, err
}

func create(ctx context.Context, pool *pgxpool.Pool, deviceID, userID string, in Input) (Command, deviceTarget, error) {
	target, err := lookupDevice(ctx, pool, deviceID)
	if err != nil {
		return Command{}, deviceTarget{}, err
	}
	if target.Status == "disabled" {
		return Command{}, deviceTarget{}, ErrDisabled
	}
	correlation, err := newCorrelationID()
	if err != nil {
		return Command{}, deviceTarget{}, err
	}
	var requested any
	if userID != "" {
		requested = database.ID(userID)
	}
	cmd, err := scanCommand(pool.QueryRow(ctx, `
		INSERT INTO device_commands (
			project_id, device_id, command, payload, status, requested_by, correlation_id
		) VALUES ($1, $2, $3, $4, 'pending', $5, $6)
		RETURNING `+commandSelect+`
	`, database.ID(target.ProjectID), database.ID(deviceID), in.Command, in.Payload, requested, correlation))
	return cmd, target, err
}

func lookupDevice(ctx context.Context, pool *pgxpool.Pool, deviceID string) (deviceTarget, error) {
	var target deviceTarget
	err := pool.QueryRow(ctx, `
		SELECT project_id::text, device_key, status FROM devices WHERE id = $1
	`, database.ID(deviceID)).Scan(&target.ProjectID, &target.DeviceKey, &target.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return deviceTarget{}, ErrNotFound
	}
	return target, err
}

func listByDevice(ctx context.Context, pool *pgxpool.Pool, deviceID string) ([]Command, error) {
	rows, err := pool.Query(ctx, `
		SELECT `+commandSelect+`
		FROM device_commands
		WHERE device_id = $1
		ORDER BY requested_at DESC, id DESC
		LIMIT $2
	`, database.ID(deviceID), listLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Command{}
	for rows.Next() {
		cmd, err := scanCommand(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, cmd)
	}
	return out, rows.Err()
}

func get(ctx context.Context, pool *pgxpool.Pool, id string) (Command, error) {
	cmd, err := scanCommand(pool.QueryRow(ctx, `
		SELECT `+commandSelect+` FROM device_commands WHERE id = $1
	`, database.ID(id)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Command{}, ErrNotFound
	}
	return cmd, err
}

// Dispatch publishes the command without blocking the caller.
// A nil publisher marks the command failed.
func Dispatch(pool *pgxpool.Pool, pub Publisher, cmd Command, deviceKey string, notify func(string, realtime.Event)) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		if pub == nil {
			_ = markFailed(ctx, pool, cmd, "MQTT is not connected", notify)
			return
		}
		body, err := marshalWire(cmd, time.Now())
		if err != nil {
			_ = markFailed(ctx, pool, cmd, "Could not encode the command", notify)
			return
		}
		topic := mqtt.DeviceTopic(cmd.ProjectID, deviceKey, "commands")
		if err := pub.Publish(topic, 1, false, body); err != nil {
			_ = markFailed(ctx, pool, cmd, "MQTT publish failed", notify)
			return
		}
		_ = markPublished(ctx, pool, cmd, topic, body, notify)
	}()
}

func markPublished(ctx context.Context, pool *pgxpool.Pool, cmd Command, topic string, body []byte, notify func(string, realtime.Event)) error {
	now := time.Now().UTC()
	tag, err := pool.Exec(ctx, `
		UPDATE device_commands
		SET status = 'published', published_at = $2
		WHERE id = $1 AND status = 'pending'
	`, database.ID(cmd.ID), now)
	if err != nil {
		return err
	}
	if err := events.Insert(ctx, pool, cmd.ProjectID, cmd.DeviceID, "command", topic, body); err != nil {
		return err
	}
	if notify != nil {
		notify(cmd.ProjectID, realtime.Event{
			Type:      realtime.DeviceEvent,
			DeviceID:  cmd.DeviceID,
			Timestamp: now,
			Data:      map[string]string{"eventType": "command"},
		})
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	updated, err := get(ctx, pool, cmd.ID)
	if err != nil {
		return err
	}
	publishUpdate(notify, updated, now)
	return nil
}

func markFailed(ctx context.Context, pool *pgxpool.Pool, cmd Command, message string, notify func(string, realtime.Event)) error {
	now := time.Now().UTC()
	tag, err := pool.Exec(ctx, `
		UPDATE device_commands
		SET status = 'failed', failed_at = $2, error_message = $3
		WHERE id = $1 AND status IN ('pending', 'published')
	`, database.ID(cmd.ID), now, clip(message))
	if err != nil || tag.RowsAffected() == 0 {
		return err
	}
	updated, err := get(ctx, pool, cmd.ID)
	if err != nil {
		return err
	}
	publishUpdate(notify, updated, now)
	return nil
}

// ApplyAck updates a pending, published, or timed-out command for this device.
// A missing command is ignored so unknown acknowledgements stay debug events only.
func ApplyAck(ctx context.Context, pool *pgxpool.Pool, deviceID string, ack Ack, at time.Time) (*Command, error) {
	if pool == nil {
		return nil, nil
	}
	status := StatusAcknowledged
	message := ""
	if !ack.Success {
		status = StatusFailed
		message = clip(ack.Error)
		if message == "" {
			message = "Device reported failure"
		}
	}
	at = at.UTC()
	cmd, err := scanCommand(pool.QueryRow(ctx, `
		UPDATE device_commands
		SET status = $3,
		    acknowledged_at = CASE WHEN $3 = 'acknowledged' THEN $4 ELSE acknowledged_at END,
		    failed_at = CASE WHEN $3 = 'failed' THEN $4 ELSE failed_at END,
		    error_message = CASE WHEN $3 = 'failed' THEN $5 ELSE error_message END
		WHERE device_id = $1
		  AND correlation_id = $2
		  AND status IN ('pending', 'published', 'timed_out')
		RETURNING `+commandSelect+`
	`, database.ID(deviceID), ack.ID, status, at, message))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cmd, nil
}

// Expire marks commands that were never acknowledged before the deadline.
func Expire(ctx context.Context, pool *pgxpool.Pool, now time.Time, timeout time.Duration, notify func(string, realtime.Event)) (int, error) {
	if pool == nil || timeout <= 0 {
		return 0, nil
	}
	now = now.UTC()
	rows, err := pool.Query(ctx, `
		UPDATE device_commands
		SET status = 'timed_out', failed_at = $1, error_message = 'Device did not acknowledge in time'
		WHERE status IN ('pending', 'published')
		  AND requested_at < $2
		RETURNING `+commandSelect+`
	`, now, now.Add(-timeout))
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		cmd, err := scanCommand(rows)
		if err != nil {
			return count, err
		}
		count++
		publishUpdate(notify, cmd, now)
	}
	return count, rows.Err()
}

func publishUpdate(notify func(string, realtime.Event), cmd Command, at time.Time) {
	if notify == nil {
		return
	}
	notify(cmd.ProjectID, realtime.Event{
		Type:      realtime.CommandUpdated,
		DeviceID:  cmd.DeviceID,
		Timestamp: at,
		Data:      cmd,
	})
}

func clip(message string) string {
	message = strings.TrimSpace(message)
	if len(message) > 500 {
		return message[:500]
	}
	return message
}
