# Phase 3 — Device registry

**Status:** complete

**Depends on:** Phase 2

**Spec:** `plan.md` sections 6.5, 8, 18, 30, 55, 71

## Goal

A project member can register a device and see its secret exactly once.

## Tasks

- [x] Device table with `device_key`, `secret_hash`, status, and metadata
- [x] Cryptographically secure secret generation and hashing
- [x] Create, list, get, update, disable, and delete
- [x] `POST /devices/:deviceId/rotate-secret` returns the new secret once
- [x] Setup wizard UI: name, one-time credentials, MQTT connection details, copyable firmware snippet, waiting state

## Acceptance criteria

A project user can create a device and receives credentials exactly once. Plaintext secrets are never stored or logged.

## Notes

- `device_key` looks like `dev_` plus 20 hex characters. The MQTT username is the device key and the MQTT password is the secret.
- The secret is 32 random bytes, encoded for firmware, and stored as a bcrypt hash. Create and rotate responses are the only places it appears. Request logs do not include bodies.
- New devices start as `unknown`. Disable sets `disabled`. `online` and `offline` wait for later ingestion. The wizard polls until `lastSeenAt` is set.
- `description` is its own column because registration asks for one. The plan's device table did not list it.
- Topics follow `makerspace/v1/projects/{projectId}/devices/{deviceKey}/...`.
- The local broker still allows anonymous connections. The wizard says so, and still shows the per-device password for production ACLs.
- Viewers can list devices and cannot create, edit, rotate, disable, or delete them.
