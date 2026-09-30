# Phase 5 — Device simulator

**Status:** not started

**Depends on:** Phase 4

**Spec:** `plan.md` sections 47, 48, 57

## Goal

Develop and demo the pipeline without a physical ESP32.

## Tasks

- [ ] Connect with device credentials
- [ ] Publish drifting greenhouse telemetry (`temperature`, `humidity`, `soil_moisture`, `pump_active`)
- [ ] Receive commands, including `set_pump`
- [ ] Publish acknowledgements
- [ ] Reconnect after failure

The CLI already lives at `tools/device-simulator` and rejects a missing key, secret, or project id. It does not open MQTT yet.

## Acceptance criteria

A developer can demo the complete device pipeline without an ESP32:

```bash
go run . --device-key "..." --secret "..." --project-id "..."
```
