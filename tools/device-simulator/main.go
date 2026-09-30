package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	fs := flag.NewFlagSet("device-simulator", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	deviceKey := fs.String("device-key", "", "public device key")
	secret := fs.String("secret", "", "one-time device secret (never printed)")
	broker := fs.String("broker", "tcp://localhost:1883", "MQTT broker URL")
	projectID := fs.String("project-id", "", "project id used in MQTT topics")

	fs.Usage = func() {
		fmt.Fprintf(os.Stdout, "Usage: device-simulator --device-key KEY --secret SECRET --project-id ID\n\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		if err == flag.ErrHelp {
			os.Exit(0)
		}
		os.Exit(2)
	}

	if *deviceKey == "" || *secret == "" || *projectID == "" {
		fs.Usage()
		os.Exit(2)
	}

	fmt.Printf("device simulator skeleton\ndevice_key=%s\nproject_id=%s\nbroker=%s\n", *deviceKey, *projectID, *broker)
	fmt.Println("MQTT connect, telemetry, and command acknowledgements land in Phase 5.")
}
