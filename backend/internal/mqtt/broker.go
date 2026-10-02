package mqtt

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
)

// Handler receives one publish. Payload is a copy. Retained reports the MQTT retain flag.
type Handler func(topic string, payload []byte, retained bool)

// Broker is the API process connection. It subscribes while a handler is set and reconnects on its own.
type Broker struct {
	client     pahomqtt.Client
	host       string
	port       int
	mu         sync.Mutex
	subMu      sync.Mutex
	handler    Handler
	subscribed bool
}

// ParsePublic extracts the host and port operators should show to devices.
func ParsePublic(raw string) (string, int, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", 0, err
	}
	host := u.Hostname()
	if host == "" {
		return "", 0, fmt.Errorf("mqtt broker url %q has no host", raw)
	}
	port := 1883
	if p := u.Port(); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil {
			return "", 0, err
		}
		port = n
	}
	return host, port, nil
}

// Connect starts a background connection with reconnect. The process keeps serving if the broker is down.
func Connect(brokerURL, username, password string) *Broker {
	host, port, err := ParsePublic(brokerURL)
	b := &Broker{host: host, port: port}
	if err != nil {
		slog.Error("mqtt broker url invalid")
		b.host = "localhost"
		b.port = 1883
		return b
	}
	opts := pahomqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID("makerspace-api-" + randomID()).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(5 * time.Second).
		SetCleanSession(true).
		SetOrderMatters(false)
	if username != "" {
		opts.SetUsername(username)
		opts.SetPassword(password)
	}
	opts.SetConnectionLostHandler(func(_ pahomqtt.Client, err error) {
		b.mu.Lock()
		b.subscribed = false
		b.mu.Unlock()
		slog.Error("mqtt connection lost", "err", err, "host", host, "port", port)
	})
	opts.SetOnConnectHandler(func(c pahomqtt.Client) {
		slog.Info("mqtt connected", "host", host, "port", port)
		b.subscribe(c)
	})
	b.client = pahomqtt.NewClient(opts)
	b.client.Connect()
	slog.Info("mqtt connecting", "host", host, "port", port)
	return b
}

func (b *Broker) Connected() bool {
	return b != nil && b.client != nil && b.client.IsConnectionOpen()
}

// Subscribed reports whether the ingest filter is active on the current connection.
func (b *Broker) Subscribed() bool {
	if b == nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.subscribed
}

// OnMessage starts delivery. A later reconnect subscribes again.
func (b *Broker) OnMessage(handler Handler) {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.handler = handler
	b.subscribed = false
	client := b.client
	open := client != nil && client.IsConnectionOpen()
	b.mu.Unlock()
	if open {
		b.subscribe(client)
	}
}

func (b *Broker) subscribe(c pahomqtt.Client) {
	if b == nil || c == nil {
		return
	}
	b.subMu.Lock()
	defer b.subMu.Unlock()
	b.mu.Lock()
	handler := b.handler
	b.mu.Unlock()
	if handler == nil {
		return
	}
	token := c.Subscribe(IngestFilter, 1, func(_ pahomqtt.Client, msg pahomqtt.Message) {
		payload := append([]byte(nil), msg.Payload()...)
		handler(msg.Topic(), payload, msg.Retained())
	})
	if !token.WaitTimeout(5*time.Second) || token.Error() != nil {
		slog.Error("mqtt subscribe failed", "filter", IngestFilter, "err", token.Error())
		return
	}
	b.mu.Lock()
	b.subscribed = true
	b.mu.Unlock()
	slog.Info("mqtt subscribed", "filter", IngestFilter)
}

func (b *Broker) Host() string {
	if b == nil || b.host == "" {
		return "localhost"
	}
	return b.host
}

func (b *Broker) Port() int {
	if b == nil || b.port == 0 {
		return 1883
	}
	return b.port
}

func (b *Broker) Close() {
	if b != nil && b.client != nil {
		b.client.Disconnect(250)
	}
}

func randomID() string {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "local"
	}
	return hex.EncodeToString(buf)
}

// SafeHost drops values that would break a generated firmware snippet.
func SafeHost(host string) string {
	if host == "" || strings.ContainsAny(host, "\"\\\n\r") {
		return "localhost"
	}
	return host
}
