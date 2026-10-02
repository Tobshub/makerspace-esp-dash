package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/config"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/database"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/ingest"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	cfg := config.Load()
	pool, err := database.Open(context.Background(), cfg.DatabaseURL)
	if err != nil || pool == nil {
		slog.Error("database configuration invalid", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	slog.Info("worker sweeping offline devices", "offline_timeout_seconds", int(cfg.DeviceOfflineTimeout/time.Second))
	ingest.FromConfig(pool, cfg).RunSweep(ctx)
}
