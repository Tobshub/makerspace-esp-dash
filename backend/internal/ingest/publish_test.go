package ingest

import (
	"context"
	"net"
	"testing"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/database"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/mqtt"
)

func TestPublishRoundTrip(t *testing.T) {
	conn, err := net.DialTimeout("tcp", "localhost:1883", time.Second)
	if err != nil {
		t.Skip("mqtt broker not reachable")
	}
	_ = conn.Close()

	pool := openPool(t)
	defer pool.Close()
	if err := database.Migrate(poolURL(t)); err != nil {
		t.Fatal(err)
	}
	projectID, cleanup := seedProject(t, pool)
	t.Cleanup(cleanup)
	deviceID, key := insertDevice(t, pool, projectID, "$2a$04$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ012", "unknown", nil)

	svc := New(pool, Options{})
	broker := mqtt.Connect("tcp://localhost:1883", "", "")
	defer broker.Close()
	broker.OnMessage(svc.OnMQTT)
	waitUntil(t, 5*time.Second, broker.Subscribed)

	topic := mqtt.DeviceTopic(projectID, key, "telemetry")
	publish(t, topic, []byte(`{"metrics":{"temperature":21.5,"pump_active":true}}`))

	waitUntil(t, 5*time.Second, func() bool {
		return metricCount(t, pool, deviceID) >= 2
	})
	status, _, seen := devicePresence(t, pool, deviceID)
	if status != "online" || seen == nil {
		t.Fatalf("status %s seen %v", status, seen)
	}
	var temperature float64
	if err := pool.QueryRow(context.Background(), `
		SELECT numeric_value FROM telemetry WHERE device_id = $1 AND metric_key = 'temperature'
	`, database.ID(deviceID)).Scan(&temperature); err != nil {
		t.Fatal(err)
	}
	if temperature != 21.5 {
		t.Fatal(temperature)
	}
}

func publish(t *testing.T, topic string, payload []byte) {
	t.Helper()
	opts := pahomqtt.NewClientOptions().
		AddBroker("tcp://localhost:1883").
		SetClientID("makerspace-test-" + time.Now().UTC().Format("150405.000"))
	client := pahomqtt.NewClient(opts)
	token := client.Connect()
	if !token.WaitTimeout(5*time.Second) || token.Error() != nil {
		t.Fatal(token.Error())
	}
	defer client.Disconnect(250)
	pub := client.Publish(topic, 0, false, payload)
	if !pub.WaitTimeout(5*time.Second) || pub.Error() != nil {
		t.Fatal(pub.Error())
	}
}

func waitUntil(t *testing.T, d time.Duration, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if ready() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("timed out")
}
