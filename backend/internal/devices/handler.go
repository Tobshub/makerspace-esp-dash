package devices

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/access"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/auth"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/httpapi"
)

func Register(r *gin.RouterGroup, pool *pgxpool.Pool, host string, port int) {
	r.GET("/projects/:projectId/devices", func(c *gin.Context) { handleList(c, pool) })
	r.POST("/projects/:projectId/devices", func(c *gin.Context) { handleCreate(c, pool, host, port) })
	r.GET("/devices/:deviceId", func(c *gin.Context) { handleGet(c, pool) })
	r.PATCH("/devices/:deviceId", func(c *gin.Context) { handleUpdate(c, pool) })
	r.DELETE("/devices/:deviceId", func(c *gin.Context) { handleDelete(c, pool) })
	r.POST("/devices/:deviceId/rotate-secret", func(c *gin.Context) { handleRotate(c, pool, host, port) })
	r.POST("/devices/:deviceId/disable", func(c *gin.Context) { handleDisable(c, pool) })
}

type deviceBody struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Metadata    json.RawMessage `json:"metadata"`
}

func handleList(c *gin.Context, pool *pgxpool.Pool) {
	if !httpapi.ValidID(c.Param("projectId")) {
		httpapi.Error(c, http.StatusNotFound, "NOT_FOUND", "Not found")
		return
	}
	if _, err := access.Project(c.Request.Context(), pool, auth.Current(c).ID, c.Param("projectId")); httpapi.WriteRead(c, err) {
		return
	}
	items, err := listDevices(c.Request.Context(), pool, c.Param("projectId"))
	if err != nil {
		httpapi.Internal(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"devices": items})
}

func handleCreate(c *gin.Context, pool *pgxpool.Pool, host string, port int) {
	if !httpapi.ValidID(c.Param("projectId")) {
		httpapi.Error(c, http.StatusNotFound, "NOT_FOUND", "Not found")
		return
	}
	membership, err := access.Project(c.Request.Context(), pool, auth.Current(c).ID, c.Param("projectId"))
	if httpapi.WriteRead(c, err) {
		return
	}
	if !access.CanWrite(membership.Role) {
		httpapi.Forbidden(c)
		return
	}
	name, description, metadata, ok := readDeviceBody(c, true)
	if !ok {
		return
	}
	issued, err := createDevice(c.Request.Context(), pool, c.Param("projectId"), name, description, metadata, host, port)
	if err != nil {
		httpapi.Internal(c, err)
		return
	}
	c.JSON(http.StatusCreated, issued)
}

func handleGet(c *gin.Context, pool *pgxpool.Pool) {
	device, ok := loadAuthorized(c, pool, false)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"device": device})
}

func handleUpdate(c *gin.Context, pool *pgxpool.Pool) {
	if _, ok := loadAuthorized(c, pool, true); !ok {
		return
	}
	name, description, metadata, ok := readDeviceBody(c, true)
	if !ok {
		return
	}
	device, err := updateDevice(c.Request.Context(), pool, c.Param("deviceId"), name, description, metadata)
	if httpapi.WriteRead(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"device": device})
}

func handleDelete(c *gin.Context, pool *pgxpool.Pool) {
	if _, ok := loadAuthorized(c, pool, true); !ok {
		return
	}
	if err := deleteDevice(c.Request.Context(), pool, c.Param("deviceId")); httpapi.WriteRead(c, err) {
		return
	}
	c.Status(http.StatusNoContent)
}

func handleRotate(c *gin.Context, pool *pgxpool.Pool, host string, port int) {
	if _, ok := loadAuthorized(c, pool, true); !ok {
		return
	}
	issued, err := rotateSecret(c.Request.Context(), pool, c.Param("deviceId"), host, port)
	if httpapi.WriteRead(c, err) {
		return
	}
	c.JSON(http.StatusOK, issued)
}

func handleDisable(c *gin.Context, pool *pgxpool.Pool) {
	if _, ok := loadAuthorized(c, pool, true); !ok {
		return
	}
	device, err := disableDevice(c.Request.Context(), pool, c.Param("deviceId"))
	if httpapi.WriteRead(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"device": device})
}

func loadAuthorized(c *gin.Context, pool *pgxpool.Pool, write bool) (Device, bool) {
	if !httpapi.ValidID(c.Param("deviceId")) {
		httpapi.Error(c, http.StatusNotFound, "NOT_FOUND", "Not found")
		return Device{}, false
	}
	device, err := getDevice(c.Request.Context(), pool, c.Param("deviceId"))
	if httpapi.WriteRead(c, err) {
		return Device{}, false
	}
	membership, err := access.Project(c.Request.Context(), pool, auth.Current(c).ID, device.ProjectID)
	if httpapi.WriteRead(c, err) {
		return Device{}, false
	}
	if write && !access.CanWrite(membership.Role) {
		httpapi.Forbidden(c)
		return Device{}, false
	}
	return device, true
}

func readDeviceBody(c *gin.Context, requireName bool) (string, string, json.RawMessage, bool) {
	var body deviceBody
	if c.ShouldBindJSON(&body) != nil {
		httpapi.Validation(c, map[string]string{"body": "Request must be JSON"})
		return "", "", nil, false
	}
	name := strings.TrimSpace(body.Name)
	description := strings.TrimSpace(body.Description)
	fields := map[string]string{}
	if requireName && (name == "" || len(name) > 80) {
		fields["name"] = "Name is required"
	}
	if len(description) > 2000 {
		fields["description"] = "Description is too long"
	}
	metadata := body.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	} else if !json.Valid(metadata) || metadata[0] != '{' {
		fields["metadata"] = "Metadata must be a JSON object"
	} else if len(metadata) > 8192 {
		fields["metadata"] = "Metadata is too large"
	}
	if len(fields) > 0 {
		httpapi.Validation(c, fields)
		return "", "", nil, false
	}
	return name, description, metadata, true
}
