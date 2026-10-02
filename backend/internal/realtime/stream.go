package realtime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/access"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/auth"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/httpapi"
)

func Register(r *gin.RouterGroup, pool *pgxpool.Pool, hub *Hub) {
	r.GET("/projects/:projectId/stream", func(c *gin.Context) {
		stream(c, pool, hub)
	})
}

func stream(c *gin.Context, pool *pgxpool.Pool, hub *Hub) {
	projectID := c.Param("projectId")
	if !httpapi.ValidID(projectID) {
		httpapi.Error(c, http.StatusNotFound, "NOT_FOUND", "Not found")
		return
	}
	if _, err := access.Project(c.Request.Context(), pool, auth.Current(c).ID, projectID); httpapi.WriteRead(c, err) {
		return
	}
	flusher, ok := c.Writer.(http.Flusher)
	if !ok || hub == nil {
		httpapi.Error(c, http.StatusInternalServerError, "INTERNAL", "Something went wrong")
		return
	}

	events, cancel := hub.Subscribe(projectID)
	defer cancel()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	if _, err := fmt.Fprintf(c.Writer, "retry: 3000\n\n: connected\n\n"); err != nil {
		return
	}
	flusher.Flush()

	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ping.C:
			if _, err := fmt.Fprintf(c.Writer, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case event, open := <-events:
			if !open {
				return
			}
			payload, err := json.Marshal(event)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", payload); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
