# Phase 5 — Device simulator

**Status:** complete

**Depends on:** Phase 4

**Spec:** `plan.md` sections 47, 48, 57, 73, 74, 75

## Goal

Develop and demo the pipeline without a physical ESP32.

## Tasks

- [x] Connect with device credentials
- [x] Publish drifting greenhouse telemetry (`temperature`, `humidity`, `soil_moisture`, `pump_active`)
- [x] Receive commands, including `set_pump`
- [x] Publish acknowledgements
- [x] Reconnect after failure

## Acceptance criteria

A developer can demo the complete device pipeline without an ESP32:

```bash
go run . --device-key "..." --secret "..." --project-id "..."
```

## Notes

- The CLI lives in `tools/device-simulator`. The MQTT username is the device key and the password is the one-time secret. The process never prints the secret.
- Topics match ingestion: telemetry (QoS 0), state (QoS 1), retained status, commands (subscribe, QoS 1), and `commands/ack` (QoS 1).
- Temperature and humidity drift on a slow curve. Soil moisture falls until `set_pump` turns the pump on, then it rises. Telemetry is published every 5 seconds by default (`--interval`, minimum 200ms).
- `set_pump` expects `{"id":"...","command":"set_pump","payload":{"enabled":true}}`. The acknowledgement is `{"id":"...","success":true,"state":{"pump_active":true}}`. Unknown commands and a missing `enabled` flag are acknowledged with `success: false`. Payloads without an id are ignored.
- The last 64 command ids are remembered. A duplicate delivery returns the original acknowledgement and does not change the pump again.
- Last Will on the status topic is retained `{"status":"offline"}`. Connect publishes retained `{"status":"online"}` and a state document with `firmwareVersion` `sim-1.0.0`. Ctrl-C publishes offline before disconnecting.
- The client retries the initial connection and reconnects after a drop, with backoff up to 30 seconds. Each connect subscribes again and announces presence.
