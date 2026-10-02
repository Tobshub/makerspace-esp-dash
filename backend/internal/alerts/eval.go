package alerts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/database"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/realtime"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/telemetry"
)

// Sample is one ingested change that may open or close alerts.
type Sample struct {
	ProjectID      string
	DeviceID       string
	PreviousStatus string
	Status         string
	Points         []telemetry.Point
	At             time.Time
}

// Notifier receives alert.triggered and alert.resolved. Nil skips fan-out.
type Notifier func(projectID string, event realtime.Event)

// OnIngest evaluates enabled rules for one device after a stored message.
func OnIngest(ctx context.Context, pool *pgxpool.Pool, sample Sample, notify Notifier) error {
	if pool == nil || sample.ProjectID == "" || sample.DeviceID == "" {
		return nil
	}
	rules, err := enabledForProject(ctx, pool, sample.ProjectID)
	if err != nil {
		return err
	}
	status := sample.Status
	if status == "" {
		status = sample.PreviousStatus
	}
	for _, rule := range rules {
		if rule.DeviceID != "" && rule.DeviceID != sample.DeviceID {
			continue
		}
		switch rule.RuleType {
		case RuleThreshold:
			for _, point := range sample.Points {
				if point.Key != rule.MetricKey || point.Numeric == nil || rule.ThresholdValue == nil {
					continue
				}
				if err := applyThreshold(ctx, pool, rule, sample.DeviceID, *point.Numeric, sample.At, notify); err != nil {
					return err
				}
			}
		case RuleOffline:
			if err := applyPresence(ctx, pool, rule, sample.DeviceID, status, sample.At, notify); err != nil {
				return err
			}
		}
	}
	return nil
}

// EvaluateOffline checks every enabled offline rule. The presence sweep calls this.
func EvaluateOffline(ctx context.Context, pool *pgxpool.Pool, now time.Time, notify Notifier) error {
	if pool == nil {
		return nil
	}
	rows, err := pool.Query(ctx, `
		SELECT r.id::text, r.project_id::text, COALESCE(r.device_id::text, ''), r.name, r.duration_seconds,
		       d.id::text, d.status
		FROM alert_rules r
		JOIN devices d ON d.project_id = r.project_id
		  AND (r.device_id IS NULL OR r.device_id = d.id)
		WHERE r.enabled AND r.rule_type = 'device_offline' AND d.status <> 'disabled'
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type pair struct {
		rule   Rule
		device string
		status string
	}
	found := []pair{}
	for rows.Next() {
		var item pair
		if err := rows.Scan(&item.rule.ID, &item.rule.ProjectID, &item.rule.DeviceID, &item.rule.Name, &item.rule.DurationSeconds, &item.device, &item.status); err != nil {
			return err
		}
		item.rule.RuleType = RuleOffline
		item.rule.Enabled = true
		found = append(found, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, item := range found {
		if err := applyPresence(ctx, pool, item.rule, item.device, item.status, now, notify); err != nil {
			return err
		}
	}
	return nil
}

func enabledForProject(ctx context.Context, pool *pgxpool.Pool, projectID string) ([]Rule, error) {
	rows, err := pool.Query(ctx, `
		SELECT `+ruleSelect+`
		FROM alert_rules r
		WHERE r.project_id = $1 AND r.enabled
	`, database.ID(projectID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Rule{}
	for rows.Next() {
		rule, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rule)
	}
	return out, rows.Err()
}

func applyThreshold(ctx context.Context, pool *pgxpool.Pool, rule Rule, deviceID string, value float64, at time.Time, notify Notifier) error {
	firing := Matches(rule.Operator, value, *rule.ThresholdValue)
	ready, err := hold(ctx, pool, rule, deviceID, firing, at)
	if err != nil {
		return err
	}
	if !firing {
		_, err = resolveRule(ctx, pool, rule, deviceID, at, notify)
		return err
	}
	if !ready {
		return nil
	}
	message := fmt.Sprintf("%s: %s %s %g (%g)", rule.Name, rule.MetricKey, rule.Operator, *rule.ThresholdValue, value)
	meta, err := json.Marshal(map[string]any{
		"metric":    rule.MetricKey,
		"operator":  rule.Operator,
		"threshold": *rule.ThresholdValue,
		"value":     value,
	})
	if err != nil {
		return err
	}
	return trigger(ctx, pool, rule, deviceID, message, at, meta, notify)
}

func applyPresence(ctx context.Context, pool *pgxpool.Pool, rule Rule, deviceID, status string, at time.Time, notify Notifier) error {
	offline := status == "offline"
	ready, err := hold(ctx, pool, rule, deviceID, offline, at)
	if err != nil {
		return err
	}
	if !offline {
		_, err = resolveRule(ctx, pool, rule, deviceID, at, notify)
		return err
	}
	if !ready {
		return nil
	}
	name, err := deviceName(ctx, pool, deviceID)
	if err != nil {
		return err
	}
	message := fmt.Sprintf("%s: %s is offline", rule.Name, name)
	meta, err := json.Marshal(map[string]any{"status": "offline"})
	if err != nil {
		return err
	}
	return trigger(ctx, pool, rule, deviceID, message, at, meta, notify)
}

func hold(ctx context.Context, pool *pgxpool.Pool, rule Rule, deviceID string, firing bool, at time.Time) (bool, error) {
	if !firing {
		_, err := pool.Exec(ctx, `
			DELETE FROM alert_breaches WHERE alert_rule_id = $1 AND device_id = $2
		`, database.ID(rule.ID), database.ID(deviceID))
		return false, err
	}
	var since time.Time
	err := pool.QueryRow(ctx, `
		INSERT INTO alert_breaches (alert_rule_id, device_id, since)
		VALUES ($1, $2, $3)
		ON CONFLICT (alert_rule_id, device_id) DO UPDATE
		SET since = alert_breaches.since
		RETURNING since
	`, database.ID(rule.ID), database.ID(deviceID), at.UTC()).Scan(&since)
	if err != nil {
		return false, err
	}
	return at.UTC().Sub(since.UTC()) >= time.Duration(rule.DurationSeconds)*time.Second, nil
}

func trigger(ctx context.Context, pool *pgxpool.Pool, rule Rule, deviceID, message string, at time.Time, meta []byte, notify Notifier) error {
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO alert_events (alert_rule_id, device_id, status, message, triggered_at, metadata)
		SELECT $1, $2, 'triggered', $3, $4, $5
		WHERE NOT EXISTS (
			SELECT 1 FROM alert_events
			WHERE alert_rule_id = $1 AND device_id = $2 AND status = 'triggered'
		)
		RETURNING id::text
	`, database.ID(rule.ID), database.ID(deviceID), message, at.UTC(), meta).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) || isUnique(err) {
		return nil
	}
	if err != nil {
		return err
	}
	emit(notify, rule.ProjectID, deviceID, realtime.AlertTriggered, id, rule.ID, message, at)
	return nil
}

