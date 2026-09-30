# Phase 11 — Debug console

**Status:** not started

**Depends on:** Phases 4 and 7

**Spec:** `plan.md` sections 6.14, 24, 38, 63

## Goal

A team can see whether a device is connecting, sending data, receiving commands, and acknowledging them.

## Tasks

- [ ] Store device events (`connected`, `disconnected`, `telemetry`, `state`, `command`, `command_ack`, `error`)
- [ ] `GET /projects/:projectId/events` and `GET /devices/:deviceId/events`
- [ ] Filters: device, event type, time range
- [ ] Live stream on `/projects/:projectId/events`
- [ ] Expandable raw JSON

This page is high priority for prototype debugging. Event storage begins in Phase 4; this phase is the console.

## Acceptance criteria

A Makerspace team can diagnose device connection, telemetry, commands, and acknowledgements from the events page.
