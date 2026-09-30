# Phase 8 — Dashboard UI

**Status:** not started

**Depends on:** Phases 6 and 7

**Spec:** `plan.md` sections 26–29, 33, 50, 51, 60, 81

## Goal

A person can open a project and see current device status without reading raw API data.

## Tasks

- [ ] Overview: device totals, online/offline, messages, metric cards, chart, recent events, device table
- [ ] Device list columns and actions
- [ ] Device detail: summary, latest telemetry, charts, state, commands, events
- [ ] Loading, empty, and error states
- [ ] Sidebar navigation already stubbed in `frontend/src/components/AppShell.tsx`

Automatic widgets from metric display types can stay simple until Phase 9. Do not build a drag-and-drop layout editor.

## Acceptance criteria

A user can open a project and see current IoT status without navigating through raw API data.
