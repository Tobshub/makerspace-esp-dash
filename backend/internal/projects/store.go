package projects

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/access"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/database"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/slug"
)

type Project struct {
	ID          string    `json:"id"`
	TeamID      string    `json:"teamId"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func listProjects(ctx context.Context, pool *pgxpool.Pool, userID, teamID string) ([]Project, error) {
	if _, err := access.TeamRole(ctx, pool, userID, teamID); err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `
		SELECT id::text, team_id::text, name, slug, description, created_at, updated_at
		FROM projects
		WHERE team_id = $1
		ORDER BY name
	`, database.ID(teamID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Project{}
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.TeamID, &p.Name, &p.Slug, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func createProject(ctx context.Context, pool *pgxpool.Pool, userID, teamID, name, description string) (Project, error) {
	if _, err := access.TeamRole(ctx, pool, userID, teamID); err != nil {
		return Project{}, err
	}
	base := slug.FromName(name)
	var last error
	for i := 0; i < 20; i++ {
		candidate := base
		if i > 0 {
			candidate = fmt.Sprintf("%s-%d", base, i+1)
		}
		var p Project
		err := pool.QueryRow(ctx, `
			INSERT INTO projects (team_id, name, slug, description)
			VALUES ($1, $2, $3, $4)
			RETURNING id::text, team_id::text, name, slug, description, created_at, updated_at
		`, database.ID(teamID), name, candidate, description).Scan(
			&p.ID, &p.TeamID, &p.Name, &p.Slug, &p.Description, &p.CreatedAt, &p.UpdatedAt,
		)
		if err == nil {
			return p, nil
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.ConstraintName == "projects_team_slug_unique" {
			last = err
			continue
		}
		return Project{}, err
	}
	if last == nil {
		last = errors.New("slug exhausted")
	}
	return Project{}, last
}

func getProject(ctx context.Context, pool *pgxpool.Pool, userID, projectID string) (Project, error) {
	var p Project
	err := pool.QueryRow(ctx, `
		SELECT p.id::text, p.team_id::text, p.name, p.slug, p.description, p.created_at, p.updated_at
		FROM projects p
		JOIN team_members tm ON tm.team_id = p.team_id AND tm.user_id = $2
		WHERE p.id = $1
	`, database.ID(projectID), database.ID(userID)).Scan(&p.ID, &p.TeamID, &p.Name, &p.Slug, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Project{}, access.ErrNotFound
	}
	return p, err
}

func updateProject(ctx context.Context, pool *pgxpool.Pool, userID, projectID, name, description string) (Project, error) {
	tag, err := pool.Exec(ctx, `
		UPDATE projects SET name = $3, description = $4, updated_at = now()
		WHERE id = $1 AND EXISTS (
			SELECT 1
			FROM projects p
			JOIN team_members tm ON tm.team_id = p.team_id AND tm.user_id = $2
			WHERE p.id = $1
		)
	`, database.ID(projectID), database.ID(userID), name, description)
	if err != nil {
		return Project{}, err
	}
	if tag.RowsAffected() == 0 {
		return Project{}, access.ErrNotFound
	}
	return getProject(ctx, pool, userID, projectID)
}

func deleteProject(ctx context.Context, pool *pgxpool.Pool, userID, projectID string) error {
	tag, err := pool.Exec(ctx, `
		DELETE FROM projects WHERE id = $1 AND EXISTS (
			SELECT 1
			FROM projects p
			JOIN team_members tm ON tm.team_id = p.team_id AND tm.user_id = $2
			WHERE p.id = $1 AND tm.role IN ('owner', 'admin')
		)
	`, database.ID(projectID), database.ID(userID))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return access.ErrNotFound
	}
	return nil
}
