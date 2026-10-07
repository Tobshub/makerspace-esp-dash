package metrics

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

var (
	ErrNotFound = access.ErrNotFound
	ErrConflict = errors.New("conflict")
)

// Definition is how a project displays one telemetry key.
type Definition struct {
	ID          string          `json:"id"`
	ProjectID   string          `json:"projectId"`
	Key         string          `json:"key"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	DataType    string          `json:"dataType"`
	Unit        string          `json:"unit"`
	DisplayType string          `json:"displayType"`
	MinValue    *float64        `json:"minValue"`
	MaxValue    *float64        `json:"maxValue"`
	Hidden      bool            `json:"hidden"`
	Metadata    json.RawMessage `json:"metadata"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
	LastSeenAt  *time.Time      `json:"lastSeenAt"`
}

const definitionSelect = `
	id::text, project_id::text, key, name, description, data_type, unit, display_type,
	min_value, max_value, hidden, metadata, created_at, updated_at,
	(SELECT max(t.recorded_at) FROM telemetry t
	 WHERE t.project_id = metric_definitions.project_id AND t.metric_key = metric_definitions.key)
`

// Discovered is a telemetry key that has no definition yet.
type Discovered struct {
	Key        string    `json:"key"`
	DataType   string    `json:"dataType"`
	LastSeenAt time.Time `json:"lastSeenAt"`
}

// Input is a create or update body after JSON decoding.
type Input struct {
	Key         string
	Name        string
	Description string
	DataType    string
	Unit        string
	DisplayType string
	MinValue    *float64
	MaxValue    *float64
	Hidden      bool
	Metadata    json.RawMessage
	checkKey    bool
}

func listDefinitions(ctx context.Context, pool *pgxpool.Pool, projectID string) ([]Definition, error) {
	rows, err := pool.Query(ctx, `
		SELECT `+definitionSelect+`
		FROM metric_definitions
		WHERE project_id = $1
		ORDER BY key
	`, database.ID(projectID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Definition{}
	for rows.Next() {
		item, err := scanDefinition(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func listDiscovered(ctx context.Context, pool *pgxpool.Pool, projectID string) ([]Discovered, error) {
	rows, err := pool.Query(ctx, `
		SELECT metric_key, data_type, recorded_at
		FROM (
			SELECT DISTINCT ON (t.metric_key)
				t.metric_key,
				CASE
					WHEN t.numeric_value IS NOT NULL THEN 'number'
					WHEN t.boolean_value IS NOT NULL THEN 'boolean'
					ELSE 'string'
				END AS data_type,
				t.recorded_at
			FROM telemetry t
			WHERE t.project_id = $1
			  AND NOT EXISTS (
				SELECT 1 FROM metric_definitions d
				WHERE d.project_id = t.project_id AND d.key = t.metric_key
			  )
			ORDER BY t.metric_key, t.recorded_at DESC, t.received_at DESC, t.id DESC
		) discovered
		ORDER BY metric_key
	`, database.ID(projectID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Discovered{}
	for rows.Next() {
		var item Discovered
		if err := rows.Scan(&item.Key, &item.DataType, &item.LastSeenAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func getDefinition(ctx context.Context, pool *pgxpool.Pool, metricID string) (Definition, error) {
	row := pool.QueryRow(ctx, `
		SELECT `+definitionSelect+`
		FROM metric_definitions
		WHERE id = $1
	`, database.ID(metricID))
	item, err := scanDefinition(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Definition{}, ErrNotFound
	}
	return item, err
}

func createDefinition(ctx context.Context, pool *pgxpool.Pool, projectID string, in Input) (Definition, error) {
	in = normalize(in)
	row := pool.QueryRow(ctx, `
		INSERT INTO metric_definitions (
			project_id, key, name, description, data_type, unit, display_type, min_value, max_value, hidden, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING `+definitionSelect+`
	`, database.ID(projectID), in.Key, in.Name, in.Description, in.DataType, in.Unit, in.DisplayType, in.MinValue, in.MaxValue, in.Hidden, in.Metadata)
	item, err := scanDefinition(row)
	if err == nil {
		return item, nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.ConstraintName == "metric_definitions_project_key_unique" {
		return Definition{}, ErrConflict
	}
	return Definition{}, err
}

func updateDefinition(ctx context.Context, pool *pgxpool.Pool, metricID string, in Input) (Definition, error) {
	in = normalize(in)
	row := pool.QueryRow(ctx, `
		UPDATE metric_definitions
		SET name = $2,
		    description = $3,
		    data_type = $4,
		    unit = $5,
		    display_type = $6,
		    min_value = $7,
		    max_value = $8,
		    hidden = $9,
		    metadata = $10,
		    updated_at = now()
		WHERE id = $1
		RETURNING `+definitionSelect+`
	`, database.ID(metricID), in.Name, in.Description, in.DataType, in.Unit, in.DisplayType, in.MinValue, in.MaxValue, in.Hidden, in.Metadata)
	item, err := scanDefinition(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Definition{}, ErrNotFound
	}
	return item, err
}

func deleteDefinition(ctx context.Context, pool *pgxpool.Pool, metricID string) error {
	tag, err := pool.Exec(ctx, `DELETE FROM metric_definitions WHERE id = $1`, database.ID(metricID))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanDefinition(row scanner) (Definition, error) {
	var item Definition
	err := row.Scan(
		&item.ID, &item.ProjectID, &item.Key, &item.Name, &item.Description,
		&item.DataType, &item.Unit, &item.DisplayType, &item.MinValue, &item.MaxValue,
		&item.Hidden, &item.Metadata, &item.CreatedAt, &item.UpdatedAt, &item.LastSeenAt,
	)
	return item, err
}
