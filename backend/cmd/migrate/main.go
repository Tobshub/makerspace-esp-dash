package main

import (
	"log/slog"
	"os"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/config"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/database"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	cfg := config.Load()
	if err := database.Migrate(cfg.DatabaseURL); err != nil {
		slog.Error("migrate failed", "err", err)
		os.Exit(1)
	}
	slog.Info("migrations applied")
}
