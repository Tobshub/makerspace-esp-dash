package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/database"
)

func registerUser(ctx context.Context, pool *pgxpool.Pool, email, name, password string) (User, string, error) {
	hash, err := hashPassword(password)
	if err != nil {
		return User{}, "", err
	}
	var user User
	err = pool.QueryRow(ctx, `
		INSERT INTO users (email, name, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id::text, email, name, created_at, updated_at
	`, email, name, hash).Scan(&user.ID, &user.Email, &user.Name, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.ConstraintName == "users_email_unique" {
			return User{}, "", ErrEmailTaken
		}
		return User{}, "", err
	}
	token, err := insertSession(ctx, pool, user.ID)
	if err != nil {
		return User{}, "", err
	}
	return user, token, nil
}

func loginUser(ctx context.Context, pool *pgxpool.Pool, email, password string) (User, string, error) {
	var user User
	var hash string
	err := pool.QueryRow(ctx, `
		SELECT id::text, email, name, password_hash, created_at, updated_at
		FROM users WHERE email = $1
	`, email).Scan(&user.ID, &user.Email, &user.Name, &hash, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, "", ErrInvalidCredentials
	}
	if err != nil {
		return User{}, "", err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return User{}, "", ErrInvalidCredentials
	}
	token, err := insertSession(ctx, pool, user.ID)
	if err != nil {
		return User{}, "", err
	}
	return user, token, nil
}

func insertSession(ctx context.Context, pool *pgxpool.Pool, userID string) (string, error) {
	if _, err := pool.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1 AND expires_at < now()`, database.ID(userID)); err != nil {
		return "", err
	}
	token, err := newToken()
	if err != nil {
		return "", err
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO sessions (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, database.ID(userID), hashToken(token), time.Now().UTC().Add(sessionTTL))
	if err != nil {
		return "", err
	}
	return token, nil
}

func userForToken(ctx context.Context, pool *pgxpool.Pool, token string) (User, error) {
	var user User
	err := pool.QueryRow(ctx, `
		SELECT u.id::text, u.email, u.name
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > now()
	`, hashToken(token)).Scan(&user.ID, &user.Email, &user.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func deleteSession(ctx context.Context, pool *pgxpool.Pool, token string) error {
	_, err := pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, hashToken(token))
	return err
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
