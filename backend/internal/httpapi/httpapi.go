// Package httpapi writes the shared JSON error envelope.
package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/access"
)

var idPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func ValidID(id string) bool {
	return idPattern.MatchString(id)
}

type envelope struct {
	Error body `json:"error"`
}

type body struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func Error(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, envelope{Error: body{Code: code, Message: message}})
}

func Validation(c *gin.Context, fields map[string]string) {
	c.AbortWithStatusJSON(http.StatusBadRequest, envelope{Error: body{
		Code:    "VALIDATION_ERROR",
		Message: "Invalid request",
		Fields:  fields,
	}})
}

func Internal(c *gin.Context, err error) {
	slog.Error("request failed",
		"err", err,
		"request_id", c.GetString("request_id"),
		"path", c.FullPath(),
	)
	Error(c, http.StatusInternalServerError, "INTERNAL", "Something went wrong")
}

// WriteRead maps a missing membership to 404. It returns true when it wrote a response.
func WriteRead(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, access.ErrNotFound) {
		Error(c, http.StatusNotFound, "NOT_FOUND", "Not found")
		return true
	}
	Internal(c, err)
	return true
}

func Forbidden(c *gin.Context) {
	Error(c, http.StatusForbidden, "FORBIDDEN", "You do not have permission to do that")
}
