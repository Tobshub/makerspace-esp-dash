package ingest

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/alerts"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/commands"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/config"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/devices"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/mqtt"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/realtime"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/telemetry"
)

const (
	defaultPayloadBytes = 16 * 1024
	sweepInterval       = 5 * time.Second
)

// Options configure validation and presence. Zero values use the MVP defaults.
type Options struct {
	OfflineTimeout     time.Duration
	MaxPayloadBytes    int
	MaxMetrics         int
	MaxKeyLength       int
	MaxStringLength    int
	TelemetryPerSecond int
	CommandTimeout     time.Duration
	Now                func() time.Time
}

// Service persists device publishes. It does not parse inside HTTP handlers.
type Service struct {
	pool           *pgxpool.Pool
	limits         telemetry.Limits
	maxPayload     int
	offlineTimeout time.Duration
	commandTimeout time.Duration
	now            func() time.Time
	limiter        *limiter
	notify         func(projectID string, event realtime.Event)
}

// PublishTo fans stored changes out to browser subscribers. A nil publisher disables it.
func (s *Service) PublishTo(p realtime.Publisher) {
	if s == nil {
		return
	}
	if p == nil {
		s.notify = nil
		return
	}
	s.notify = p.Publish
}

// FromConfig builds the ingestion service from process configuration.
func FromConfig(pool *pgxpool.Pool, cfg config.Config) *Service {
	return New(pool, Options{
		OfflineTimeout:     cfg.DeviceOfflineTimeout,
		MaxPayloadBytes:    cfg.MQTTMaxPayloadBytes,
		MaxMetrics:         cfg.MQTTMaxMetrics,
		MaxKeyLength:       cfg.MQTTMaxKeyLength,
		MaxStringLength:    cfg.MQTTMaxStringLength,
		TelemetryPerSecond: cfg.MQTTTelemetryPerSecond,
		CommandTimeout:     cfg.CommandTimeout,
	})
}

// New builds a service. A nil pool accepts publishes and stores nothing.
func New(pool *pgxpool.Pool, opt Options) *Service {
	if opt.Now == nil {
		opt.Now = time.Now
	}
	if opt.OfflineTimeout <= 0 {
		opt.OfflineTimeout = 60 * time.Second
	}
	if opt.MaxPayloadBytes <= 0 {
		opt.MaxPayloadBytes = defaultPayloadBytes
	}
	if opt.CommandTimeout <= 0 {
		opt.CommandTimeout = 30 * time.Second
	}
	return &Service{
		pool: pool,
		limits: telemetry.Limits{
			MaxMetrics:      opt.MaxMetrics,
			MaxKeyLength:    opt.MaxKeyLength,
			MaxStringLength: opt.MaxStringLength,
		},
		maxPayload:     opt.MaxPayloadBytes,
		offlineTimeout: opt.OfflineTimeout,
		now:            opt.Now,
		commandTimeout: opt.CommandTimeout,
		limiter:        newLimiter(opt.TelemetryPerSecond, opt.Now),
	}
}

// OnMQTT is the broker callback. It recovers panics and logs storage failures.
func (s *Service) OnMQTT(topic string, payload []byte, retained bool) {
	if s == nil || s.pool == nil {
		return
	}
	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("ingest panic", "mqtt_topic", topic)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.HandleMessage(ctx, topic, payload, retained); err != nil {
		slog.Error("ingest failed", "err", err, "mqtt_topic", topic)
	}
}

// Handle stores a live publish. Retained replay uses HandleMessage.
func (s *Service) Handle(ctx context.Context, topic string, payload []byte) error {
	return s.HandleMessage(ctx, topic, payload, false)
}

