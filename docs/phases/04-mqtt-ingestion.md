# Phase 4 — MQTT ingestion

**Status:** not started

**Depends on:** Phase 3

**Spec:** `plan.md` sections 6.6–6.8, 6.14, 7, 9, 10, 12, 13, 39, 56, 72, 74

## Goal

A valid MQTT telemetry publish becomes database rows and an updated device presence.

## Tasks

- [ ] Subscribe to `makerspace/v1/projects/{projectId}/devices/{deviceKey}/...`
- [ ] Resolve the device from the topic and authenticate it when broker auth is on
- [ ] Parse and validate telemetry: scalar metrics only, size and rate limits, no nested objects
- [ ] Store one telemetry row per metric value, including unknown keys
- [ ] Update `last_seen_at` and latest `device_state`
- [ ] Store a debug event
- [ ] Presence via Last Will plus a configurable offline timeout (default 60 seconds)
- [ ] Keep parsing out of HTTP handlers: subscriber → parser → validator → service → repository

## Acceptance criteria

Publishing valid MQTT telemetry results in database rows and updated device status.
