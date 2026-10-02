package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/config"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/database"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/ingest"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/mqtt"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/realtime"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/server"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	cfg := config.Load()

	pool, err := database.Open(context.Background(), cfg.DatabaseURL)
	if err != nil {
		slog.Error("database configuration invalid", "err", err)
		os.Exit(1)
	}
	if pool != nil {
		defer pool.Close()
	}

	host, port, err := mqtt.ParsePublic(cfg.MQTTBrokerURL)
	if err != nil {
		slog.Error("mqtt broker url invalid")
		host, port = "localhost", 1883
	}
	hub := realtime.New()
	broker := mqtt.Connect(cfg.MQTTBrokerURL, cfg.MQTTUsername, cfg.MQTTPassword)
	defer broker.Close()
	if pool != nil {
		incoming := ingest.FromConfig(pool, cfg)
		incoming.PublishTo(hub)
		broker.OnMessage(incoming.OnMQTT)
		go incoming.RunSweep(context.Background())
		slog.Info("mqtt ingest",
			"broker_auth", cfg.MQTTUsername != "",
			"offline_timeout_seconds", int(cfg.DeviceOfflineTimeout/time.Second),
			"telemetry_per_second", cfg.MQTTTelemetryPerSecond,
		)
	}

	engine := server.New(server.Deps{
		Config:     cfg,
		Pool:       pool,
		MQTTUp:     broker.Connected,
		BrokerHost: host,
		BrokerPort: port,
		Hub:        hub,
	})
	slog.Info("api listening", "addr", cfg.HTTPAddr)
	if err := engine.Run(cfg.HTTPAddr); err != nil {
		slog.Error("api stopped", "err", err)
		os.Exit(1)
	}
}
