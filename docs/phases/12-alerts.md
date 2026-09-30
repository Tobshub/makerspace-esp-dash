# Phase 12 — Alerts

**Status:** not started

**Depends on:** Phases 4 and 7

**Spec:** `plan.md` sections 6.12, 6.13, 23, 36, 37, 64

## Goal

A user configures a threshold, sees it trigger, and sees it resolve.

## Tasks

- [ ] Alert rules: `metric_threshold` and `device_offline`
- [ ] Operators `>`, `>=`, `<`, `<=`, `==`, `!=`
- [ ] Optional duration so brief spikes do not alert
- [ ] Lifecycle: inactive, triggered, resolved, without duplicate active alerts
- [ ] Alert events API and UI
- [ ] Realtime `alert.triggered` and `alert.resolved`

Dashboard notification is enough. Email, SMS, push, and webhooks stay out of the MVP.

## Acceptance criteria

A user can configure a threshold and see an alert trigger and later resolve.
