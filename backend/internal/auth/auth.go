package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/httpapi"
)

const sessionTTL = 14 * 24 * time.Hour

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrEmailTaken = errors.New("email taken")

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func PublicRoutes(r *gin.RouterGroup, pool *pgxpool.Pool) {
	r.POST("/auth/register", func(c *gin.Context) { register(c, pool) })
	r.POST("/auth/login", func(c *gin.Context) { login(c, pool) })
}

func PrivateRoutes(r *gin.RouterGroup, pool *pgxpool.Pool) {
	r.POST("/auth/logout", func(c *gin.Context) { logout(c, pool) })
	r.GET("/auth/me", me)
}

func Require(pool *pgxpool.Pool) gin.HandlerFunc {
	return requireToken(pool, func(c *gin.Context) string {
		return bearer(c.GetHeader("Authorization"))
	})
}

// RequireStream accepts the bearer header or an access_token query value.
// EventSource cannot set Authorization, so the project stream uses the query.
func RequireStream(pool *pgxpool.Pool) gin.HandlerFunc {
	return requireToken(pool, streamToken)
}

func requireToken(pool *pgxpool.Pool, token func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if pool == nil {
			httpapi.Error(c, http.StatusServiceUnavailable, "UNAVAILABLE", "Database unavailable")
			return
		}
		raw := token(c)
		if raw == "" {
			httpapi.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Sign in required")
			return
		}
		user, err := userForToken(c.Request.Context(), pool, raw)
		if errors.Is(err, ErrInvalidCredentials) {
			httpapi.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Sign in required")
			return
		}
		if err != nil {
			httpapi.Internal(c, err)
			return
		}
		c.Set("user_id", user.ID)
		c.Set("user_email", user.Email)
		c.Set("user_name", user.Name)
		c.Next()
	}
}

func streamToken(c *gin.Context) string {
	if raw := bearer(c.GetHeader("Authorization")); raw != "" {
		return raw
	}
	raw := strings.TrimSpace(c.Query("access_token"))
	if raw == "" || len(raw) > 128 || strings.ContainsAny(raw, " \t\r\n") {
		return ""
	}
	return raw
}

func Current(c *gin.Context) User {
	return User{
		ID:    c.GetString("user_id"),
		Email: c.GetString("user_email"),
		Name:  c.GetString("user_name"),
	}
}

func bearer(header string) string {
	const prefix = "Bearer "
	if len(header) < len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}
	token := strings.TrimSpace(header[len(prefix):])
	if token == "" || len(token) > 128 {
		return ""
	}
	return token
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func hashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
