# Phase 13 — ESP32 starter

**Status:** complete

**Depends on:** Phases 4 and 10

**Spec:** `plan.md` sections 31, 65, 73–76

## Goal

A fresh ESP32 connects after copying credentials from the dashboard and flashing this example.

## Tasks

- [x] Finish `examples/esp32-basic` (PlatformIO sketch already stubbed)
- [x] Wi-Fi and MQTT connect, with exponential backoff and resubscribe
- [x] Authenticate as the device, never with a broker admin password
- [x] Last Will offline message and online state including `firmwareVersion`
- [x] Periodic sample telemetry
- [x] `set_led` command, acknowledgement, and a small command-id cache
- [x] README with flash and credential steps

## Acceptance criteria

A fresh ESP32 can be connected by copying credentials and flashing the starter firmware.

## Notes

- Credentials stay in `src/main.cpp` as empty defines. The setup wizard prints the MQTT host, device key, secret, and project id to paste in. Wi-Fi is filled in separately. Do not commit them.
- MQTT username is the device key and the password is the device secret. The sketch does not contain a broker admin password.
- Status is retained QoS 1. The Last Will is `{"status":"offline"}`. On connect the sketch publishes `{"status":"online"}` and state `{"firmwareVersion":"1.0.0","led":false}`.
- Telemetry is QoS 0 and not retained, every 5 seconds: `temperature`, `humidity`, and `led`. `timestamp` is included after NTP succeeds.
- `set_led` expects `{"id","command":"set_led","payload":{"enabled":true|false}}`. The acknowledgement is QoS 1. The last 8 ids are cached, so a duplicate delivery does not toggle the LED again.
- A failed Wi-Fi or broker attempt waits 1 second, then 2, 4, and so on up to 30 seconds. The loop does not block forever. After reconnect the sketch subscribes to commands and announces itself again.
- The library is `256dpi/MQTT` (the arduino-mqtt client) because it can publish QoS 1. PubSubClient publishes only at QoS 0.
