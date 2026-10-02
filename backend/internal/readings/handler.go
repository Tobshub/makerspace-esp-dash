package readings

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/access"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/auth"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/httpapi"
)

func Register(r *gin.RouterGroup, pool *pgxpool.Pool) {
	r.GET("/devices/:deviceId/telemetry/latest", func(c *gin.Context) { handleDeviceLatest(c, pool) })
	r.GET("/devices/:deviceId/telemetry", func(c *gin.Context) { handleHistory(c, pool) })
	r.GET("/projects/:projectId/telemetry/latest", func(c *gin.Context) { handleProjectLatest(c, pool) })
}

func handleDeviceLatest(c *gin.Context, pool *pgxpool.Pool) {
	deviceID, ok := authorizeDevice(c, pool)
	if !ok {
		return
	}
	latest, err := latestDevice(c.Request.Context(), pool, deviceID)
	if httpapi.WriteRead(c, err) {
		return
	}
	c.JSON(http.StatusOK, latest)
}

func handleHistory(c *gin.Context, pool *pgxpool.Pool) {
	deviceID, ok := authorizeDevice(c, pool)
	if !ok {
		return
	}
	query, fields := ParseHistory(time.Now(), c.Request.URL.Query())
	if len(fields) > 0 {
		httpapi.Validation(c, fields)
		return
	}
	series, err := history(c.Request.Context(), pool, deviceID, query)
	if httpapi.WriteRead(c, err) {
		return
	}
	c.JSON(http.StatusOK, series)
}

func handleProjectLatest(c *gin.Context, pool *pgxpool.Pool) {
	projectID := c.Param("projectId")
	if !httpapi.ValidID(projectID) {
		httpapi.Error(c, http.StatusNotFound, "NOT_FOUND", "Not found")
		return
	}
	if _, err := access.Project(c.Request.Context(), pool, auth.Current(c).ID, projectID); httpapi.WriteRead(c, err) {
		return
	}
	latest, err := latestProject(c.Request.Context(), pool, projectID)
	if httpapi.WriteRead(c, err) {
		return
	}
	c.JSON(http.StatusOK, latest)
}

func authorizeDevice(c *gin.Context, pool *pgxpool.Pool) (string, bool) {
	deviceID := c.Param("deviceId")
	if !httpapi.ValidID(deviceID) {
		httpapi.Error(c, http.StatusNotFound, "NOT_FOUND", "Not found")
		return "", false
	}
	projectID, err := deviceProject(c.Request.Context(), pool, deviceID)
	if httpapi.WriteRead(c, err) {
		return "", false
	}
	if _, err := access.Project(c.Request.Context(), pool, auth.Current(c).ID, projectID); httpapi.WriteRead(c, err) {
		return "", false
	}
	return deviceID, true
}
