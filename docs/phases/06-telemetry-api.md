# Phase 6 — Telemetry API

**Status:** not started

**Depends on:** Phase 4

**Spec:** `plan.md` sections 19, 41, 58

## Goal

The frontend can read latest values and a historical series for any numeric metric.

## Tasks

- [ ] `GET /devices/:deviceId/telemetry/latest`
- [ ] `GET /devices/:deviceId/telemetry` with `metric`, `from`, `to`, `limit`, `resolution`
- [ ] `GET /projects/:projectId/telemetry/latest`
- [ ] Query validation and the indexes from `plan.md` section 6.7
- [ ] Support `resolution=raw` now, and leave the parameter in place for `1m`, `5m`, `1h`, and `1d`
- [ ] Tests for storage and retrieval

## Acceptance criteria

Latest values and historical series are available for any numeric metric the device has sent.
