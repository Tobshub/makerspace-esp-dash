package commands

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/access"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/auth"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/httpapi"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/realtime"
)

func Register(r *gin.RouterGroup, pool *pgxpool.Pool, pub Publisher, notify func(string, realtime.Event)) {
	r.POST("/devices/:deviceId/commands", func(c *gin.Context) { handleCreate(c, pool, pub, notify) })
	r.GET("/devices/:deviceId/commands", func(c *gin.Context) { handleList(c, pool) })
	r.GET("/commands/:commandId", func(c *gin.Context) { handleGet(c, pool) })
}

type createBody struct {
	Command string          `json:"command"`
	Payload json.RawMessage `json:"payload"`
}

func handleCreate(c *gin.Context, pool *pgxpool.Pool, pub Publisher, notify func(string, realtime.Event)) {
	deviceID := c.Param("deviceId")
	projectID, ok := authorizeDevice(c, pool, deviceID, true)
	if !ok {
		return
	}
	var body createBody
	if err := c.ShouldBindJSON(&body); err != nil {
		httpapi.Validation(c, map[string]string{"body": "Request must be JSON"})
		return
	}
	in, fields := validateInput(body.Command, body.Payload)
	if len(fields) > 0 {
		httpapi.Validation(c, fields)
		return
	}
	cmd, target, err := create(c.Request.Context(), pool, deviceID, auth.Current(c).ID, in)
	if errors.Is(err, ErrNotFound) || (err == nil && target.ProjectID != projectID) {
		httpapi.Error(c, http.StatusNotFound, "NOT_FOUND", "Not found")
		return
	}
	if errors.Is(err, ErrDisabled) {
		httpapi.Error(c, http.StatusConflict, "CONFLICT", "Device is disabled")
		return
	}
	if err != nil {
		httpapi.Internal(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"command": cmd})
	Dispatch(pool, pub, cmd, target.DeviceKey, notify)
}

func handleList(c *gin.Context, pool *pgxpool.Pool) {
	deviceID := c.Param("deviceId")
	if _, ok := authorizeDevice(c, pool, deviceID, false); !ok {
		return
	}
	items, err := listByDevice(c.Request.Context(), pool, deviceID)
	if httpapi.WriteRead(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"commands": items})
}

func handleGet(c *gin.Context, pool *pgxpool.Pool) {
	if !httpapi.ValidID(c.Param("commandId")) {
		httpapi.Error(c, http.StatusNotFound, "NOT_FOUND", "Not found")
		return
	}
	cmd, err := get(c.Request.Context(), pool, c.Param("commandId"))
	if httpapi.WriteRead(c, err) {
		return
	}
	if _, err := access.Project(c.Request.Context(), pool, auth.Current(c).ID, cmd.ProjectID); httpapi.WriteRead(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"command": cmd})
}

func authorizeDevice(c *gin.Context, pool *pgxpool.Pool, deviceID string, write bool) (string, bool) {
	if !httpapi.ValidID(deviceID) {
		httpapi.Error(c, http.StatusNotFound, "NOT_FOUND", "Not found")
		return "", false
	}
	target, err := lookupDevice(c.Request.Context(), pool, deviceID)
	if httpapi.WriteRead(c, err) {
		return "", false
	}
	membership, err := access.Project(c.Request.Context(), pool, auth.Current(c).ID, target.ProjectID)
	if httpapi.WriteRead(c, err) {
		return "", false
	}
	if write && !access.CanWrite(membership.Role) {
		httpapi.Forbidden(c)
		return "", false
	}
	return target.ProjectID, true
}
