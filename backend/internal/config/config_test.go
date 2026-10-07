package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("MQTT_BROKER_URL", "")
	t.Setenv("MQTT_PUBLIC_HOST", "")
	t.Setenv("MQTT_PUBLIC_PORT", "")
	t.Setenv("DEVICE_OFFLINE_TIMEOUT_SECONDS", "")
	t.Setenv("MQTT_MAX_PAYLOAD_BYTES", "")
	t.Setenv("MQTT_MAX_METRICS", "")
	t.Setenv("MQTT_MAX_KEY_LENGTH", "")
	t.Setenv("MQTT_MAX_STRING_LENGTH", "")
	t.Setenv("MQTT_TELEMETRY_PER_SECOND", "")

	cfg := Load()
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q", cfg.HTTPAddr)
	}
	if cfg.MQTTBrokerURL != "tcp://localhost:1883" {
		t.Fatalf("MQTTBrokerURL = %q", cfg.MQTTBrokerURL)
	}
	if cfg.MQTTPublicHost != "" || cfg.MQTTPublicPort != 0 {
		t.Fatalf("public mqtt = %q:%d", cfg.MQTTPublicHost, cfg.MQTTPublicPort)
	}
	if cfg.DeviceOfflineTimeout.Seconds() != 60 {
		t.Fatalf("timeout = %s", cfg.DeviceOfflineTimeout)
	}
	if cfg.MQTTMaxPayloadBytes != 16*1024 || cfg.MQTTMaxMetrics != 50 || cfg.MQTTMaxKeyLength != 64 {
		t.Fatalf("payload limits %+v", cfg)
	}
	if cfg.MQTTMaxStringLength != 1024 || cfg.MQTTTelemetryPerSecond != 10 {
		t.Fatalf("rate limits %+v", cfg)
	}
}
