package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/httpapi"
)

type credentialsBody struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

func register(c *gin.Context, pool *pgxpool.Pool) {
	if pool == nil {
		httpapi.Error(c, http.StatusServiceUnavailable, "UNAVAILABLE", "Database unavailable")
		return
	}
	var body credentialsBody
	if c.ShouldBindJSON(&body) != nil {
		httpapi.Validation(c, map[string]string{"body": "Request must be JSON"})
		return
	}
	email := normalizeEmail(body.Email)
	name := strings.TrimSpace(body.Name)
	fields := map[string]string{}
	if !strings.Contains(email, "@") || len(email) > 254 {
		fields["email"] = "Enter a valid email"
	}
	if name == "" || len(name) > 80 {
		fields["name"] = "Name is required"
	}
	if len(body.Password) < 8 || len(body.Password) > 72 {
		fields["password"] = "Password must be 8 to 72 characters"
	}
	if len(fields) > 0 {
		httpapi.Validation(c, fields)
		return
	}
	user, token, err := registerUser(c.Request.Context(), pool, email, name, body.Password)
	if errors.Is(err, ErrEmailTaken) {
		httpapi.Error(c, http.StatusConflict, "CONFLICT", "An account with that email already exists")
		return
	}
	if err != nil {
		httpapi.Internal(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"token": token, "user": user})
}

func login(c *gin.Context, pool *pgxpool.Pool) {
	if pool == nil {
		httpapi.Error(c, http.StatusServiceUnavailable, "UNAVAILABLE", "Database unavailable")
		return
	}
	var body credentialsBody
	if c.ShouldBindJSON(&body) != nil {
		httpapi.Validation(c, map[string]string{"body": "Request must be JSON"})
		return
	}
	email := normalizeEmail(body.Email)
	if email == "" || body.Password == "" {
		httpapi.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Email or password is incorrect")
		return
	}
	user, token, err := loginUser(c.Request.Context(), pool, email, body.Password)
	if errors.Is(err, ErrInvalidCredentials) {
		httpapi.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "Email or password is incorrect")
		return
	}
	if err != nil {
		httpapi.Internal(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "user": user})
}

func logout(c *gin.Context, pool *pgxpool.Pool) {
	raw := bearer(c.GetHeader("Authorization"))
	if raw != "" {
		if err := deleteSession(c.Request.Context(), pool, raw); err != nil {
			httpapi.Internal(c, err)
			return
		}
	}
	c.Status(http.StatusNoContent)
}

func me(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"user": Current(c)})
}
