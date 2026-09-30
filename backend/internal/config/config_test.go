package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("MQTT_BROKER_URL", "")
	t.Setenv("DEVICE_OFFLINE_TIMEOUT_SECONDS", "")

	cfg := Load()
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q", cfg.HTTPAddr)
	}
	if cfg.MQTTBrokerURL != "tcp://localhost:1883" {
		t.Fatalf("MQTTBrokerURL = %q", cfg.MQTTBrokerURL)
	}
	if cfg.DeviceOfflineTimeout.Seconds() != 60 {
		t.Fatalf("timeout = %s", cfg.DeviceOfflineTimeout)
	}
}
