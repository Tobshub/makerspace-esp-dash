package main

import (
	"strings"
	"testing"
	"time"
)

func TestParseArgsRequiresCredentials(t *testing.T) {
	_, usage, err := parseArgs(nil)
	if err != errUsage {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(usage, "--device-key") || strings.Contains(usage, "shown-once") {
		t.Fatalf("usage = %s", usage)
	}
}

func TestParseArgsHidesSecret(t *testing.T) {
	const secret = "super-secret-value"
	cfg, _, err := parseArgs([]string{
		"--device-key", "dev_0123456789abcdef0123",
		"--secret", secret,
		"--project-id", "AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE",
		"--interval", "1s",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Secret != secret || cfg.Interval != time.Second {
		t.Fatalf("cfg = %+v", cfg)
	}
	if cfg.ProjectID != "AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE" {
		t.Fatalf("project = %s", cfg.ProjectID)
	}
	_, usage, err := parseArgs([]string{"--help"})
	if err != errHelp {
		t.Fatal(err)
	}
	if strings.Contains(usage, secret) {
		t.Fatal("usage printed the secret")
	}
}

func TestParseArgsRejectsBadIdentity(t *testing.T) {
	_, _, err := parseArgs([]string{
		"--device-key", "nope",
		"--secret", "secret",
		"--project-id", "not-a-uuid",
	})
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("err = %v", err)
	}
}