func resolveRule(ctx context.Context, pool *pgxpool.Pool, rule Rule, deviceID string, at time.Time, notify Notifier) (int, error) {
	var rows pgx.Rows
	var err error
	if deviceID == "" {
		rows, err = pool.Query(ctx, `
			UPDATE alert_events
			SET status = 'resolved', resolved_at = $2
			WHERE alert_rule_id = $1 AND status = 'triggered'
			RETURNING id::text, device_id::text, message
		`, database.ID(rule.ID), at.UTC())
	} else {
		rows, err = pool.Query(ctx, `
			UPDATE alert_events
			SET status = 'resolved', resolved_at = $3
			WHERE alert_rule_id = $1 AND device_id = $2 AND status = 'triggered'
			RETURNING id::text, device_id::text, message
		`, database.ID(rule.ID), database.ID(deviceID), at.UTC())
	}
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var id, resolvedDevice, message string
		if err := rows.Scan(&id, &resolvedDevice, &message); err != nil {
			return count, err
		}
		count++
		emit(notify, rule.ProjectID, resolvedDevice, realtime.AlertResolved, id, rule.ID, message, at)
	}
	return count, rows.Err()
}

func deviceName(ctx context.Context, pool *pgxpool.Pool, deviceID string) (string, error) {
	var name string
	err := pool.QueryRow(ctx, `SELECT name FROM devices WHERE id = $1`, database.ID(deviceID)).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "Device", nil
	}
	return name, err
}

func emit(notify Notifier, projectID, deviceID, eventType, eventID, ruleID, message string, at time.Time) {
	if notify == nil {
		return
	}
	notify(projectID, realtime.Event{
		Type:      eventType,
		DeviceID:  deviceID,
		Timestamp: at.UTC(),
		Data: map[string]any{
			"alertEventId": eventID,
			"ruleId":       ruleID,
			"message":      message,
			"status":       statusFromEvent(eventType),
		},
	})
}

func statusFromEvent(eventType string) string {
	if eventType == realtime.AlertResolved {
		return "resolved"
	}
	return "triggered"
}