// HandleMessage resolves the device from the topic, validates the leaf, and stores the result.
// Disabled devices are dropped. Unknown devices and project mismatches are dropped.
// The subscriber cannot see the publisher password. When the broker requires
// authentication, topic ACLs must bind username device_key to that device's prefix.
// This service still drops unknown and disabled devices in that mode.
func (s *Service) HandleMessage(ctx context.Context, topic string, payload []byte, retained bool) error {
	if s == nil || s.pool == nil {
		return nil
	}
	parsed, ok := mqtt.ParseDeviceTopic(topic)
	if !ok {
		return nil
	}
	if retained && (parsed.Leaf == "telemetry" || parsed.Leaf == "events" || parsed.Leaf == "commands/ack") {
		return nil
	}
	device, found, err := s.lookup(ctx, parsed.ProjectID, parsed.DeviceKey)
	if err != nil {
		return err
	}
	if !found {
		slog.Debug("ingest skipped", "reason", "unknown device", "mqtt_topic", topic)
		return nil
	}
	if device.Status == devices.StatusDisabled {
		slog.Debug("ingest skipped", "reason", "disabled", "project_id", device.ProjectID, "device_id", device.ID, "device_key", device.DeviceKey, "mqtt_topic", topic)
		return nil
	}
	if len(payload) == 0 || len(payload) > s.maxPayload {
		if retained {
			return nil
		}
		reason := "empty payload"
		if len(payload) > s.maxPayload {
			reason = "payload too large"
		}
		slog.Warn("ingest rejected", "reason", reason, "project_id", device.ProjectID, "device_id", device.ID, "device_key", device.DeviceKey, "mqtt_topic", topic)
		return s.writeError(ctx, device, topic, reason)
	}
	switch parsed.Leaf {
	case "telemetry":
		return s.ingestTelemetry(ctx, device, topic, payload)
	case "state":
		return s.ingestState(ctx, device, topic, payload, retained)
	case "status":
		return s.ingestStatus(ctx, device, topic, payload, retained)
	case "events":
		return s.ingestEvent(ctx, device, topic, payload)
	case "commands/ack":
		return s.ingestAck(ctx, device, topic, payload)
	default:
		return nil
	}
}

func (s *Service) ingestTelemetry(ctx context.Context, device deviceRow, topic string, payload []byte) error {
	if !s.limiter.allow(device.ID) {
		slog.Warn("ingest rate limited", "project_id", device.ProjectID, "device_id", device.ID, "device_key", device.DeviceKey, "mqtt_topic", topic)
		return nil
	}
	received := s.now().UTC()
	packet, err := telemetry.Parse(payload, s.limits, received)
	if err != nil {
		slog.Warn("ingest rejected", "reason", err.Error(), "project_id", device.ProjectID, "device_id", device.ID, "device_key", device.DeviceKey, "mqtt_topic", topic)
		return s.writeError(ctx, device, topic, err.Error())
	}
	return s.persist(ctx, device, change{
		status:        devices.StatusOnline,
		touchLastSeen: true,
		eventType:     "telemetry",
		topic:         topic,
		payload:       payload,
		points:        packet.Points,
		recorded:      packet.RecordedAt,
		received:      packet.ReceivedAt,
	})
}

func (s *Service) ingestState(ctx context.Context, device deviceRow, topic string, payload []byte, retained bool) error {
	state, err := telemetry.ParseState(payload)
	if err != nil {
		if retained {
			return nil
		}
		slog.Warn("ingest rejected", "reason", err.Error(), "project_id", device.ProjectID, "device_id", device.ID, "device_key", device.DeviceKey, "mqtt_topic", topic)
		return s.writeError(ctx, device, topic, err.Error())
	}
	if retained {
		ch := change{state: state.Body, firmware: state.FirmwareVersion, received: s.now().UTC()}
		switch state.Presence {
		case "offline":
			if device.Status != devices.StatusOffline {
				ch.status = devices.StatusOffline
				ch.eventType = "disconnected"
				ch.topic = topic
				ch.payload = state.Body
			}
		case "online":
			if s.fresh(device) && device.Status != devices.StatusOnline {
				ch.status = devices.StatusOnline
				ch.eventType = "connected"
				ch.topic = topic
				ch.payload = state.Body
			}
		}
		return s.persist(ctx, device, ch)
	}
	status := devices.StatusOnline
	eventType := "state"
	if state.Presence == "offline" {
		status = devices.StatusOffline
		eventType = "disconnected"
	}
	return s.persist(ctx, device, change{
		status:        status,
		touchLastSeen: true,
		eventType:     eventType,
		topic:         topic,
		payload:       state.Body,
		firmware:      state.FirmwareVersion,
		state:         state.Body,
		received:      s.now().UTC(),
	})
}

