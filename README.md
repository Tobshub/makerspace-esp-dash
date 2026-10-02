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

The API subscribes to device topics and stores telemetry, state, and presence. Publish JSON to:

```text
makerspace/v1/projects/{projectId}/devices/{deviceKey}/telemetry
```

```json
{"timestamp": 1700000000000, "metrics": {"temperature": 28.4, "pump_active": true}}
```

`timestamp` is optional Unix milliseconds. Set a retained Last Will on the `status` topic with `{"status":"offline"}`, and publish `{"status":"online"}` when the device connects. Devices that stop publishing are marked offline after `DEVICE_OFFLINE_TIMEOUT_SECONDS` (default 60).

Signed-in project members can read what was stored:

```text
GET /api/v1/devices/{deviceId}/telemetry/latest
GET /api/v1/devices/{deviceId}/telemetry?metric=temperature&from=2026-10-01T00:00:00Z&to=2026-10-02T00:00:00Z
GET /api/v1/projects/{projectId}/telemetry/latest
```

History defaults to the last 24 hours, `resolution=raw`, and at most 500 points. `1m`, `5m`, `1h`, and `1d` are reserved.

The project overview and device page read those values, plus:

```text
GET /api/v1/projects/{projectId}/overview
GET /api/v1/projects/{projectId}/events
GET /api/v1/devices/{deviceId}/state
GET /api/v1/devices/{deviceId}/events
```

Open browsers also subscribe to `GET /api/v1/projects/{projectId}/stream` (Server-Sent Events). The page sends the session token as `access_token` because `EventSource` cannot set a bearer header. Telemetry, presence, state, and command acknowledgements update the open project without a manual refresh. The API process emits those events. A separate worker can still mark devices offline, and the page refetches about every 30 seconds.

## Simulator

The device simulator stands in for an ESP32. It connects as the device, publishes drifting greenhouse telemetry, and answers `set_pump`.

```bash
cd tools/device-simulator
go run . \
  --device-key "dev_..." \
  --secret "shown-once" \
  --project-id "<project-uuid>"
```

The process prints the device key and broker URL. It does not print the secret. Stop it with Ctrl-C. It publishes `{"status":"offline"}` before it disconnects.

While it is running, turn the pump on:

```bash
mosquitto_pub -h localhost -t "makerspace/v1/projects/<project-uuid>/devices/dev_.../commands" \
  -q 1 \
  -m '{"id":"cmd_1","command":"set_pump","payload":{"enabled":true}}'
```

The device page shows the simulator as online, with firmware `sim-1.0.0`. Use `--interval` to change the 5 second telemetry period.

## Physical ESP32

The PlatformIO sketch in `examples/esp32-basic` is a skeleton. A device can still publish the telemetry JSON above with the credentials from the setup wizard. The wizard shows the presence topic and a Last Will payload. Phase 13 fills in Wi-Fi, reconnect, and commands.

## Tests

```bash
make test
make lint
```
