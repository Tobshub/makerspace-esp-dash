# Phase 7 — Realtime updates

**Status:** not started

**Depends on:** Phases 4 and 6

**Spec:** `plan.md` section 25 and 59

## Goal

Dashboard values change when telemetry arrives, without a manual refresh.

## Tasks

- [ ] `GET /api/v1/projects/:projectId/stream` using Server-Sent Events
- [ ] Authorize the subscription to the project's team
- [ ] Emit `device.online`, `device.offline`, `telemetry.received`, `state.updated`, and `command.updated`
- [ ] Frontend EventSource client that updates the TanStack Query cache
- [ ] Reconnect handling

## Acceptance criteria

Dashboard values update without manual refresh when telemetry arrives.
