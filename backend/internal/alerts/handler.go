package alerts

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/access"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/auth"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/httpapi"
)

func Register(r *gin.RouterGroup, pool *pgxpool.Pool) {
	r.GET("/projects/:projectId/alerts", func(c *gin.Context) { handleList(c, pool) })
	r.POST("/projects/:projectId/alerts", func(c *gin.Context) { handleCreate(c, pool) })
	r.GET("/projects/:projectId/alert-events", func(c *gin.Context) { handleEvents(c, pool) })
	r.PATCH("/alerts/:alertId", func(c *gin.Context) { handleUpdate(c, pool) })
	r.DELETE("/alerts/:alertId", func(c *gin.Context) { handleDelete(c, pool) })
}

type body struct {
	Name            string          `json:"name"`
	RuleType        string          `json:"ruleType"`
	DeviceID        string          `json:"deviceId"`
	MetricKey       string          `json:"metricKey"`
	Operator        string          `json:"operator"`
	ThresholdValue  *float64        `json:"thresholdValue"`
	DurationSeconds *int            `json:"durationSeconds"`
	Enabled         *bool           `json:"enabled"`
	Configuration   json.RawMessage `json:"configuration"`
}

func handleList(c *gin.Context, pool *pgxpool.Pool) {
	if _, ok := authorizeProject(c, pool, false); !ok {
		return
	}
	items, err := listRules(c.Request.Context(), pool, c.Param("projectId"))
	if httpapi.WriteRead(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"alerts": items})
}

func handleCreate(c *gin.Context, pool *pgxpool.Pool) {
	if _, ok := authorizeProject(c, pool, true); !ok {
		return
	}
	in, ok := readBody(c)
	if !ok {
		return
	}
	item, err := createRule(c.Request.Context(), pool, c.Param("projectId"), in)
	if errors.Is(err, errDevice) {
		httpapi.Validation(c, map[string]string{"deviceId": "Choose a device in this project"})
		return
	}
	if err != nil {
		httpapi.Internal(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"alert": item})
}

func handleUpdate(c *gin.Context, pool *pgxpool.Pool) {
	current, ok := loadAuthorized(c, pool, true)
	if !ok {
		return
	}
	in, ok := readBody(c)
	if !ok {
		return
	}
	item, err := updateRule(c.Request.Context(), pool, current.ID, in)
	if errors.Is(err, errDevice) {
		httpapi.Validation(c, map[string]string{"deviceId": "Choose a device in this project"})
		return
	}
	if httpapi.WriteRead(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"alert": item})
}

func handleDelete(c *gin.Context, pool *pgxpool.Pool) {
	current, ok := loadAuthorized(c, pool, true)
	if !ok {
		return
	}
	if err := deleteRule(c.Request.Context(), pool, current.ID); httpapi.WriteRead(c, err) {
		return
	}
	c.Status(http.StatusNoContent)
}

func handleEvents(c *gin.Context, pool *pgxpool.Pool) {
	if _, ok := authorizeProject(c, pool, false); !ok {
		return
	}
	limit := 50
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 100 {
			httpapi.Validation(c, map[string]string{"limit": "Must be from 1 to 100"})
			return
		}
		limit = n
	}
	items, err := listEvents(c.Request.Context(), pool, c.Param("projectId"), limit)
	if httpapi.WriteRead(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"events": items})
}

func readBody(c *gin.Context) (Input, bool) {
	var raw body
	if err := c.ShouldBindJSON(&raw); err != nil {
		httpapi.Validation(c, map[string]string{"body": "Request must be JSON"})
		return Input{}, false
	}
	duration := 0
	if raw.DurationSeconds != nil {
		duration = *raw.DurationSeconds
	}
	enabled := true
	if raw.Enabled != nil {
		enabled = *raw.Enabled
	}
	in, fields := validate(Input{
		Name: raw.Name, RuleType: raw.RuleType, DeviceID: raw.DeviceID, MetricKey: raw.MetricKey,
		Operator: raw.Operator, ThresholdValue: raw.ThresholdValue, DurationSeconds: duration,
		Enabled: enabled, Configuration: raw.Configuration,
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

func loadAuthorized(c *gin.Context, pool *pgxpool.Pool, write bool) (Rule, bool) {
	if !httpapi.ValidID(c.Param("alertId")) {
		httpapi.Error(c, http.StatusNotFound, "NOT_FOUND", "Not found")
		return Rule{}, false
	}
	item, err := getRule(c.Request.Context(), pool, c.Param("alertId"))
	if httpapi.WriteRead(c, err) {
		return Rule{}, false
	}
	membership, err := access.Project(c.Request.Context(), pool, auth.Current(c).ID, item.ProjectID)
	if httpapi.WriteRead(c, err) {
		return Rule{}, false
	}
	if write && !access.CanWrite(membership.Role) {
		httpapi.Forbidden(c)
		return Rule{}, false
	}
	return item, true
}