func (s *Service) ingestStatus(ctx context.Context, device deviceRow, topic string, payload []byte, retained bool) error {
	online, err := telemetry.ParsePresence(payload)
	if err != nil {
		if retained {
			return nil
		}
		slog.Warn("ingest rejected", "reason", err.Error(), "project_id", device.ProjectID, "device_id", device.ID, "device_key", device.DeviceKey, "mqtt_topic", topic)
		return s.writeError(ctx, device, topic, err.Error())
	}
	status := devices.StatusOnline
	eventType := "connected"
	if !online {
		status = devices.StatusOffline
		eventType = "disconnected"
	}
	if retained {
		if online {
			if !s.fresh(device) || device.Status == devices.StatusOnline {
				return nil
			}
		} else if device.Status == devices.StatusOffline {
			return nil
		}
		return s.persist(ctx, device, change{
			status:    status,
			eventType: eventType,
			topic:     topic,
			payload:   payload,
			received:  s.now().UTC(),
		})
	}
	return s.persist(ctx, device, change{
		status:        status,
		touchLastSeen: true,
		eventType:     eventType,
		topic:         topic,
		payload:       payload,
		received:      s.now().UTC(),
	})
}

func (s *Service) ingestEvent(ctx context.Context, device deviceRow, topic string, payload []byte) error {
	event, err := telemetry.ParseDeviceEvent(payload)
	if err != nil {
		slog.Warn("ingest rejected", "reason", err.Error(), "project_id", device.ProjectID, "device_id", device.ID, "device_key", device.DeviceKey, "mqtt_topic", topic)
		return s.writeError(ctx, device, topic, err.Error())
	}
	status := devices.StatusOnline
	if event.Type == "disconnected" {
		status = devices.StatusOffline
	}
	return s.persist(ctx, device, change{
		status:        status,
		touchLastSeen: true,
		eventType:     event.Type,
		topic:         topic,
		payload:       event.Body,
		received:      s.now().UTC(),
	})
}

func (s *Service) ingestAck(ctx context.Context, device deviceRow, topic string, payload []byte) error {
	if _, err := telemetry.ParseState(payload); err != nil {
		slog.Warn("ingest rejected", "reason", err.Error(), "project_id", device.ProjectID, "device_id", device.ID, "device_key", device.DeviceKey, "mqtt_topic", topic)
		return s.writeError(ctx, device, topic, err.Error())
	}
	if err := s.persist(ctx, device, change{
		status:        devices.StatusOnline,
		touchLastSeen: true,
		eventType:     "command_ack",
		topic:         topic,
		payload:       payload,
		received:      s.now().UTC(),
	}); err != nil {
		return err
	}
	ack, err := commands.ParseAck(payload)
	if err != nil {
		return nil
	}
	updated, err := commands.ApplyAck(ctx, s.pool, device.ID, ack, s.now().UTC())
	if err != nil || updated == nil || s.notify == nil {
		return err
	}
	s.notify(device.ProjectID, realtime.Event{
		Type:      realtime.CommandUpdated,
		DeviceID:  device.ID,
		Timestamp: s.now().UTC(),
		Data:      updated,
	})
	return nil
}

func (s *Service) fresh(device deviceRow) bool {
	if device.LastSeenAt == nil {
		return false
	}
	return s.now().UTC().Sub(device.LastSeenAt.UTC()) <= s.offlineTimeout
}

// RunSweep marks stale online devices offline until ctx is cancelled.
// The API process runs this. The worker can run it as well.
func (s *Service) RunSweep(ctx context.Context) {
	if s == nil || s.pool == nil {
		<-ctx.Done()
		return
	}
	s.sweepOnce(ctx)
	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sweepOnce(ctx)
		}
	}
}

func (s *Service) sweepOnce(ctx context.Context) {
	if _, err := s.Sweep(ctx); err != nil {
		slog.Error("offline sweep", "err", err)
	}
	if err := alerts.EvaluateOffline(ctx, s.pool, s.now().UTC(), s.notify); err != nil {
		slog.Error("offline alerts", "err", err)
	}
	if _, err := commands.Expire(ctx, s.pool, s.now().UTC(), s.commandTimeout, s.notify); err != nil {
		slog.Error("command timeout", "err", err)
	}
}
