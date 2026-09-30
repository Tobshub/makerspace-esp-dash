package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds process configuration loaded from the environment.
// Device secrets and broker admin passwords must never be logged.
type Config struct {
	HTTPAddr                 string
	DatabaseURL              string
	MQTTBrokerURL            string
	MQTTUsername             string
	MQTTPassword             string
	DeviceOfflineTimeout     time.Duration
	AppURL                   string
	APIURL                   string
	TelemetryRetentionDays   int
	DeviceEventRetentionDays int
}

func Load() Config {
	return Config{
		HTTPAddr:                 env("HTTP_ADDR", ":8080"),
		DatabaseURL:              os.Getenv("DATABASE_URL"),
		MQTTBrokerURL:            env("MQTT_BROKER_URL", "tcp://localhost:1883"),
		MQTTUsername:             os.Getenv("MQTT_USERNAME"),
		MQTTPassword:             os.Getenv("MQTT_PASSWORD"),
		DeviceOfflineTimeout:     time.Duration(envInt("DEVICE_OFFLINE_TIMEOUT_SECONDS", 60)) * time.Second,
		AppURL:                   env("APP_URL", "http://localhost:5173"),
		APIURL:                   env("API_URL", "http://localhost:8080"),
		TelemetryRetentionDays:   envInt("TELEMETRY_RETENTION_DAYS", 90),
		DeviceEventRetentionDays: envInt("DEVICE_EVENT_RETENTION_DAYS", 30),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
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
