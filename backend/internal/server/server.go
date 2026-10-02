package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/config"
)

// Deps are the process dependencies the router probes and serves.
type Deps struct {
	Config       config.Config
	Pool         *pgxpool.Pool
	DatabasePing func(context.Context) error
	MQTTUp       func() bool
	BrokerHost   string
	BrokerPort   int
}

func New(d Deps) *gin.Engine {
	router := gin.New()
	router.Use(requestLog(), cors(d.Config.AppURL), gin.Recovery())

	router.GET("/health", func(c *gin.Context) { writeStatus(c, d, false) })
	router.GET("/ready", func(c *gin.Context) { writeStatus(c, d, true) })

	router.GET("/api/v1", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"service": "makerspace-esp-dash", "version": "v1"})
	})

	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{"code": "NOT_FOUND", "message": "Not found"},
		})
	})
	return router
}

func writeStatus(c *gin.Context, d Deps, strict bool) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	database := "ok"
	if err := ping(ctx, d); err != nil {
		database = "down"
	}
	mqttStatus := "down"
	if d.MQTTUp != nil && d.MQTTUp() {
		mqttStatus = "ok"
	}
	status := "ok"
	code := http.StatusOK
	if database != "ok" || mqttStatus != "ok" {
		status = "degraded"
		if strict {
			code = http.StatusServiceUnavailable
		}
	}
	c.JSON(code, gin.H{
		"status":   status,
		"database": database,
		"mqtt":     mqttStatus,
	})
}

func ping(ctx context.Context, d Deps) error {
	if d.DatabasePing != nil {
		return d.DatabasePing(ctx)
	}
	if d.Pool == nil {
		return errNoDatabase
	}
	return d.Pool.Ping(ctx)
}

type simpleError string

func (e simpleError) Error() string { return string(e) }

const errNoDatabase simpleError = "database not configured"

func requestLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = randomID()
		}
		c.Set("request_id", id)
		c.Header("X-Request-ID", id)
		start := time.Now()
		c.Next()
		slog.Info("request",
			"request_id", id,
			"user_id", c.GetString("user_id"),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}
}

func cors(appURL string) gin.HandlerFunc {
	allowed := map[string]bool{
		appURL:                  true,
		"http://localhost:5173": true,
		"http://127.0.0.1:5173": true,
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if allowed[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			c.Header("Vary", "Origin")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func randomID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "request"
	}
	return hex.EncodeToString(buf)
}
