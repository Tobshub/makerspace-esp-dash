package controls

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/access"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/auth"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/httpapi"
)

func Register(r *gin.RouterGroup, pool *pgxpool.Pool) {
	r.GET("/projects/:projectId/controls", func(c *gin.Context) { handleList(c, pool) })
	r.POST("/projects/:projectId/controls", func(c *gin.Context) { handleCreate(c, pool) })
	r.PATCH("/controls/:controlId", func(c *gin.Context) { handleUpdate(c, pool) })
	r.DELETE("/controls/:controlId", func(c *gin.Context) { handleDelete(c, pool) })
}

type body struct {
	Key           string          `json:"key"`
	Name          string          `json:"name"`
	ControlType   string          `json:"controlType"`
	Command       string          `json:"command"`
	Configuration json.RawMessage `json:"configuration"`
}

func handleList(c *gin.Context, pool *pgxpool.Pool) {
	if _, ok := authorizeProject(c, pool, false); !ok {
		return
	}
	items, err := list(c.Request.Context(), pool, c.Param("projectId"))
	if httpapi.WriteRead(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"controls": items})
}

func handleCreate(c *gin.Context, pool *pgxpool.Pool) {
	if _, ok := authorizeProject(c, pool, true); !ok {
		return
	}
	in, ok := readBody(c, true)
	if !ok {
		return
	}
	item, err := create(c.Request.Context(), pool, c.Param("projectId"), in)
	if errors.Is(err, ErrConflict) {
		httpapi.Error(c, http.StatusConflict, "CONFLICT", "A control with this key already exists")
		return
	}
	if err != nil {
		httpapi.Internal(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"control": item})
}

func handleUpdate(c *gin.Context, pool *pgxpool.Pool) {
	current, ok := loadAuthorized(c, pool, true)
	if !ok {
		return
	}
	in, ok := readBody(c, false)
	if !ok {
		return
	}
	in.Key = current.Key
	item, err := update(c.Request.Context(), pool, current.ID, in)
	if httpapi.WriteRead(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"control": item})
}

func handleDelete(c *gin.Context, pool *pgxpool.Pool) {
	current, ok := loadAuthorized(c, pool, true)
	if !ok {
		return
	}
	if err := delete(c.Request.Context(), pool, current.ID); httpapi.WriteRead(c, err) {
		return
	}
	c.Status(http.StatusNoContent)
}

func readBody(c *gin.Context, checkKey bool) (Input, bool) {
	var raw body
	if err := c.ShouldBindJSON(&raw); err != nil {
		httpapi.Validation(c, map[string]string{"body": "Request must be JSON"})
		return Input{}, false
	}
	in, fields := validate(Input{
		Key: raw.Key, Name: raw.Name, ControlType: raw.ControlType, Command: raw.Command,
		Configuration: raw.Configuration, checkKey: checkKey,
	})
	if len(fields) > 0 {
		httpapi.Validation(c, fields)
		return Input{}, false
	}
	return in, true
}

func authorizeProject(c *gin.Context, pool *pgxpool.Pool, write bool) (access.Membership, bool) {
	if !httpapi.ValidID(c.Param("projectId")) {
		httpapi.Error(c, http.StatusNotFound, "NOT_FOUND", "Not found")
		return access.Membership{}, false
	}
	membership, err := access.Project(c.Request.Context(), pool, auth.Current(c).ID, c.Param("projectId"))
	if httpapi.WriteRead(c, err) {
		return access.Membership{}, false
	}
	if write && !access.CanWrite(membership.Role) {
		httpapi.Forbidden(c)
		return access.Membership{}, false
	}
	return membership, true
}

func loadAuthorized(c *gin.Context, pool *pgxpool.Pool, write bool) (Control, bool) {
	if !httpapi.ValidID(c.Param("controlId")) {
		httpapi.Error(c, http.StatusNotFound, "NOT_FOUND", "Not found")
		return Control{}, false
	}
	item, err := get(c.Request.Context(), pool, c.Param("controlId"))
	if httpapi.WriteRead(c, err) {
		return Control{}, false
	}
	membership, err := access.Project(c.Request.Context(), pool, auth.Current(c).ID, item.ProjectID)
	if httpapi.WriteRead(c, err) {
		return Control{}, false
	}
	if write && !access.CanWrite(membership.Role) {
		httpapi.Forbidden(c)
		return Control{}, false
	}
	return item, true
}
