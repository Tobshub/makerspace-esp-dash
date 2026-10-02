package readings

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/access"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/database"
)

// Value is the latest stored scalar for one metric.
type Value struct {
	Metric       string    `json:"metric"`
	NumericValue *float64  `json:"numericValue"`
	BooleanValue *bool     `json:"booleanValue"`
	StringValue  *string   `json:"stringValue"`
	RecordedAt   time.Time `json:"recordedAt"`
	ReceivedAt   time.Time `json:"receivedAt"`
}

// Point is one sample in a history series.
type Point struct {
	NumericValue *float64  `json:"numericValue"`
	BooleanValue *bool     `json:"booleanValue"`
	StringValue  *string   `json:"stringValue"`
	RecordedAt   time.Time `json:"recordedAt"`
	ReceivedAt   time.Time `json:"receivedAt"`
}

// DeviceLatest is every metric's newest value for one device.
type DeviceLatest struct {
	DeviceID string  `json:"deviceId"`
	Metrics  []Value `json:"metrics"`
}

// Series is a history response. Points are oldest first.
// Truncated means the window holds more than Limit points; the newest Limit are kept.
type Series struct {
	DeviceID   string    `json:"deviceId"`
	Metric     string    `json:"metric"`
	Resolution string    `json:"resolution"`
	From       time.Time `json:"from"`
	To         time.Time `json:"to"`
	Limit      int       `json:"limit"`
	Truncated  bool      `json:"truncated"`
	Points     []Point   `json:"points"`
}

// DeviceOverview is one device inside a project latest response.
type DeviceOverview struct {
	DeviceID  string  `json:"deviceId"`
	DeviceKey string  `json:"deviceKey"`
	Name      string  `json:"name"`
	Metrics   []Value `json:"metrics"`
}

// ProjectLatest is the newest value of each metric on each device in a project.
type ProjectLatest struct {
	ProjectID string           `json:"projectId"`
	Devices   []DeviceOverview `json:"devices"`
}

func deviceProject(ctx context.Context, pool *pgxpool.Pool, deviceID string) (string, error) {
	var projectID string
	err := pool.QueryRow(ctx, `SELECT project_id::text FROM devices WHERE id = $1`, database.ID(deviceID)).Scan(&projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", access.ErrNotFound
	}
	return projectID, err
}

func latestDevice(ctx context.Context, pool *pgxpool.Pool, deviceID string) (DeviceLatest, error) {
	rows, err := pool.Query(ctx, `
		SELECT DISTINCT ON (metric_key)
			metric_key, numeric_value, boolean_value, string_value, recorded_at, received_at
		FROM telemetry
		WHERE device_id = $1
		ORDER BY metric_key, recorded_at DESC, received_at DESC, id DESC
	`, database.ID(deviceID))
	if err != nil {
		return DeviceLatest{}, err
	}
	defer rows.Close()
	out := DeviceLatest{DeviceID: deviceID, Metrics: []Value{}}
	for rows.Next() {
		value, err := scanValue(rows)
		if err != nil {
			return DeviceLatest{}, err
		}
		out.Metrics = append(out.Metrics, value)
	}
	return out, rows.Err()
}

func history(ctx context.Context, pool *pgxpool.Pool, deviceID string, q HistoryQuery) (Series, error) {
	rows, err := pool.Query(ctx, `
		SELECT numeric_value, boolean_value, string_value, recorded_at, received_at
		FROM telemetry
		WHERE device_id = $1
		  AND metric_key = $2
		  AND recorded_at >= $3
		  AND recorded_at <= $4
		ORDER BY recorded_at DESC, received_at DESC, id DESC
		LIMIT $5
	`, database.ID(deviceID), q.Metric, q.From, q.To, q.Limit+1)
	if err != nil {
		return Series{}, err
	}
	defer rows.Close()
	points := []Point{}
	for rows.Next() {
		var point Point
		if err := rows.Scan(&point.NumericValue, &point.BooleanValue, &point.StringValue, &point.RecordedAt, &point.ReceivedAt); err != nil {
			return Series{}, err
		}
		point.RecordedAt = point.RecordedAt.UTC()
		point.ReceivedAt = point.ReceivedAt.UTC()
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return Series{}, err
	}
	truncated := len(points) > q.Limit
	if truncated {
		points = points[:q.Limit]
	}
	for i, j := 0, len(points)-1; i < j; i, j = i+1, j-1 {
		points[i], points[j] = points[j], points[i]
	}
	return Series{
		DeviceID:   deviceID,
		Metric:     q.Metric,
		Resolution: q.Resolution,
		From:       q.From,
		To:         q.To,
		Limit:      q.Limit,
		Truncated:  truncated,
		Points:     points,
	}, nil
}

func latestProject(ctx context.Context, pool *pgxpool.Pool, projectID string) (ProjectLatest, error) {
	rows, err := pool.Query(ctx, `
		SELECT d.id::text, d.device_key, d.name,
		       t.metric_key, t.numeric_value, t.boolean_value, t.string_value, t.recorded_at, t.received_at
		FROM devices d
		LEFT JOIN LATERAL (
			SELECT DISTINCT ON (metric_key)
				metric_key, numeric_value, boolean_value, string_value, recorded_at, received_at
			FROM telemetry
			WHERE device_id = d.id
			ORDER BY metric_key, recorded_at DESC, received_at DESC, id DESC
		) t ON true
		WHERE d.project_id = $1
		ORDER BY d.created_at DESC, d.id, t.metric_key
	`, database.ID(projectID))
	if err != nil {
		return ProjectLatest{}, err
	}
	defer rows.Close()
	out := ProjectLatest{ProjectID: projectID, Devices: []DeviceOverview{}}
	index := map[string]int{}
	for rows.Next() {
		var id, key, name string
		var metric *string
		var numeric *float64
		var boolean *bool
		var text *string
		var recorded, received *time.Time
		if err := rows.Scan(&id, &key, &name, &metric, &numeric, &boolean, &text, &recorded, &received); err != nil {
			return ProjectLatest{}, err
		}
		pos, ok := index[id]
		if !ok {
			out.Devices = append(out.Devices, DeviceOverview{
				DeviceID:  id,
				DeviceKey: key,
				Name:      name,
				Metrics:   []Value{},
			})
			pos = len(out.Devices) - 1
			index[id] = pos
		}
		if metric == nil || recorded == nil || received == nil {
			continue
		}
		out.Devices[pos].Metrics = append(out.Devices[pos].Metrics, Value{
			Metric:       *metric,
			NumericValue: numeric,
			BooleanValue: boolean,
			StringValue:  text,
			RecordedAt:   recorded.UTC(),
			ReceivedAt:   received.UTC(),
		})
	}
	return out, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanValue(row rowScanner) (Value, error) {
	var value Value
	if err := row.Scan(&value.Metric, &value.NumericValue, &value.BooleanValue, &value.StringValue, &value.RecordedAt, &value.ReceivedAt); err != nil {
		return Value{}, err
	}
	value.RecordedAt = value.RecordedAt.UTC()
	value.ReceivedAt = value.ReceivedAt.UTC()
	return value, nil
}
