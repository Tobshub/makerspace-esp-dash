# Phase 12 — Alerts

**Status:** complete

**Depends on:** Phases 4 and 7

**Spec:** `plan.md` sections 6.12, 6.13, 23, 36, 37, 64

## Goal

A user configures a threshold, sees it trigger, and sees it resolve.

## Tasks

- [x] Alert rules: `metric_threshold` and `device_offline`
- [x] Operators `>`, `>=`, `<`, `<=`, `==`, `!=`
- [x] Optional duration so brief spikes do not alert
- [x] Lifecycle: inactive, triggered, resolved, without duplicate active alerts
- [x] Alert events API and UI
- [x] Realtime `alert.triggered` and `alert.resolved`

## Acceptance criteria

A user can configure a threshold and see an alert trigger and later resolve.

## Notes

- Rules are `GET/POST /api/v1/projects/:projectId/alerts` and `PATCH/DELETE /api/v1/alerts/:alertId`. History is `GET /api/v1/projects/:projectId/alert-events`.
- A threshold compares the numeric telemetry value. `durationSeconds` is how long the condition must hold. `0` alerts on the sample that crosses the line.
- An offline rule alerts after the device has stayed offline for `durationSeconds`. Coming back online resolves it. The presence sweep checks rules whose duration has not elapsed yet.
- One open `triggered` row exists per rule and device. A later sample that still matches does not open another. Clearing the condition sets `resolved`.
- The overview count and a banner on the open project follow `alert.triggered` and `alert.resolved`. Email, SMS, push, and webhooks stay out of the MVP.
