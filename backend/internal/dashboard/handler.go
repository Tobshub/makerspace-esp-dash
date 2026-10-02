package dashboard

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/access"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/auth"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/httpapi"
)

func Register(r *gin.RouterGroup, pool *pgxpool.Pool) {
	r.GET("/projects/:projectId/overview", func(c *gin.Context) { handleOverview(c, pool) })
	r.GET("/projects/:projectId/events", func(c *gin.Context) { handleProjectEvents(c, pool) })
	r.GET("/devices/:deviceId/events", func(c *gin.Context) { handleDeviceEvents(c, pool) })
	r.GET("/devices/:deviceId/state", func(c *gin.Context) { handleState(c, pool) })
}

func handleOverview(c *gin.Context, pool *pgxpool.Pool) {
	projectID := c.Param("projectId")
	if !member(c, pool, projectID) {
		return
	}
	body, err := overview(c.Request.Context(), pool, projectID)
	if httpapi.WriteRead(c, err) {
		return
	}
	c.JSON(http.StatusOK, body)
}

func handleProjectEvents(c *gin.Context, pool *pgxpool.Pool) {
	projectID := c.Param("projectId")
	if !member(c, pool, projectID) {
		return
	}
	eventType, limit, ok := eventQuery(c)
	if !ok {
		return
	}
	items, err := projectEvents(c.Request.Context(), pool, projectID, eventType, limit)
	if httpapi.WriteRead(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"events": items})
}

func handleDeviceEvents(c *gin.Context, pool *pgxpool.Pool) {
	deviceID, ok := authorizeDevice(c, pool)
	if !ok {
		return
	}
	eventType, limit, ok := eventQuery(c)
	if !ok {
		return
	}
	items, err := deviceEvents(c.Request.Context(), pool, deviceID, eventType, limit)
	if httpapi.WriteRead(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"events": items})
}

func handleState(c *gin.Context, pool *pgxpool.Pool) {
	deviceID, ok := authorizeDevice(c, pool)
	if !ok {
		return
	}
	body, err := deviceState(c.Request.Context(), pool, deviceID)
	if httpapi.WriteRead(c, err) {
		return
	}
	c.JSON(http.StatusOK, body)
}

func member(c *gin.Context, pool *pgxpool.Pool, projectID string) bool {
	if !httpapi.ValidID(projectID) {
		httpapi.Error(c, http.StatusNotFound, "NOT_FOUND", "Not found")
		return false
	}
	if _, err := access.Project(c.Request.Context(), pool, auth.Current(c).ID, projectID); httpapi.WriteRead(c, err) {
		return false
	}
	return true
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
	if !member(c, pool, projectID) {
		return "", false
	}
	return deviceID, true
}

func eventQuery(c *gin.Context) (string, int, bool) {
	eventType := c.Query("event_type")
	if len(eventType) > 64 {
		httpapi.Validation(c, map[string]string{"event_type": "Must be 64 characters or fewer"})
		return "", 0, false
	}
	limit := 20
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 100 {
			httpapi.Validation(c, map[string]string{"limit": "Must be from 1 to 100"})
			return "", 0, false
		}
		limit = n
	}
	return eventType, limit, true
}
