# Phase 9 — Metric definitions

**Status:** complete

**Depends on:** Phases 4 and 8

**Spec:** `plan.md` sections 6.6, 20, 32, 34, 61

## Goal

A team turns an arbitrary telemetry key into a dashboard widget without a backend change.

## Tasks

- [x] Metric definition CRUD
- [x] `GET /projects/:projectId/metrics/discovered` for keys that have no definition yet
- [x] Configure display name, unit, data type, display type, min, max, and description
- [x] Render number, line, gauge, boolean, and text widgets from those definitions
- [x] Metrics page empty state

Telemetry must keep arriving before any definition exists.

## Acceptance criteria

A team can turn an arbitrary telemetry key into a useful dashboard visualization without backend changes.

## Notes

- Definitions live in `metric_definitions`, unique per project and key. Ingestion does not read this table.
- `GET /api/v1/projects/:projectId/metrics/discovered` lists telemetry keys that have no definition, with the data type of the latest value.
- Number metrics can display as a number, line chart, or gauge. A gauge requires min and max. Booleans display as on/off or status. Strings display as text.
- The metrics page is `/projects/:projectId/metrics`. The overview and device page render a widget for each definition. Unconfigured keys stay as plain latest-value cards.
- Viewers can read definitions. Creating, editing, and deleting them requires a member, admin, or owner.
