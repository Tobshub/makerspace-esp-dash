# Makerspace IoT Dashboard

Reusable dashboard for Makerspace teams building products on ESP32 devices. A new team connects a different product by registering a device and defining metrics. The backend does not hardcode temperature, humidity, or any other field.

The full specification is [`plan.md`](plan.md). Work is split into [phases](docs/phases/README.md).

## Architecture

```text
React client  -- HTTP / SSE -->  Go API (Gin)
                                  |        |
                             PostgreSQL   MQTT broker
                                            |
                                          ESP32
```

Telemetry, device state, commands, acknowledgements, presence, and configuration stay separate. Metric keys are arbitrary scalars.

## Prerequisites

- Go 1.25+
- Node.js 22+
- Docker

## Environment

```bash
cp .env.example .env
```

Variables are listed in `.env.example`. Do not commit `.env` or device secrets.

## Run locally

```bash
make infra
make api
make web
```

- API: http://localhost:8080/health
- UI: http://localhost:5173

`make infra` starts PostgreSQL, Mosquitto, and Redis. Redis is unused until a later phase needs it.

## Database migrations

```bash
make migrate
```

Goose applies the SQL files in `backend/migrations`. Run this after `make infra` and before using the API.

`GET /health` returns 200 and reports whether PostgreSQL and MQTT are reachable. `GET /ready` returns 503 until both are connected.

## Sign in

Open http://localhost:5173/login and create an account. The API uses a bearer token stored in the browser. A user can create a team and project, and cannot open another team's project.

```bash
make seed
```

creates a demo team, project, and device. Sign in as `demo@makerspace.local` with password `demo-password`. The device secret is printed once, on the first seed, and is not stored in plaintext.

## MQTT broker

Local Mosquitto listens on port 1883 and allows anonymous connections. See `infra/mosquitto/README.md`. Production must use TLS and per-device topic authorization. Do not put a broker admin password in firmware.

## Simulator

```bash
cd tools/device-simulator
go run . --help
```

MQTT publishing is Phase 5. Until then the command only checks flags.

## Physical ESP32

The PlatformIO sketch in `examples/esp32-basic` is a skeleton. Phase 13 fills in Wi-Fi, MQTT, telemetry, and commands.

## Tests

```bash
make test
make lint
```
