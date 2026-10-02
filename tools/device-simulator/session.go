package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
)

const (
	defaultInterval     = 5 * time.Second
	minInterval         = 200 * time.Millisecond
	defaultFirmware     = "sim-1.0.0"
	defaultRetry        = 2 * time.Second
	maxReconnectBackoff = 30 * time.Second
)

var (
	onlinePayload  = []byte(`{"status":"online"}`)
	offlinePayload = []byte(`{"status":"offline"}`)
)

// Config is the simulator process configuration. Secret is the MQTT password and is never logged.
type Config struct {
	DeviceKey    string
	Secret       string
	ProjectID    string
	Broker       string
	Interval     time.Duration
	Firmware     string
	connectRetry time.Duration
}

// Run connects as the device, publishes greenhouse telemetry, and acknowledges commands until ctx is cancelled.
// A down broker is retried. After a drop, the session subscribes and announces itself again.
// A clean shutdown publishes retained offline status. An unclean drop is covered by the Last Will.
func Run(ctx context.Context, cfg Config) error {
	if err := cfg.validate(); err != nil {
		return err
	}
	cfg.ProjectID = strings.ToLower(cfg.ProjectID)
	cfg.DeviceKey = strings.ToLower(cfg.DeviceKey)
	topics := topicsFor(cfg.ProjectID, cfg.DeviceKey)
	bed := NewGreenhouse(time.Now())
	retry := cfg.connectRetry
	if retry <= 0 {
		retry = defaultRetry
	}

	opts := pahomqtt.NewClientOptions().
		AddBroker(cfg.Broker).
		SetClientID("sim-"+cfg.DeviceKey).
		SetUsername(cfg.DeviceKey).
		SetPassword(cfg.Secret).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(retry).
		SetMaxReconnectInterval(maxReconnectBackoff).
		SetCleanSession(true).
		SetOrderMatters(false).
		SetKeepAlive(30*time.Second).
		SetPingTimeout(10*time.Second).
		SetConnectTimeout(10*time.Second).
		SetWriteTimeout(5*time.Second).
		SetWill(topics.status, string(offlinePayload), 1, true)
	opts.SetConnectionAttemptHandler(func(broker *url.URL, tlsCfg *tls.Config) *tls.Config {
		slog.Info("mqtt connecting", "broker", broker.Redacted(), "device_key", cfg.DeviceKey)
		return tlsCfg
	})
	opts.SetOnConnectHandler(func(c pahomqtt.Client) {
		slog.Info("mqtt connected", "device_key", cfg.DeviceKey, "project_id", cfg.ProjectID)
		if err := subscribeCommands(c, topics); err != nil {
			slog.Error("mqtt subscribe failed", "err", err, "device_key", cfg.DeviceKey, "mqtt_topic", topics.commands)
			return
		}
		if err := announce(c, topics, bed, cfg.Firmware); err != nil {
			slog.Error("mqtt announce failed", "err", err, "device_key", cfg.DeviceKey)
		}
	})
	opts.SetConnectionLostHandler(func(_ pahomqtt.Client, err error) {
		slog.Error("mqtt connection lost", "err", err, "device_key", cfg.DeviceKey)
	})

	client := pahomqtt.NewClient(opts)
	client.AddRoute(topics.commands, func(c pahomqtt.Client, msg pahomqtt.Message) {
		onCommand(c, topics, bed, cfg, msg)
	})
	client.Connect()
	slog.Info("device simulator",
		"device_key", cfg.DeviceKey,
		"project_id", cfg.ProjectID,
		"broker", redactBroker(cfg.Broker),
		"interval", cfg.Interval.String(),
		"telemetry", topics.telemetry,
		"commands", topics.commands,
	)

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			if client.IsConnectionOpen() {
				if err := publish(client, topics.status, 1, true, offlinePayload); err != nil {
					slog.Error("mqtt offline status failed", "err", err, "device_key", cfg.DeviceKey)
				}
			}
			client.Disconnect(500)
			slog.Info("device simulator stopped", "device_key", cfg.DeviceKey)
			return nil
		case now := <-ticker.C:
			if !client.IsConnectionOpen() {
				continue
			}
			sample := bed.Advance(now)
			payload, err := telemetryPayload(now, sample)
			if err != nil {
				slog.Error("telemetry encode failed", "err", err, "device_key", cfg.DeviceKey)
				continue
			}
			if err := publish(client, topics.telemetry, 0, false, payload); err != nil {
				slog.Error("telemetry publish failed", "err", err, "device_key", cfg.DeviceKey, "mqtt_topic", topics.telemetry)
				continue
			}
			slog.Info("telemetry",
				"device_key", cfg.DeviceKey,
				"temperature", sample.Temperature,
				"humidity", sample.Humidity,
				"soil_moisture", sample.SoilMoisture,
				"pump_active", sample.PumpActive,
			)
		}
	}
}

