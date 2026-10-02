package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net"
	"testing"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	broker "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
)

func TestSimulatorPublishesAndAcknowledges(t *testing.T) {
	addr := freeAddr(t)
	startBroker(t, addr)
	ctx, stop := context.WithCancel(context.Background())
	defer stop()

	cfg := testConfig(t, "tcp://"+addr)
	cfg.Interval = 200 * time.Millisecond
	cfg.connectRetry = 100 * time.Millisecond
	errCh := make(chan error, 1)
	go func() { errCh <- Run(ctx, cfg) }()

	observer := dial(t, "tcp://"+addr, "observer-"+cfg.DeviceKey)
	defer observer.Disconnect(200)
	topics := topicsFor(cfg.ProjectID, cfg.DeviceKey)
	statusCh := subscribeOne(t, observer, topics.status)
	telemetryCh := subscribeOne(t, observer, topics.telemetry)
	ackCh := subscribeOne(t, observer, topics.ack)

	status := waitPayload(t, statusCh, 8*time.Second)
	var presence struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(status, &presence); err != nil || presence.Status != "online" {
		t.Fatalf("status = %s (%v)", status, err)
	}
	sample := waitPayload(t, telemetryCh, 8*time.Second)
	if !json.Valid(sample) || !containsMetric(sample, "temperature") {
		t.Fatalf("telemetry = %s", sample)
	}

	command := []byte(`{"id":"cmd_pump","command":"set_pump","payload":{"enabled":true}}`)
	token := observer.Publish(topics.commands, 1, false, command)
	if !token.WaitTimeout(5*time.Second) || token.Error() != nil {
		t.Fatalf("publish command: %v", token.Error())
	}
	ackRaw := waitPayload(t, ackCh, 8*time.Second)
	var ack ackBody
	if err := json.Unmarshal(ackRaw, &ack); err != nil {
		t.Fatal(err)
	}
	if ack.ID != "cmd_pump" || !ack.Success || ack.State == nil || !ack.State.PumpActive {
		t.Fatalf("ack = %+v raw=%s", ack, ackRaw)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		raw := waitPayload(t, telemetryCh, time.Until(deadline))
		var packet struct {
			Metrics struct {
				Pump bool `json:"pump_active"`
			} `json:"metrics"`
		}
		if json.Unmarshal(raw, &packet) == nil && packet.Metrics.Pump {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("telemetry did not report the pump: %s", raw)
		}
	}

	stop()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("simulator did not stop")
	}
	offline := waitPayload(t, statusCh, 5*time.Second)
	if err := json.Unmarshal(offline, &presence); err != nil || presence.Status != "offline" {
		t.Fatalf("offline status = %s (%v)", offline, err)
	}
}

func TestSimulatorRetriesUntilBrokerExists(t *testing.T) {
	addr := freeAddr(t)
	ctx, stop := context.WithCancel(context.Background())
	defer stop()

	cfg := testConfig(t, "tcp://"+addr)
	cfg.Interval = 200 * time.Millisecond
	cfg.connectRetry = 100 * time.Millisecond
	errCh := make(chan error, 1)
	go func() { errCh <- Run(ctx, cfg) }()

	time.Sleep(300 * time.Millisecond)
	select {
	case err := <-errCh:
		t.Fatalf("simulator exited before the broker existed: %v", err)
	default:
	}

	startBroker(t, addr)
	observer := dial(t, "tcp://"+addr, "observer-late-"+cfg.DeviceKey)
	defer observer.Disconnect(200)
	topics := topicsFor(cfg.ProjectID, cfg.DeviceKey)
	telemetryCh := subscribeOne(t, observer, topics.telemetry)
	raw := waitPayload(t, telemetryCh, 8*time.Second)
	if !containsMetric(raw, "soil_moisture") {
		t.Fatalf("telemetry = %s", raw)
	}
	stop()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("simulator did not stop")
	}
}

func testConfig(t *testing.T, brokerURL string) Config {
	t.Helper()
	return Config{
		DeviceKey: testDeviceKey(t),
		Secret:    "test-secret",
		ProjectID: "12345678-9abc-def0-1234-56789abcdef0",
		Broker:    brokerURL,
		Interval:  time.Second,
		Firmware:  defaultFirmware,
	}
}

func testDeviceKey(t *testing.T) string {
	t.Helper()
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal(err)
	}
	return "dev_" + hex.EncodeToString(buf)
}

func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr
}

func startBroker(t *testing.T, addr string) *broker.Server {
	t.Helper()
	server := broker.New(nil)
	if err := server.AddHook(new(auth.AllowHook), nil); err != nil {
		t.Fatal(err)
	}
	listener := listeners.NewTCP(listeners.Config{ID: "sim", Address: addr})
	if err := server.AddListener(listener); err != nil {
		t.Fatal(err)
	}
	go func() { _ = server.Serve() }()
	t.Cleanup(func() { _ = server.Close() })
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return server
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("broker did not listen")
	return nil
}

func dial(t *testing.T, brokerURL, clientID string) pahomqtt.Client {
	t.Helper()
	opts := pahomqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID(clientID).
		SetAutoReconnect(false).
		SetConnectRetry(false).
		SetConnectTimeout(3 * time.Second)
	client := pahomqtt.NewClient(opts)
	token := client.Connect()
	if !token.WaitTimeout(5*time.Second) || token.Error() != nil {
		t.Fatalf("observer connect: %v", token.Error())
	}
	return client
}

func subscribeOne(t *testing.T, client pahomqtt.Client, topic string) <-chan []byte {
	t.Helper()
	ch := make(chan []byte, 8)
	token := client.Subscribe(topic, 1, func(_ pahomqtt.Client, msg pahomqtt.Message) {
		ch <- append([]byte(nil), msg.Payload()...)
	})
	if !token.WaitTimeout(5*time.Second) || token.Error() != nil {
		t.Fatalf("subscribe %s: %v", topic, token.Error())
	}
	return ch
}

func waitPayload(t *testing.T, ch <-chan []byte, timeout time.Duration) []byte {
	t.Helper()
	if timeout <= 0 {
		t.Fatal("timed out waiting for mqtt payload")
	}
	select {
	case payload := <-ch:
		return payload
	case <-time.After(timeout):
		t.Fatal("timed out waiting for mqtt payload")
	}
	return nil
}

func containsMetric(payload []byte, key string) bool {
	var packet struct {
		Metrics map[string]json.RawMessage `json:"metrics"`
	}
	if json.Unmarshal(payload, &packet) != nil {
		return false
	}
	_, ok := packet.Metrics[key]
	return ok
}
