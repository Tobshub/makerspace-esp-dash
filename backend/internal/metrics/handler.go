package metrics

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/access"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/auth"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/httpapi"
)

func Register(r *gin.RouterGroup, pool *pgxpool.Pool) {
	r.GET("/projects/:projectId/metrics", func(c *gin.Context) { handleList(c, pool) })
	r.POST("/projects/:projectId/metrics", func(c *gin.Context) { handleCreate(c, pool) })
	r.GET("/projects/:projectId/metrics/discovered", func(c *gin.Context) { handleDiscovered(c, pool) })
	r.GET("/metrics/:metricId", func(c *gin.Context) { handleGet(c, pool) })
	r.PATCH("/metrics/:metricId", func(c *gin.Context) { handleUpdate(c, pool) })
	r.DELETE("/metrics/:metricId", func(c *gin.Context) { handleDelete(c, pool) })
}

type definitionBody struct {
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
}

func handleList(c *gin.Context, pool *pgxpool.Pool) {
	if _, ok := authorizeProject(c, pool, false); !ok {
		return
	}
	items, err := listDefinitions(c.Request.Context(), pool, c.Param("projectId"))
	if err != nil {
		httpapi.Internal(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"metrics": items})
}

func handleDiscovered(c *gin.Context, pool *pgxpool.Pool) {
	if _, ok := authorizeProject(c, pool, false); !ok {
		return
	}
	items, err := listDiscovered(c.Request.Context(), pool, c.Param("projectId"))
	if err != nil {
		httpapi.Internal(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"metrics": items})
}

func handleCreate(c *gin.Context, pool *pgxpool.Pool) {
	if _, ok := authorizeProject(c, pool, true); !ok {
		return
	}
	in, ok := readBody(c, true)
	if !ok {
		return
	}
	item, err := createDefinition(c.Request.Context(), pool, c.Param("projectId"), in)
	if errors.Is(err, ErrConflict) {
		httpapi.Error(c, http.StatusConflict, "CONFLICT", "A definition for this key already exists")
		return
	}
	if err != nil {
		httpapi.Internal(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"metric": item})
}

func handleGet(c *gin.Context, pool *pgxpool.Pool) {
	item, ok := loadAuthorized(c, pool, false)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"metric": item})
}

func handleUpdate(c *gin.Context, pool *pgxpool.Pool) {
	if _, ok := loadAuthorized(c, pool, true); !ok {
		return
	}
	in, ok := readBody(c, false)
	if !ok {
		return
	}
	item, err := updateDefinition(c.Request.Context(), pool, c.Param("metricId"), in)
	if httpapi.WriteRead(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"metric": item})
}

func handleDelete(c *gin.Context, pool *pgxpool.Pool) {
	if _, ok := loadAuthorized(c, pool, true); !ok {
		return
	}
	if err := deleteDefinition(c.Request.Context(), pool, c.Param("metricId")); httpapi.WriteRead(c, err) {
		return
	}
	c.Status(http.StatusNoContent)
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

func loadAuthorized(c *gin.Context, pool *pgxpool.Pool, write bool) (Definition, bool) {
	if !httpapi.ValidID(c.Param("metricId")) {
		httpapi.Error(c, http.StatusNotFound, "NOT_FOUND", "Not found")
		return Definition{}, false
	}
	item, err := getDefinition(c.Request.Context(), pool, c.Param("metricId"))
	if httpapi.WriteRead(c, err) {
		return Definition{}, false
	}
	membership, err := access.Project(c.Request.Context(), pool, auth.Current(c).ID, item.ProjectID)
	if httpapi.WriteRead(c, err) {
		return Definition{}, false
	}
	if write && !access.CanWrite(membership.Role) {
		httpapi.Forbidden(c)
		return Definition{}, false
	}
	return item, true
}

func readBody(c *gin.Context, withKey bool) (Input, bool) {
	var body definitionBody
	if c.ShouldBindJSON(&body) != nil {
		httpapi.Validation(c, map[string]string{"body": "Request must be JSON"})
		return Input{}, false
	}
	in := Input{
		Key:         strings.TrimSpace(body.Key),
		Name:        body.Name,
		Description: body.Description,
		DataType:    body.DataType,
		Unit:        body.Unit,
		DisplayType: body.DisplayType,
		MinValue:    body.MinValue,
		MaxValue:    body.MaxValue,
		Hidden:      body.Hidden,
		Metadata:    body.Metadata,
		checkKey:    withKey,
	}
	if fields := validate(in); len(fields) > 0 {
		httpapi.Validation(c, fields)
		return Input{}, false
	}
	return in, true
}