func subscribeCommands(client pahomqtt.Client, topics deviceTopics) error {
	token := client.Subscribe(topics.commands, 1, nil)
	if !token.WaitTimeout(5 * time.Second) {
		return errors.New("subscribe timeout")
	}
	return token.Error()
}

func onCommand(client pahomqtt.Client, topics deviceTopics, bed *Greenhouse, cfg Config, msg pahomqtt.Message) {
	payload := append([]byte(nil), msg.Payload()...)
	result, ok := bed.HandleCommand(payload)
	if !ok {
		slog.Warn("command ignored", "device_key", cfg.DeviceKey, "mqtt_topic", topics.commands)
		return
	}
	if err := publish(client, topics.ack, 1, false, result.Ack); err != nil {
		slog.Error("ack publish failed", "err", err, "device_key", cfg.DeviceKey, "command_id", result.ID, "mqtt_topic", topics.ack)
		return
	}
	attrs := []any{"device_key", cfg.DeviceKey, "command_id", result.ID, "command", result.Command, "success", result.Success}
	if result.Error != "" {
		attrs = append(attrs, "reason", result.Error)
	}
	slog.Info("command ack", attrs...)
	if !result.Changed {
		return
	}
	if err := publishSample(client, topics, bed, cfg.Firmware, time.Now()); err != nil {
		slog.Error("state publish failed", "err", err, "device_key", cfg.DeviceKey, "command_id", result.ID)
	}
}

func announce(client pahomqtt.Client, topics deviceTopics, bed *Greenhouse, firmware string) error {
	if err := publish(client, topics.status, 1, true, onlinePayload); err != nil {
		return err
	}
	return publishSample(client, topics, bed, firmware, time.Now())
}

func publishSample(client pahomqtt.Client, topics deviceTopics, bed *Greenhouse, firmware string, now time.Time) error {
	sample := bed.Reading(now)
	state, err := statePayload(firmware, sample)
	if err != nil {
		return err
	}
	if err := publish(client, topics.state, 1, false, state); err != nil {
		return err
	}
	payload, err := telemetryPayload(now, sample)
	if err != nil {
		return err
	}
	return publish(client, topics.telemetry, 0, false, payload)
}

func publish(client pahomqtt.Client, topic string, qos byte, retained bool, payload []byte) error {
	token := client.Publish(topic, qos, retained, payload)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("publish timeout %s", topic)
	}
	return token.Error()
}

func (c Config) validate() error {
	if c.DeviceKey == "" || c.Secret == "" || c.ProjectID == "" {
		return errors.New("device-key, secret, and project-id are required")
	}
	if !deviceKeyPattern.MatchString(c.DeviceKey) {
		return errors.New("device-key must look like dev_ followed by 20 hex characters")
	}
	if !projectIDPattern.MatchString(c.ProjectID) {
		return errors.New("project-id must be a UUID")
	}
	if c.Interval < minInterval {
		return fmt.Errorf("interval must be at least %s", minInterval)
	}
	u, err := url.Parse(c.Broker)
	if err != nil || u.Host == "" || !allowedScheme(u.Scheme) {
		return errors.New("broker URL is invalid")
	}
	if c.Firmware == "" || len(c.Firmware) > 64 || strings.ContainsAny(c.Firmware, "\r\n") {
		return errors.New("firmware version is invalid")
	}
	return nil
}

func redactBroker(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return u.Redacted()
}

func allowedScheme(scheme string) bool {
	switch scheme {
	case "tcp", "ssl", "ws", "wss", "mqtt", "mqtts":
		return true
	default:
		return false
	}
}
