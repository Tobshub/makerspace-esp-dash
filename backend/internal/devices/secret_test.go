package devices

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSecretIsHashed(t *testing.T) {
	secret, hash, err := GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}
	if secret == hash || !strings.HasPrefix(hash, "$2") {
		t.Fatal("secret must be stored as a bcrypt hash")
	}
	if !CheckSecret(hash, secret) || CheckSecret(hash, "other-secret") {
		t.Fatal("hash check failed")
	}
	if strings.Contains(hash, secret) {
		t.Fatal("hash contains the secret")
	}
}

func TestDeviceJSONOmitsSecret(t *testing.T) {
	body, err := json.Marshal(Device{Name: "Greenhouse", Metadata: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "secret") {
		t.Fatalf("device json leaked a secret field: %s", body)
	}
}

func TestDeviceKeyPrefix(t *testing.T) {
	key, err := GenerateDeviceKey()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(key, "dev_") || len(key) != 4+20 {
		t.Fatalf("key %q", key)
	}
}

func TestFirmwareSnippetPresence(t *testing.T) {
	snippet := FirmwareSnippet("localhost", 1883, "pid", "dev_abc", "secret")
	if !strings.Contains(snippet, `{"status":"offline"}`) || !strings.Contains(snippet, "/status") {
		t.Fatalf("snippet missing presence contract:\n%s", snippet)
	}
}
