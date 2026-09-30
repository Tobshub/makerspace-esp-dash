# Phase 10 — Commands and controls

**Status:** not started

**Depends on:** Phases 4 and 5

**Spec:** `plan.md` sections 6.9, 6.10, 11, 21, 22, 35, 62, 74, 75

## Goal

A user sends a command from the dashboard and sees whether the device acknowledged it.

## Tasks

- [ ] Persist commands with status `pending`, `published`, `acknowledged`, `failed`, `timed_out`
- [ ] Publish to the device command topic at QoS 1 without blocking the HTTP request
- [ ] Handle acknowledgements and timeouts
- [ ] Control definitions: button, toggle, and slider
- [ ] UI states: Sending, Sent, Acknowledged, Failed, Timed out
- [ ] Command history on the device page

## Acceptance criteria

A user can control a simulated device or ESP32 from the dashboard and see whether the command was acknowledged.
