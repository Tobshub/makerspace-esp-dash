# Phase 10 — Commands and controls

**Status:** complete

**Depends on:** Phases 4 and 5

**Spec:** `plan.md` sections 6.9, 6.10, 11, 21, 22, 35, 62, 74, 75

## Goal

A user sends a command from the dashboard and sees whether the device acknowledged it.

## Tasks

- [x] Persist commands with status `pending`, `published`, `acknowledged`, `failed`, `timed_out`
- [x] Publish to the device command topic at QoS 1 without blocking the HTTP request
- [x] Handle acknowledgements and timeouts
- [x] Control definitions: button, toggle, and slider
- [x] UI states: Sending, Sent, Acknowledged, Failed, Timed out
- [x] Command history on the device page

## Acceptance criteria

A user can control a simulated device or ESP32 from the dashboard and see whether the command was acknowledged.

## Notes

- `POST /api/v1/devices/:deviceId/commands` returns `pending` immediately. A background publish marks the row `published`, or `failed` if MQTT is down.
- The published payload is `{id, command, payload, timestamp}`. `id` is the correlation id (`cmd_` plus random hex).
- Acknowledgements on `commands/ack` update `pending`, `published`, or `timed_out` rows. A late acknowledgement can still replace a timeout.
- Unpublished commands expire after `COMMAND_TIMEOUT_SECONDS` (default 30).
- Controls live in `control_definitions`. The Controls page turns a button, toggle, or slider into that command payload and shows the live status.
- Viewers can read history. Sending a command requires a member role. Disabled devices reject commands.
