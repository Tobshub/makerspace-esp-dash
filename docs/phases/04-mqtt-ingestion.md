# Phase 4 — MQTT ingestion

**Status:** complete

**Depends on:** Phase 3

**Spec:** `plan.md` sections 6.6–6.8, 6.14, 7, 9, 10, 12, 13, 39, 56, 72, 74

## Goal

A valid MQTT telemetry publish becomes database rows and an updated device presence.

## Tasks

- [x] Subscribe to `makerspace/v1/projects/{projectId}/devices/{deviceKey}/...`
- [x] Resolve the device from the topic and authenticate it when broker auth is on
- [x] Parse and validate telemetry: scalar metrics only, size and rate limits, no nested objects
- [x] Store one telemetry row per metric value, including unknown keys
- [x] Update `last_seen_at` and latest `device_state`
- [x] Store a debug event
- [x] Presence via Last Will plus a configurable offline timeout (default 60 seconds)
- [x] Keep parsing out of HTTP handlers: subscriber → parser → validator → service → repository

## Acceptance criteria

Publishing valid MQTT telemetry results in database rows and updated device status.

## Notes

- The API subscribes to `makerspace/v1/projects/+/devices/+/#` and resubscribes after reconnect. Parsing lives in `internal/telemetry`. `internal/ingest` resolves the device and writes rows. HTTP handlers do not see payloads.
- Telemetry accepts numbers, booleans, and strings. Nested objects, arrays, nulls, and non-finite numbers are rejected and stored as an `error` debug event. Unknown metric keys are stored without a metric definition.
- `timestamp` is Unix milliseconds. When it is omitted, `recorded_at` is the server receive time. `received_at` is always the server time.
- State is the latest JSON document, not history. A string `firmwareVersion` updates the device row.
- Presence uses `.../status`. Last Will payload is `{"status":"offline"}`. On connect, publish retained `{"status":"online"}`. A live telemetry, state, event, or acknowledgement also marks the device online and refreshes `last_seen_at`.
- Retained telemetry is ignored so a restart does not replay a stream. Retained online status does not extend `last_seen_at` or revive a device that is already past the offline timeout.
- The API sweeps every 5 seconds and marks a device offline when `now - last_seen_at` is greater than `DEVICE_OFFLINE_TIMEOUT_SECONDS` (default 60). `make worker` runs the same sweep.
- Defaults: 16 KiB payloads, 50 metrics, 64-character keys, 1024-character strings, 10 telemetry messages per second per device. The rate limit is in-process.
- The subscriber cannot see the publisher password. It drops unknown devices, project mismatches, and disabled devices. When `MQTT_USERNAME` is set, the broker must authenticate `device_key` / `device_secret` and limit each device to its own topic prefix. Local Mosquitto still allows anonymous publishes.
- Command acknowledgements are stored as debug events and refresh presence. Command execution is Phase 10. The API ignores the `commands` leaf so its own publishes are not ingested.
- The device list and device page poll every 5 seconds so online and offline changes show up before the realtime stream in Phase 7.
