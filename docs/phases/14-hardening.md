# Phase 14 — Hardening

**Status:** not started

**Depends on:** the MVP loop through Phase 13

**Spec:** `plan.md` sections 39, 40, 43, 45, 46, 66, 86

## Goal

Close the security, limit, and operability gaps that the earlier phases deliberately left open.

## Tasks

- [ ] Production MQTT TLS instructions and per-device topic ACL strategy
- [ ] Payload size limits and per-device message rate limits
- [ ] API rate limiting where it helps
- [ ] Authorization review across project routes
- [ ] Query performance review on telemetry indexes
- [ ] Secret and logging audit (no plaintext device secrets)
- [ ] Telemetry and device-event retention job
- [ ] Command timeout worker
- [ ] End-to-end test: project, device, simulated MQTT, telemetry, command, acknowledgement

Prometheus metrics stay optional.

## Acceptance criteria

The checklist in `plan.md` section 86 holds for the paths this phase touches, and the MVP workflow in section 67 still passes.
