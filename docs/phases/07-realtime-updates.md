# Phase 7 — Realtime updates

**Status:** complete

**Depends on:** Phases 4 and 6

**Spec:** `plan.md` section 25 and 59

## Goal

Dashboard values change when telemetry arrives, without a manual refresh.

## Tasks

- [x] `GET /api/v1/projects/:projectId/stream` using Server-Sent Events
- [x] Authorize the subscription to the project's team
- [x] Emit `device.online`, `device.offline`, `telemetry.received`, `state.updated`, and `command.updated`
- [x] Frontend EventSource client that updates the TanStack Query cache
- [x] Reconnect handling

## Acceptance criteria

Dashboard values update without manual refresh when telemetry arrives.

## Notes

- The stream is project-scoped. A member of the project's team may subscribe. Anyone else gets 404. A missing or bad session gets 401.
- EventSource cannot set `Authorization`, so this route also accepts `access_token` in the query string. Other routes still require the bearer header. The request log records the path, not the query.
- The API process publishes after a successful ingest commit, and when its own offline sweep marks a device offline. The optional worker sweep updates the database but does not reach open browsers. Pages also refetch about every 30 seconds, and a reconnected stream invalidates cached project data.
- Each event is JSON: `type`, `deviceId`, `timestamp`, and `data`. Telemetry `data` is the metric map from that publish.
- The shell opens one stream for the selected project. On error it closes and tries again after 3 seconds. The first open does not refetch; a later open does, so a gap is filled from the API.
