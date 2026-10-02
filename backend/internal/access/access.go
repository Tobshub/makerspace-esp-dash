// Package access checks team membership for project-scoped requests.
package access

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/database"
)

const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
	RoleViewer = "viewer"
)

var ErrNotFound = errors.New("not found")

// Membership is the caller's role on a team or project.
type Membership struct {
	TeamID string
	Role   string
}

func ValidRole(role string) bool {
	switch role {
	case RoleOwner, RoleAdmin, RoleMember, RoleViewer:
		return true
	default:
		return false
	}
}

func CanWrite(role string) bool {
	return role == RoleOwner || role == RoleAdmin || role == RoleMember
}

func CanManage(role string) bool {
	return role == RoleOwner || role == RoleAdmin
}

// TeamRole returns the caller's role. Missing membership is ErrNotFound,
// including when the team itself does not exist.
func TeamRole(ctx context.Context, pool *pgxpool.Pool, userID, teamID string) (string, error) {
	var role string
	err := pool.QueryRow(ctx, `
		SELECT role FROM team_members WHERE team_id = $1 AND user_id = $2
	`, database.ID(teamID), database.ID(userID)).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return role, nil
}

// Project returns the owning team and the caller's role.
func Project(ctx context.Context, pool *pgxpool.Pool, userID, projectID string) (Membership, error) {
	var m Membership
	err := pool.QueryRow(ctx, `
		SELECT p.team_id::text, tm.role
		FROM projects p
		JOIN team_members tm ON tm.team_id = p.team_id AND tm.user_id = $2
		WHERE p.id = $1
	`, database.ID(projectID), database.ID(userID)).Scan(&m.TeamID, &m.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		return Membership{}, ErrNotFound
	}
	if err != nil {
		return Membership{}, err
	}
	return m, nil
}
