package controls

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

// Control is one dashboard input mapped to a command.
type Control struct {
	ID            string          `json:"id"`
	ProjectID     string          `json:"projectId"`
	Name          string          `json:"name"`
	Key           string          `json:"key"`
	ControlType   string          `json:"controlType"`
	Command       string          `json:"command"`
	Configuration json.RawMessage `json:"configuration"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
}

const controlSelect = `
	id::text, project_id::text, name, key, control_type, command, configuration, created_at, updated_at
`

func scanControl(row pgx.Row) (Control, error) {
	var item Control
	err := row.Scan(&item.ID, &item.ProjectID, &item.Name, &item.Key, &item.ControlType, &item.Command, &item.Configuration, &item.CreatedAt, &item.UpdatedAt)
	return item, err
}

func list(ctx context.Context, pool *pgxpool.Pool, projectID string) ([]Control, error) {
	rows, err := pool.Query(ctx, `
		SELECT `+controlSelect+`
		FROM control_definitions
		WHERE project_id = $1
		ORDER BY name, key
	`, database.ID(projectID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Control{}
	for rows.Next() {
		item, err := scanControl(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func create(ctx context.Context, pool *pgxpool.Pool, projectID string, in Input) (Control, error) {
	item, err := scanControl(pool.QueryRow(ctx, `
		INSERT INTO control_definitions (project_id, name, key, control_type, command, configuration)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+controlSelect+`
	`, database.ID(projectID), in.Name, in.Key, in.ControlType, in.Command, in.Configuration))
	if isUnique(err) {
		return Control{}, ErrConflict
	}
	return item, err
}

func get(ctx context.Context, pool *pgxpool.Pool, id string) (Control, error) {
	item, err := scanControl(pool.QueryRow(ctx, `
		SELECT `+controlSelect+` FROM control_definitions WHERE id = $1
	`, database.ID(id)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Control{}, ErrNotFound
	}
	return item, err
}

func update(ctx context.Context, pool *pgxpool.Pool, id string, in Input) (Control, error) {
	item, err := scanControl(pool.QueryRow(ctx, `
		UPDATE control_definitions
		SET name = $2, control_type = $3, command = $4, configuration = $5, updated_at = now()
		WHERE id = $1
		RETURNING `+controlSelect+`
	`, database.ID(id), in.Name, in.ControlType, in.Command, in.Configuration))
	if errors.Is(err, pgx.ErrNoRows) {
		return Control{}, ErrNotFound
	}
	return item, err
}

func delete(ctx context.Context, pool *pgxpool.Pool, id string) error {
	tag, err := pool.Exec(ctx, `DELETE FROM control_definitions WHERE id = $1`, database.ID(id))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
