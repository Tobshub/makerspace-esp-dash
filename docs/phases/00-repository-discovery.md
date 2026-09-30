# Phase 0 — Repository discovery

**Status:** complete

**Spec:** `plan.md` section 52

## Finding

The workspace contained only `plan.md`. There was no backend, frontend, authentication, database tooling, component library, API convention, or Docker setup to reuse.

## Decisions

- Go module at `backend/`, Gin API in `backend/cmd/api`.
- React + TypeScript + Vite in `frontend/`, with React Router and TanStack Query.
- PostgreSQL 16, Eclipse Mosquitto 2, and optional Redis via Docker Compose.
- Goose for SQL migrations, starting in Phase 1.
- UUIDs, `/api/v1`, and the error envelope in `plan.md` section 70.
- Device simulator is its own small Go module so it can run without importing the API.
- Local Mosquitto allows anonymous connections. Production TLS and per-device ACLs stay in Phase 14.
- Redis is in Compose and unused until a later phase needs it.

## Plan deviations

None yet. Package directories match `plan.md` section 14. Screens are routed placeholders, not finished UI.
