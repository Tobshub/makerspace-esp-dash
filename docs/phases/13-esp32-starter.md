# Phase 13 — ESP32 starter

**Status:** not started

**Depends on:** Phases 4 and 10

**Spec:** `plan.md` sections 31, 65, 73–76

## Goal

A fresh ESP32 connects after copying credentials from the dashboard and flashing this example.

## Tasks

- [ ] Finish `examples/esp32-basic` (PlatformIO sketch already stubbed)
- [ ] Wi-Fi and MQTT connect, with exponential backoff and resubscribe
- [ ] Authenticate as the device, never with a broker admin password
- [ ] Last Will offline message and online state including `firmwareVersion`
- [ ] Periodic sample telemetry
- [ ] `set_led` command, acknowledgement, and a small command-id cache
- [ ] README with flash and credential steps

## Acceptance criteria

A fresh ESP32 can be connected by copying credentials and flashing the starter firmware.
