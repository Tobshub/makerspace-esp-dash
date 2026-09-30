# Implementation phases

`plan.md` is the full specification. These documents are the working breakdown. Implement them in order. Do not expand into the non-goals in `plan.md` section 68 before the MVP loop in section 67 works.

If time is short, `plan.md` section 85 is the cut order. The phase numbers below stay the same.

| Phase | Name | Outcome |
| --- | --- | --- |
| 0 | [Repository discovery](00-repository-discovery.md) | Done. Greenfield skeleton. |
| 1 | [Infrastructure](01-infrastructure.md) | Postgres, MQTT, health, migrations. |
| 2 | [Teams and projects](02-teams-and-projects.md) | Auth, teams, projects, authorization. |
| 3 | [Device registry](03-device-registry.md) | One-time device credentials. |
| 4 | [MQTT ingestion](04-mqtt-ingestion.md) | Telemetry and state land in the database. |
| 5 | [Device simulator](05-device-simulator.md) | Demo the pipeline without hardware. |
| 6 | [Telemetry API](06-telemetry-api.md) | Latest and historical queries. |
| 7 | [Realtime updates](07-realtime-updates.md) | Dashboard updates without refresh. |
| 8 | [Dashboard UI](08-dashboard-ui.md) | Project overview a person can read. |
| 9 | [Metric definitions](09-metric-definitions.md) | Arbitrary keys become widgets. |
| 10 | [Commands and controls](10-commands-and-controls.md) | Dashboard drives a device. |
| 11 | [Debug console](11-debug-console.md) | Live device communication log. |
| 12 | [Alerts](12-alerts.md) | Threshold and offline alerts. |
| 13 | [ESP32 starter](13-esp32-starter.md) | Flashable firmware example. |
| 14 | [Hardening](14-hardening.md) | Limits, ACLs, retention, review. |

After each phase: format, lint, typecheck, test, confirm the app starts, and update docs (`plan.md` section 84).
