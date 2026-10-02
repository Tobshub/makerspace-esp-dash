package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	cfg, usage, err := parseArgs(os.Args[1:])
	if err != nil {
		if errors.Is(err, errHelp) {
			fmt.Fprint(os.Stdout, usage)
			os.Exit(0)
		}
		if errors.Is(err, errUsage) {
			fmt.Fprint(os.Stdout, usage)
			os.Exit(2)
		}
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := Run(ctx, cfg); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

var (
	errHelp  = errors.New("help")
	errUsage = errors.New("usage")
)

func parseArgs(args []string) (Config, string, error) {
	fs := flag.NewFlagSet("device-simulator", flag.ContinueOnError)
	var buf bytes.Buffer
	fs.SetOutput(&buf)
	deviceKey := fs.String("device-key", "", "public device key")
	secret := fs.String("secret", "", "one-time device secret (never printed)")
	broker := fs.String("broker", "tcp://localhost:1883", "MQTT broker URL")
	projectID := fs.String("project-id", "", "project id used in MQTT topics")
	interval := fs.Duration("interval", defaultInterval, "telemetry publish interval")
	firmware := fs.String("firmware", defaultFirmware, "firmwareVersion published in state")
	fs.Usage = func() {
		fmt.Fprintf(&buf, "Usage: device-simulator --device-key KEY --secret SECRET --project-id ID\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return Config{}, buf.String(), errHelp
		}
		return Config{}, buf.String(), err
	}
	cfg := Config{
		DeviceKey: *deviceKey,
		Secret:    *secret,
		ProjectID: *projectID,
		Broker:    *broker,
		Interval:  *interval,
		Firmware:  *firmware,
	}
	if cfg.DeviceKey == "" || cfg.Secret == "" || cfg.ProjectID == "" {
		fs.Usage()
		return Config{}, buf.String(), errUsage
	}
	if err := cfg.validate(); err != nil {
		return Config{}, buf.String(), err
	}
	return cfg, buf.String(), nil
}
