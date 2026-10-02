package alerts

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/access"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/database"
)

var ErrNotFound = access.ErrNotFound

// Rule is one configured alert.
type Rule struct {
	ID              string          `json:"id"`
	ProjectID       string          `json:"projectId"`
	DeviceID        string          `json:"deviceId"`
	MetricKey       string          `json:"metricKey"`
	Name            string          `json:"name"`
	RuleType        string          `json:"ruleType"`
	Operator        string          `json:"operator"`
	ThresholdValue  *float64        `json:"thresholdValue"`
	DurationSeconds int             `json:"durationSeconds"`
	Enabled         bool            `json:"enabled"`
	Configuration   json.RawMessage `json:"configuration"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
	Active          bool            `json:"active"`
}

// Event is one trigger or resolution.
type Event struct {
	ID          string          `json:"id"`
	AlertRuleID string          `json:"alertRuleId"`
	RuleName    string          `json:"ruleName"`
	DeviceID    string          `json:"deviceId"`
	DeviceName  string          `json:"deviceName"`
	Status      string          `json:"status"`
	Message     string          `json:"message"`
	TriggeredAt time.Time       `json:"triggeredAt"`
	ResolvedAt  *time.Time      `json:"resolvedAt"`
	Metadata    json.RawMessage `json:"metadata"`
}

const ruleSelect = `
	r.id::text, r.project_id::text, COALESCE(r.device_id::text, ''), r.metric_key, r.name, r.rule_type,
	r.operator, r.threshold_value, r.duration_seconds, r.enabled, r.configuration, r.created_at, r.updated_at,
	EXISTS (
		SELECT 1 FROM alert_events e
		WHERE e.alert_rule_id = r.id AND e.status = 'triggered'
	)
`

func scanRule(row pgx.Row) (Rule, error) {
	var rule Rule
	err := row.Scan(
		&rule.ID, &rule.ProjectID, &rule.DeviceID, &rule.MetricKey, &rule.Name, &rule.RuleType,
		&rule.Operator, &rule.ThresholdValue, &rule.DurationSeconds, &rule.Enabled, &rule.Configuration,
		&rule.CreatedAt, &rule.UpdatedAt, &rule.Active,
	)
	return rule, err
}

func listRules(ctx context.Context, pool *pgxpool.Pool, projectID string) ([]Rule, error) {
	rows, err := pool.Query(ctx, `
		SELECT `+ruleSelect+`
		FROM alert_rules r
		WHERE r.project_id = $1
		ORDER BY r.name, r.id
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

func createRule(ctx context.Context, pool *pgxpool.Pool, projectID string, in Input) (Rule, error) {
	if err := deviceInProject(ctx, pool, projectID, in.DeviceID); err != nil {
		return Rule{}, err
	}
	rule, err := scanRule(pool.QueryRow(ctx, `
		INSERT INTO alert_rules (
			project_id, device_id, metric_key, name, rule_type, operator, threshold_value,
			duration_seconds, enabled, configuration
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING `+stringsReplaceRule()+`
	`, database.ID(projectID), deviceArg(in.DeviceID), in.MetricKey, in.Name, in.RuleType, in.Operator, in.ThresholdValue, in.DurationSeconds, in.Enabled, in.Configuration))
	return rule, err
}

func stringsReplaceRule() string {
	return `
		id::text, project_id::text, COALESCE(device_id::text, ''), metric_key, name, rule_type,
		operator, threshold_value, duration_seconds, enabled, configuration, created_at, updated_at,
		false
	`
}

func getRule(ctx context.Context, pool *pgxpool.Pool, id string) (Rule, error) {
	rule, err := scanRule(pool.QueryRow(ctx, `
		SELECT `+ruleSelect+` FROM alert_rules r WHERE r.id = $1
	`, database.ID(id)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Rule{}, ErrNotFound
	}
	return rule, err
}

func updateRule(ctx context.Context, pool *pgxpool.Pool, id string, in Input) (Rule, error) {
	current, err := getRule(ctx, pool, id)
	if err != nil {
		return Rule{}, err
	}
	if err := deviceInProject(ctx, pool, current.ProjectID, in.DeviceID); err != nil {
		return Rule{}, err
	}
	tag, err := pool.Exec(ctx, `
		UPDATE alert_rules
		SET device_id = $2, metric_key = $3, name = $4, rule_type = $5, operator = $6,
		    threshold_value = $7, duration_seconds = $8, enabled = $9, configuration = $10, updated_at = now()
		WHERE id = $1
	`, database.ID(id), deviceArg(in.DeviceID), in.MetricKey, in.Name, in.RuleType, in.Operator, in.ThresholdValue, in.DurationSeconds, in.Enabled, in.Configuration)
	if err != nil {
		return Rule{}, err
	}
	if tag.RowsAffected() == 0 {
		return Rule{}, ErrNotFound
	}
	rule, err := getRule(ctx, pool, id)
	if err != nil {
		return Rule{}, err
	}
	if !in.Enabled {
		if _, err := resolveRule(ctx, pool, rule, "", time.Now().UTC(), nil); err != nil {
			return Rule{}, err
		}
		rule.Active = false
	}
	return rule, nil
}

func deleteRule(ctx context.Context, pool *pgxpool.Pool, id string) error {
	tag, err := pool.Exec(ctx, `DELETE FROM alert_rules WHERE id = $1`, database.ID(id))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func listEvents(ctx context.Context, pool *pgxpool.Pool, projectID string, limit int) ([]Event, error) {
	rows, err := pool.Query(ctx, `
		SELECT e.id::text, e.alert_rule_id::text, r.name, e.device_id::text, d.name,
		       e.status, e.message, e.triggered_at, e.resolved_at, e.metadata
		FROM alert_events e
		JOIN alert_rules r ON r.id = e.alert_rule_id
		JOIN devices d ON d.id = e.device_id
		WHERE r.project_id = $1
		ORDER BY e.triggered_at DESC, e.id DESC
		LIMIT $2
	`, database.ID(projectID), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var item Event
		if err := rows.Scan(&item.ID, &item.AlertRuleID, &item.RuleName, &item.DeviceID, &item.DeviceName, &item.Status, &item.Message, &item.TriggeredAt, &item.ResolvedAt, &item.Metadata); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func deviceInProject(ctx context.Context, pool *pgxpool.Pool, projectID, deviceID string) error {
	if deviceID == "" {
		return nil
	}
	var found bool
	err := pool.QueryRow(ctx, `
		SELECT true FROM devices WHERE id = $1 AND project_id = $2
	`, database.ID(deviceID), database.ID(projectID)).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		return errDevice
	}
	return err
}

func deviceArg(id string) any {
	if id == "" {
		return nil
	}
	return database.ID(id)
}

var errDevice = errors.New("device not in project")

func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
