package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/config"
)

func main() {
	cfg := config.Load()

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	router.GET("/health", func(c *gin.Context) {
		// Phase 1 replaces unchecked with live database and MQTT probes.
		c.JSON(http.StatusOK, gin.H{
			"status":   "ok",
			"database": "unchecked",
			"mqtt":     "unchecked",
		})
	})
	router.GET("/ready", func(c *gin.Context) {
		// Connectivity checks arrive in Phase 1. Until then the process is
		// ready to serve the skeleton API only.
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/api/v1", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service": "makerspace-esp-dash",
			"version": "v1",
		})
	})

	log.Printf("api listening on %s", cfg.HTTPAddr)
	if err := router.Run(cfg.HTTPAddr); err != nil {
		log.Fatal(err)
	}
}
