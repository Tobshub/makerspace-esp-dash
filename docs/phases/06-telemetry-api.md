# Phase 6 — Telemetry API

**Status:** complete

**Depends on:** Phase 4

**Spec:** `plan.md` sections 19, 41, 58

## Goal

The frontend can read latest values and a historical series for any numeric metric.

## Tasks

- [x] `GET /devices/:deviceId/telemetry/latest`
- [x] `GET /devices/:deviceId/telemetry` with `metric`, `from`, `to`, `limit`, `resolution`
- [x] `GET /projects/:projectId/telemetry/latest`
- [x] Query validation and the indexes from `plan.md` section 6.7
- [x] Support `resolution=raw` now, and leave the parameter in place for `1m`, `5m`, `1h`, and `1d`
- [x] Tests for storage and retrieval

## Acceptance criteria

Latest values and historical series are available for any numeric metric the device has sent.

## Notes

- Routes sit under `/api/v1` and require a project member. Viewers can read. Another team's device or project is a 404.
- Latest returns one scalar per metric key, including booleans and strings. History takes `metric` and returns oldest-first points. `from` and `to` are inclusive RFC3339 timestamps. When they are omitted, the window is the last 24 hours.
- `limit` defaults to 500 and cannot exceed 5000. If the window has more rows, the newest `limit` points are returned and `truncated` is true.
- `resolution` accepts `raw`, `1m`, `5m`, `1h`, and `1d`. Only `raw` returns points. The other four are reserved for averages and are rejected until aggregation exists.
- Indexes from section 6.7 are already on `telemetry`: `(device_id, metric_key, recorded_at DESC)`, `(project_id, recorded_at DESC)`, and `(metric_key, recorded_at DESC)`.
- The device page lists the latest values and refreshes them every 5 seconds. Charts use the history endpoint in a later phase.
