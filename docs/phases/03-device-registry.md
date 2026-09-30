# Phase 3 — Device registry

**Status:** not started

**Depends on:** Phase 2

**Spec:** `plan.md` sections 6.5, 8, 18, 30, 55, 71

## Goal

A project member can register a device and see its secret exactly once.

## Tasks

- [ ] Device table with `device_key`, `secret_hash`, status, and metadata
- [ ] Cryptographically secure secret generation and hashing
- [ ] Create, list, get, update, disable, and delete
- [ ] `POST /devices/:deviceId/rotate-secret` returns the new secret once
- [ ] Setup wizard UI: name, one-time credentials, MQTT connection details, copyable firmware snippet, waiting state

## Acceptance criteria

A project user can create a device and receives credentials exactly once. Plaintext secrets are never stored or logged.
