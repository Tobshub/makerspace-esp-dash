package config

import (
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds process configuration loaded from the environment.
// Device secrets and broker admin passwords must never be logged.
type Config struct {
	HTTPAddr                 string
	DatabaseURL              string
	MQTTBrokerURL            string
	MQTTUsername             string
	MQTTPassword             string
	MQTTPublicHost           string
	MQTTPublicPort           int
	DeviceOfflineTimeout     time.Duration
	CommandTimeout           time.Duration
	AppURL                   string
	APIURL                   string
	TelemetryRetentionDays   int
	DeviceEventRetentionDays int
	MQTTMaxPayloadBytes      int
	MQTTMaxMetrics           int
	MQTTMaxKeyLength         int
	MQTTMaxStringLength      int
	MQTTTelemetryPerSecond   int
}

func Load() Config {
	loadDotEnv()
	return Config{
		HTTPAddr:                 env("HTTP_ADDR", ":8080"),
		DatabaseURL:              env("DATABASE_URL", "postgres://makerspace:makerspace@localhost:5432/makerspace?sslmode=disable"),
		MQTTBrokerURL:            env("MQTT_BROKER_URL", "tcp://localhost:1883"),
		MQTTUsername:             os.Getenv("MQTT_USERNAME"),
		MQTTPassword:             os.Getenv("MQTT_PASSWORD"),
		MQTTPublicHost:           os.Getenv("MQTT_PUBLIC_HOST"),
		MQTTPublicPort:           envInt("MQTT_PUBLIC_PORT", 0),
		DeviceOfflineTimeout:     time.Duration(envInt("DEVICE_OFFLINE_TIMEOUT_SECONDS", 60)) * time.Second,
		CommandTimeout:           time.Duration(envInt("COMMAND_TIMEOUT_SECONDS", 30)) * time.Second,
		AppURL:                   env("APP_URL", "http://localhost:5173"),
		APIURL:                   env("API_URL", "http://localhost:8080"),
		TelemetryRetentionDays:   envInt("TELEMETRY_RETENTION_DAYS", 90),
		DeviceEventRetentionDays: envInt("DEVICE_EVENT_RETENTION_DAYS", 30),
		MQTTMaxPayloadBytes:      positive(envInt("MQTT_MAX_PAYLOAD_BYTES", 16*1024), 16*1024),
		MQTTMaxMetrics:           positive(envInt("MQTT_MAX_METRICS", 50), 50),
		MQTTMaxKeyLength:         positive(envInt("MQTT_MAX_KEY_LENGTH", 64), 64),
		MQTTMaxStringLength:      positive(envInt("MQTT_MAX_STRING_LENGTH", 1024), 1024),
		MQTTTelemetryPerSecond:   positive(envInt("MQTT_TELEMETRY_PER_SECOND", 10), 10),
	}
}

func loadDotEnv() {
	dir, err := os.Getwd()
	if err != nil {
		return
	}
	for {
		path := filepath.Join(dir, ".env")
		if _, err := os.Stat(path); err == nil {
			_ = godotenv.Load(path)
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return
		}
		dir = parent
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func positive(n, fallback int) int {
	if n <= 0 {
		return fallback
	}
	return n
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
