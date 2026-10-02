package teams

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/access"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/database"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/slug"
)

var ErrUserNotFound = errors.New("user not found")
var ErrConflict = errors.New("conflict")
var ErrLastOwner = errors.New("last owner")

type Team struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Member struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

func listTeams(ctx context.Context, pool *pgxpool.Pool, userID string) ([]Team, error) {
	rows, err := pool.Query(ctx, `
		SELECT t.id::text, t.name, t.slug, tm.role, t.created_at, t.updated_at
		FROM teams t
		JOIN team_members tm ON tm.team_id = t.id
		WHERE tm.user_id = $1
		ORDER BY t.name
	`, database.ID(userID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	teams := []Team{}
	for rows.Next() {
		var team Team
		if err := rows.Scan(&team.ID, &team.Name, &team.Slug, &team.Role, &team.CreatedAt, &team.UpdatedAt); err != nil {
			return nil, err
		}
		teams = append(teams, team)
	}
	return teams, rows.Err()
}

func createTeam(ctx context.Context, pool *pgxpool.Pool, userID, name string) (Team, error) {
	base := slug.FromName(name)
	var last error
	for i := 0; i < 20; i++ {
		candidate := base
		if i > 0 {
			candidate = fmt.Sprintf("%s-%d", base, i+1)
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return Team{}, err
		}
		var team Team
		err = tx.QueryRow(ctx, `
			INSERT INTO teams (name, slug) VALUES ($1, $2)
			RETURNING id::text, name, slug, created_at, updated_at
		`, name, candidate).Scan(&team.ID, &team.Name, &team.Slug, &team.CreatedAt, &team.UpdatedAt)
		if err == nil {
			_, err = tx.Exec(ctx, `
				INSERT INTO team_members (team_id, user_id, role) VALUES ($1, $2, 'owner')
			`, database.ID(team.ID), database.ID(userID))
		}
		if err == nil {
			if err = tx.Commit(ctx); err != nil {
				return Team{}, err
			}
			team.Role = access.RoleOwner
			return team, nil
		}
		_ = tx.Rollback(ctx)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.ConstraintName == "teams_slug_unique" {
			last = err
			continue
		}
		return Team{}, err
	}
	if last == nil {
		last = errors.New("slug exhausted")
	}
	return Team{}, last
}

func getTeam(ctx context.Context, pool *pgxpool.Pool, userID, teamID string) (Team, error) {
	var team Team
	err := pool.QueryRow(ctx, `
		SELECT t.id::text, t.name, t.slug, tm.role, t.created_at, t.updated_at
		FROM teams t
		JOIN team_members tm ON tm.team_id = t.id AND tm.user_id = $2
		WHERE t.id = $1
	`, database.ID(teamID), database.ID(userID)).Scan(&team.ID, &team.Name, &team.Slug, &team.Role, &team.CreatedAt, &team.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Team{}, access.ErrNotFound
	}
	return team, err
}

func updateTeam(ctx context.Context, pool *pgxpool.Pool, userID, teamID, name string) (Team, error) {
	tag, err := pool.Exec(ctx, `
		UPDATE teams SET name = $3, updated_at = now()
		WHERE id = $1 AND EXISTS (
			SELECT 1 FROM team_members WHERE team_id = $1 AND user_id = $2
		)
	`, database.ID(teamID), database.ID(userID), name)
	if err != nil {
		return Team{}, err
	}
	if tag.RowsAffected() == 0 {
		return Team{}, access.ErrNotFound
	}
	return getTeam(ctx, pool, userID, teamID)
}

func deleteTeam(ctx context.Context, pool *pgxpool.Pool, userID, teamID string) error {
	tag, err := pool.Exec(ctx, `
		DELETE FROM teams WHERE id = $1 AND EXISTS (
			SELECT 1 FROM team_members
			WHERE team_id = $1 AND user_id = $2 AND role = 'owner'
		)
	`, database.ID(teamID), database.ID(userID))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		role, err := access.TeamRole(ctx, pool, userID, teamID)
		if err != nil {
			return err
		}
		if role != access.RoleOwner {
			return errNotOwner
		}
		return access.ErrNotFound
	}
	return nil
}

var errNotOwner = errors.New("not owner")

func listMembers(ctx context.Context, pool *pgxpool.Pool, teamID string) ([]Member, error) {
	rows, err := pool.Query(ctx, `
		SELECT tm.id::text, u.id::text, u.email, u.name, tm.role, tm.created_at
		FROM team_members tm
		JOIN users u ON u.id = tm.user_id
		WHERE tm.team_id = $1
		ORDER BY u.email
	`, database.ID(teamID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	members := []Member{}
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.ID, &m.UserID, &m.Email, &m.Name, &m.Role, &m.CreatedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func addMember(ctx context.Context, pool *pgxpool.Pool, teamID, email, role string) (Member, error) {
	var userID string
	err := pool.QueryRow(ctx, `SELECT id::text FROM users WHERE email = $1`, strings.ToLower(strings.TrimSpace(email))).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Member{}, ErrUserNotFound
	}
	if err != nil {
		return Member{}, err
	}
	var m Member
	err = pool.QueryRow(ctx, `
		INSERT INTO team_members (team_id, user_id, role)
		VALUES ($1, $2, $3)
		RETURNING id::text, role, created_at
	`, database.ID(teamID), database.ID(userID), role).Scan(&m.ID, &m.Role, &m.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.ConstraintName == "team_members_team_user_unique" {
			return Member{}, ErrConflict
		}
		return Member{}, err
	}
	m.UserID = userID
	err = pool.QueryRow(ctx, `SELECT email, name FROM users WHERE id = $1`, database.ID(userID)).Scan(&m.Email, &m.Name)
	return m, err
}

func memberRole(ctx context.Context, pool *pgxpool.Pool, teamID, memberID string) (string, error) {
	var role string
	err := pool.QueryRow(ctx, `
		SELECT role FROM team_members WHERE id = $1 AND team_id = $2
	`, database.ID(memberID), database.ID(teamID)).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", access.ErrNotFound
	}
	return role, err
}

func ownerCount(ctx context.Context, pool *pgxpool.Pool, teamID string) (int, error) {
	var n int
	err := pool.QueryRow(ctx, `
		SELECT count(*) FROM team_members WHERE team_id = $1 AND role = 'owner'
	`, database.ID(teamID)).Scan(&n)
	return n, err
}

func updateMemberRole(ctx context.Context, pool *pgxpool.Pool, teamID, memberID, role string) (Member, error) {
	current, err := memberRole(ctx, pool, teamID, memberID)
	if err != nil {
		return Member{}, err
	}
	if current == access.RoleOwner && role != access.RoleOwner {
		n, err := ownerCount(ctx, pool, teamID)
		if err != nil {
			return Member{}, err
		}
		if n <= 1 {
			return Member{}, ErrLastOwner
		}
	}
	tag, err := pool.Exec(ctx, `
		UPDATE team_members SET role = $3 WHERE id = $1 AND team_id = $2
	`, database.ID(memberID), database.ID(teamID), role)
	if err != nil {
		return Member{}, err
	}
	if tag.RowsAffected() == 0 {
		return Member{}, access.ErrNotFound
	}
	return getMember(ctx, pool, teamID, memberID)
}

func removeMember(ctx context.Context, pool *pgxpool.Pool, teamID, memberID string) error {
	current, err := memberRole(ctx, pool, teamID, memberID)
	if err != nil {
		return err
	}
	if current == access.RoleOwner {
		n, err := ownerCount(ctx, pool, teamID)
		if err != nil {
			return err
		}
		if n <= 1 {
			return ErrLastOwner
		}
	}
	tag, err := pool.Exec(ctx, `DELETE FROM team_members WHERE id = $1 AND team_id = $2`, database.ID(memberID), database.ID(teamID))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return access.ErrNotFound
	}
	return nil
}

func getMember(ctx context.Context, pool *pgxpool.Pool, teamID, memberID string) (Member, error) {
	var m Member
	err := pool.QueryRow(ctx, `
		SELECT tm.id::text, u.id::text, u.email, u.name, tm.role, tm.created_at
		FROM team_members tm
		JOIN users u ON u.id = tm.user_id
		WHERE tm.id = $1 AND tm.team_id = $2
	`, database.ID(memberID), database.ID(teamID)).Scan(&m.ID, &m.UserID, &m.Email, &m.Name, &m.Role, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Member{}, access.ErrNotFound
	}
	return m, err
}
