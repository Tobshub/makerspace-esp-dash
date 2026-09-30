# Phase 1 — Infrastructure

**Status:** not started

**Depends on:** Phase 0

**Spec:** `plan.md` sections 42–45, 53

## Goal

Start local infrastructure and prove the API can reach PostgreSQL and MQTT.

## Tasks

- [ ] PostgreSQL available through Docker Compose
- [ ] MQTT broker available through Docker Compose
- [ ] Backend connects to PostgreSQL
- [ ] Backend connects to MQTT
- [ ] Goose migration runner wired (`make migrate`)
- [ ] `/health` and `/ready` report database and MQTT connectivity
- [ ] `.env.example` matches the variables the process actually reads

## Acceptance criteria

The backend starts, migrations run, the MQTT connection is established, and the health endpoint reports the required dependencies.
