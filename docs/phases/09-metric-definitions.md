# Phase 9 — Metric definitions

**Status:** not started

**Depends on:** Phases 4 and 8

**Spec:** `plan.md` sections 6.6, 20, 32, 34, 61

## Goal

A team turns an arbitrary telemetry key into a dashboard widget without a backend change.

## Tasks

- [ ] Metric definition CRUD
- [ ] `GET /projects/:projectId/metrics/discovered` for keys that have no definition yet
- [ ] Configure display name, unit, data type, display type, min, max, and description
- [ ] Render number, line, gauge, boolean, and text widgets from those definitions
- [ ] Metrics page empty state

Telemetry must keep arriving before any definition exists.

## Acceptance criteria

A team can turn an arbitrary telemetry key into a useful dashboard visualization without backend changes.
