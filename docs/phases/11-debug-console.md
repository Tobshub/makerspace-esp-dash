# Phase 11 — Debug console

**Status:** complete

**Depends on:** Phases 4 and 7

**Spec:** `plan.md` sections 6.14, 24, 38, 63

## Goal

A team can see whether a device is connecting, sending data, receiving commands, and acknowledging them.

## Tasks

- [x] Store device events (`connected`, `disconnected`, `telemetry`, `state`, `command`, `command_ack`, `error`)
- [x] `GET /projects/:projectId/events` and `GET /devices/:deviceId/events`
- [x] Filters: device, event type, time range
- [x] Live stream on `/projects/:projectId/events`
- [x] Expandable raw JSON

## Acceptance criteria

A Makerspace team can diagnose device connection, telemetry, commands, and acknowledgements from the events page.

## Notes

- Event rows have been stored since Phase 4. This phase adds the console and the filters `event_type`, `device_id`, `from`, and `to` (RFC3339).
- A published command is stored as event type `command`. The device acknowledgement remains `command_ack`.
- The project SSE stream emits `device.event` after a stored event. The Events page refetches from that, and again about every 30 seconds.
- Each row expands to the raw JSON payload.
