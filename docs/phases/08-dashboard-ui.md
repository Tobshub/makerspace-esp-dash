# Phase 8 — Dashboard UI

**Status:** complete

**Depends on:** Phases 6 and 7

**Spec:** `plan.md` sections 26–29, 33, 50, 51, 60, 81

## Goal

A person can open a project and see current device status without reading raw API data.

## Tasks

- [x] Overview: device totals, online/offline, messages, metric cards, chart, recent events, device table
- [x] Device list columns and actions
- [x] Device detail: summary, latest telemetry, charts, state, commands, events
- [x] Loading, empty, and error states
- [x] Sidebar navigation already stubbed in `frontend/src/components/AppShell.tsx`

Automatic widgets from metric display types can stay simple until Phase 9. Do not build a drag-and-drop layout editor.

## Acceptance criteria

A user can open a project and see current IoT status without navigating through raw API data.

## Notes

- Overview is `/projects/:projectId/overview`. It reads `GET /api/v1/projects/:projectId/overview`, latest telemetry, recent events, and device history.
- Counts exclude disabled devices from the total. Messages today are telemetry events since UTC midnight. Active alerts stay at 0 until Phase 12.
- Metric cards are the newest raw keys, up to eight, with a small chart on the first four numeric series. There is no widget editor.
- The device list adds Open, Edit, Rotate secret, Disable, and Delete. Rotate, disable, and delete stay hidden for viewers.
- Device detail shows status, firmware, last seen, Wi-Fi RSSI when state includes `wifi_rssi`, `wifiRssi`, or `rssi`, and created time. Charts take a numeric metric and a 1 hour, 24 hour, or 7 day window.
- State is the latest JSON document. Commands are `command_ack` events until Phase 10 stores command history. Events are the latest device activity.
