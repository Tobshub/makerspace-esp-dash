# Migrations

Schema changes use [Goose](https://github.com/pressly/goose).

The first migration lands in Phase 1. Later phases add tables from `plan.md` section 6: users, teams, projects, devices, telemetry, commands, controls, alerts, and events.

Run migrations with `make migrate` once the Goose command is wired.
