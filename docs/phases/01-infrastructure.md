# Phase 1 — Infrastructure

**Status:** complete

**Depends on:** Phase 0

**Spec:** `plan.md` sections 42–45, 53

## Goal

Start local infrastructure and prove the API can reach PostgreSQL and MQTT.

## Tasks

- [x] PostgreSQL available through Docker Compose
- [x] MQTT broker available through Docker Compose
- [x] Backend connects to PostgreSQL
- [x] Backend connects to MQTT
- [x] Goose migration runner wired (`make migrate`)
- [x] `/health` and `/ready` report database and MQTT connectivity
- [x] `.env.example` matches the variables the process actually reads

## Acceptance criteria

The backend starts, migrations run, the MQTT connection is established, and the health endpoint reports the required dependencies.

## Notes

- `GET /health` stays 200 and includes `database` and `mqtt` as `ok` or `down`. `GET /ready` returns 503 when either dependency is down.
- The API loads `.env` from the working directory or a parent directory and does not override variables already set in the environment.
- `DATABASE_URL` defaults to the Compose credentials in `.env.example`.
- Redis is still unused. Telemetry rate limits are in-process.
- The MQTT client reconnects in the background and subscribes for ingestion.
