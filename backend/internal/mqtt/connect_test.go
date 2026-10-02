package mqtt

import (
	"net"
	"testing"
	"time"
)

func TestConnectLocalBroker(t *testing.T) {
	conn, err := net.DialTimeout("tcp", "localhost:1883", time.Second)
	if err != nil {
		t.Skip("mqtt broker not reachable")
	}
	_ = conn.Close()

	broker := Connect("tcp://localhost:1883", "", "")
	defer broker.Close()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if broker.Connected() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("broker is up but the client did not connect")
}
