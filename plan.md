# Makerspace IoT Dashboard — Implementation Plan

Working phases: [docs/phases](docs/phases/README.md). This file remains the full specification.

## 1. Project Goal

Build a reusable IoT dashboard platform for Makerspace teams building products around ESP32 devices.

The platform should allow a team to:

1. Create a project.
2. Register one or more ESP32 devices.
3. Generate secure device credentials.
4. Connect ESP32 devices over MQTT.
5. Send arbitrary telemetry from devices.
6. View current and historical telemetry in a web dashboard.
7. Define reusable metrics and dashboard widgets without backend changes.
8. Send commands from the dashboard to devices.
9. Track device online/offline state.
10. Debug device communication from a live events/logs interface.
11. Add configurable alerts for telemetry thresholds and offline devices.

The primary design principle is:

> A new Makerspace team should be able to connect a different IoT product without requiring backend code changes.

Examples of projects the platform should support:

- Smart greenhouse
- Smart bin
- Energy monitor
- Weather station
- Access-control system
- Smart irrigation system
- Environmental monitor
- Robotics project
- Smart lighting
- Water-level monitor

---

# 2. Recommended Technology Stack

## Backend

- Go
- Gin
- PostgreSQL
- MQTT client library for Go
- Redis optional for:
  - device presence
  - command acknowledgements
  - pub/sub fan-out
  - rate limiting
- WebSockets or Server-Sent Events for live dashboard updates
- SQL migrations using Goose, Atlas, or another existing migration tool in the repository

## Frontend

- React
- React Router
- TypeScript
- TanStack Query
- A reusable component library already used by the project, if one exists
- Recharts or another lightweight charting library
- Native WebSocket/EventSource client for real-time updates

## IoT Transport

Use MQTT for ESP32 ↔ server messaging.

Recommended local broker:

- Eclipse Mosquitto

Possible production alternatives:

- EMQX
- HiveMQ
- Managed MQTT provider

Do not couple the application code tightly to one broker.

## Database

PostgreSQL.

The first version may store telemetry directly in PostgreSQL.

Do not introduce a dedicated time-series database until actual usage proves PostgreSQL insufficient.

---

# 3. Development Principles

The implementation agent must follow these rules.

## 3.1 Inspect Before Changing

Before implementing anything:

- inspect the repository
- identify the current frontend and backend structure
- identify existing authentication
- identify database tooling
- identify linting/testing conventions
- identify Docker configuration
- identify environment-variable conventions
- reuse existing components and architecture where sensible

Do not rewrite existing working systems unless necessary.

## 3.2 Build Incrementally

Implement the system in vertical slices.

A vertical slice means:

ESP32 message → MQTT → backend → database → API/live stream → frontend

Do not build all frontend pages before device ingestion works.

## 3.3 Keep IoT Data Generic

Never hardcode fields such as:

- temperature
- humidity
- waterLevel
- motion
- voltage

Telemetry must support arbitrary project-specific fields.

## 3.4 Separate Device Concepts

Treat these as distinct concepts:

- telemetry
- device state
- commands
- command acknowledgements
- device presence
- configuration

Do not put everything into one generic message type.

---

# 4. High-Level Architecture

```text
                    ┌──────────────────────────┐
                    │       React Client       │
                    │                          │
                    │ Dashboard                │
                    │ Devices                  │
                    │ Metrics                  │
                    │ Controls                 │
                    │ Alerts                   │
                    │ Live Events              │
                    └────────────┬─────────────┘
                                 │
                          HTTP / WebSocket
                                 │
                                 ▼
                    ┌──────────────────────────┐
                    │        Go API            │
                    │        Gin               │
                    │                          │
                    │ REST API                 │
                    │ Device Registry          │
                    │ Telemetry Service        │
                    │ Commands                 │
                    │ Alert Engine             │
                    │ Realtime Gateway         │
                    └───────┬─────────┬────────┘
                            │         │
                      SQL   │         │ MQTT
                            │         │
                            ▼         ▼
                    ┌───────────┐  ┌───────────┐
                    │PostgreSQL │  │MQTT Broker│
                    └───────────┘  └─────┬─────┘
                                        │
                                        │ Wi-Fi
                                        ▼
                                  ┌───────────┐
                                  │   ESP32   │
                                  │ Sensors / │
                                  │ Actuators │
                                  └───────────┘
```

---

# 5. Core Domain Model

The expected hierarchy is:

```text
User
  └── Team
       └── Project
            ├── Devices
            ├── Metrics
            ├── Dashboard Widgets
            ├── Controls
            ├── Alerts
            └── Events
```

---

# 6. Database Design

Adapt naming to existing project conventions.

Use UUIDs unless the repository already uses another ID strategy.

## 6.1 users

Use the existing users/authentication table if present.

Minimum assumed fields:

```text
id
email
name
created_at
updated_at
```

---

## 6.2 teams

```text
id
name
slug
created_at
updated_at
```

---

## 6.3 team_members

```text
id
team_id
user_id
role
created_at
```

Recommended roles:

```text
owner
admin
member
viewer
```

Add unique constraint:

```text
(team_id, user_id)
```

---

## 6.4 projects

```text
id
team_id
name
slug
description
created_at
updated_at
```

Unique constraint:

```text
(team_id, slug)
```

---

## 6.5 devices

```text
id
project_id
name
device_key
secret_hash
status
last_seen_at
firmware_version
metadata JSONB
created_at
updated_at
```

`device_key` is a public identifier used in MQTT topics.

Example:

```text
esp32-01HF8K3M...
```

Never store plaintext device secrets after initial creation.

Possible status values:

```text
online
offline
unknown
disabled
```

Indexes:

```text
project_id
device_key UNIQUE
last_seen_at
```

---

## 6.6 metric_definitions

Represents the meaning and display configuration of a telemetry field.

```text
id
project_id
key
name
description
data_type
unit
display_type
min_value
max_value
metadata JSONB
created_at
updated_at
```

Example:

```json
{
  "key": "temperature",
  "name": "Temperature",
  "data_type": "number",
  "unit": "°C",
  "display_type": "line"
}
```

Supported `data_type` values initially:

```text
number
boolean
string
```

Future:

```text
location
enum
json
```

Supported `display_type` values initially:

```text
number
line
gauge
boolean
status
text
```

Unique constraint:

```text
(project_id, key)
```

---

## 6.7 telemetry

Use one row per reported metric value.

```text
id
project_id
device_id
metric_key
numeric_value NULL
boolean_value NULL
string_value NULL
recorded_at
received_at
```

Only one typed value column should be populated for each row.

Indexes:

```text
(device_id, metric_key, recorded_at DESC)
(project_id, recorded_at DESC)
(metric_key, recorded_at DESC)
```

For the MVP, this is easier to query and graph than storing every packet only as opaque JSON.

Optionally also store the raw packet in an event table.

---

## 6.8 device_state

Stores the latest reported state.

```text
id
device_id
state JSONB
updated_at
```

One record per device.

---

## 6.9 device_commands

```text
id
project_id
device_id
command
payload JSONB
status
requested_by
requested_at
published_at
acknowledged_at
failed_at
correlation_id
error_message
```

Suggested status values:

```text
pending
published
acknowledged
failed
timed_out
```

Index:

```text
(device_id, requested_at DESC)
correlation_id UNIQUE
```

---

## 6.10 control_definitions

Defines dashboard controls.

```text
id
project_id
name
key
control_type
command
configuration JSONB
created_at
updated_at
```

Possible `control_type`:

```text
button
toggle
slider
select
number
```

Example configuration:

```json
{
  "min": 0,
  "max": 100,
  "step": 5
}
```

---

## 6.11 dashboard_widgets

```text
id
project_id
metric_definition_id NULL
control_definition_id NULL
widget_type
title
position_x
position_y
width
height
configuration JSONB
created_at
updated_at
```

Initial dashboard layout can be fixed.

Do not block MVP delivery on drag-and-drop layout editing.

---

## 6.12 alert_rules

```text
id
project_id
device_id NULL
metric_key NULL
name
rule_type
operator
threshold_value
duration_seconds
enabled
configuration JSONB
created_at
updated_at
```

Examples:

```text
metric_threshold
device_offline
```

Operators:

```text
>
>=
<
<=
==
!=
```

---

## 6.13 alert_events

```text
id
alert_rule_id
device_id
status
message
triggered_at
resolved_at
metadata JSONB
```

---

## 6.14 device_events

Useful for debugging.

```text
id
project_id
device_id
event_type
topic
payload JSONB
created_at
```

Examples of `event_type`:

```text
connected
disconnected
telemetry
state
command
command_ack
error
```

Apply retention later if event volume becomes large.

---

# 7. MQTT Topic Design

Use a predictable topic hierarchy.

Recommended:

```text
makerspace/v1/projects/{projectId}/devices/{deviceKey}/telemetry
makerspace/v1/projects/{projectId}/devices/{deviceKey}/state
makerspace/v1/projects/{projectId}/devices/{deviceKey}/commands
makerspace/v1/projects/{projectId}/devices/{deviceKey}/commands/ack
makerspace/v1/projects/{projectId}/devices/{deviceKey}/events
```

The ESP32 publishes to:

```text
telemetry
state
commands/ack
events
```

The ESP32 subscribes to:

```text
commands
```

---

# 8. MQTT Authentication

Every device receives:

```text
device_key
device_secret
```

The secret must only be displayed once after device registration.

Store only a secure hash server-side.

Preferred MQTT credentials:

```text
username = device_key
password = device_secret
```

Broker authorization should ensure a device can only access topics belonging to itself.

At minimum, document this requirement even if the local MVP broker initially has simpler credentials.

Never embed a system-wide MQTT administrator password into ESP32 firmware examples.

---

# 9. MQTT Telemetry Contract

Example packet:

```json
{
  "timestamp": 1789844400000,
  "metrics": {
    "temperature": 28.4,
    "humidity": 71,
    "pump_active": true,
    "mode": "automatic"
  }
}
```

Rules:

- `timestamp` is optional.
- If omitted, server uses receive time.
- `metrics` must be an object.
- Only scalar values are supported initially:
  - number
  - boolean
  - string
- Reject nested objects in `metrics` for the MVP.
- Reject excessively large payloads.
- Validate field-name length.
- Apply message-rate limits.

The backend should accept metrics even when no `metric_definition` exists yet.

Unknown metrics should still be stored.

Optionally create them automatically as unconfigured metrics.

---

# 10. Device State Contract

Topic:

```text
.../state
```

Example:

```json
{
  "firmwareVersion": "1.0.3",
  "wifiRssi": -61,
  "mode": "automatic",
  "relay": true
}
```

Store the latest state as JSONB.

Do not treat state as telemetry history unless explicitly configured.

---

# 11. Command Contract

Backend → ESP32:

```json
{
  "id": "cmd_123",
  "command": "set_fan",
  "payload": {
    "enabled": true
  },
  "timestamp": 1789844400000
}
```

ESP32 acknowledgement:

```json
{
  "id": "cmd_123",
  "success": true,
  "state": {
    "fan": true
  }
}
```

Failure acknowledgement:

```json
{
  "id": "cmd_123",
  "success": false,
  "error": "invalid fan mode"
}
```

The command ID/correlation ID is mandatory.

---

# 12. Device Presence

Implement device online/offline status.

Preferred method:

MQTT Last Will and Testament.

When connecting, the device configures an LWT message indicating offline state.

On successful connection it publishes online state.

Additionally, the backend updates `last_seen_at` whenever any valid device message arrives.

A background job should mark devices offline when:

```text
current_time - last_seen_at > DEVICE_OFFLINE_TIMEOUT
```

Default MVP timeout:

```text
60 seconds
```

Make this configurable.

---

# 13. Backend Services

Create clear service boundaries.

Suggested modules:

```text
auth
teams
projects
devices
mqtt
telemetry
metrics
commands
controls
alerts
events
realtime
```

Avoid putting MQTT message parsing directly inside route handlers.

Suggested flow:

```text
MQTT subscriber
    ↓
parser
    ↓
validator
    ↓
service
    ↓
repository/database
    ↓
realtime event broadcaster
```

---

# 14. Suggested Backend Directory Structure

Adapt this to the current repository.

```text
backend/
├── cmd/
│   ├── api/
│   │   └── main.go
│   └── worker/
│       └── main.go
│
├── internal/
│   ├── config/
│   ├── database/
│   ├── auth/
│   ├── teams/
│   ├── projects/
│   ├── devices/
│   ├── telemetry/
│   ├── metrics/
│   ├── commands/
│   ├── controls/
│   ├── alerts/
│   ├── mqtt/
│   ├── realtime/
│   └── events/
│
├── migrations/
├── tests/
├── Dockerfile
└── go.mod
```

If the repository already follows another layout, keep the existing convention.

---

# 15. HTTP API

Prefix:

```text
/api/v1
```

Use the repository's existing response-envelope conventions if present.

---

# 16. Team APIs

```text
GET    /teams
POST   /teams
GET    /teams/:teamId
PATCH  /teams/:teamId
DELETE /teams/:teamId
```

Membership:

```text
GET    /teams/:teamId/members
POST   /teams/:teamId/members
PATCH  /teams/:teamId/members/:memberId
DELETE /teams/:teamId/members/:memberId
```

---

# 17. Project APIs

```text
GET    /teams/:teamId/projects
POST   /teams/:teamId/projects

GET    /projects/:projectId
PATCH  /projects/:projectId
DELETE /projects/:projectId
```

---

# 18. Device APIs

```text
GET    /projects/:projectId/devices
POST   /projects/:projectId/devices

GET    /devices/:deviceId
PATCH  /devices/:deviceId
DELETE /devices/:deviceId
```

Device creation response:

```json
{
  "device": {
    "id": "...",
    "deviceKey": "...",
    "name": "Greenhouse ESP32"
  },
  "credentials": {
    "username": "...",
    "secret": "..."
  }
}
```

The plaintext secret must never be retrievable again.

Add:

```text
POST /devices/:deviceId/rotate-secret
```

Return the new secret once.

---

# 19. Telemetry APIs

Latest values:

```text
GET /devices/:deviceId/telemetry/latest
```

Historical:

```text
GET /devices/:deviceId/telemetry
```

Query parameters:

```text
metric
from
to
limit
resolution
```

Example:

```text
GET /devices/123/telemetry?metric=temperature&from=...&to=...
```

Project overview:

```text
GET /projects/:projectId/telemetry/latest
```

---

# 20. Metric APIs

```text
GET    /projects/:projectId/metrics
POST   /projects/:projectId/metrics
GET    /metrics/:metricId
PATCH  /metrics/:metricId
DELETE /metrics/:metricId
```

Also consider:

```text
GET /projects/:projectId/metrics/discovered
```

This returns telemetry keys received from devices that do not yet have configured definitions.

---

# 21. Command APIs

```text
POST /devices/:deviceId/commands
GET  /devices/:deviceId/commands
GET  /commands/:commandId
```

Request:

```json
{
  "command": "set_fan",
  "payload": {
    "enabled": true
  }
}
```

Response:

```json
{
  "id": "...",
  "status": "pending"
}
```

Do not block the HTTP request waiting indefinitely for device acknowledgement.

---

# 22. Control APIs

```text
GET    /projects/:projectId/controls
POST   /projects/:projectId/controls
PATCH  /controls/:controlId
DELETE /controls/:controlId
```

---

# 23. Alert APIs

```text
GET    /projects/:projectId/alerts
POST   /projects/:projectId/alerts
PATCH  /alerts/:alertId
DELETE /alerts/:alertId
```

Events:

```text
GET /projects/:projectId/alert-events
```

---

# 24. Debug/Event APIs

```text
GET /projects/:projectId/events
GET /devices/:deviceId/events
```

Support filters:

```text
event_type
device_id
from
to
```

---

# 25. Realtime Browser Updates

Use WebSocket or Server-Sent Events.

For the MVP, Server-Sent Events is acceptable if the browser only needs server → client updates.

Suggested stream:

```text
GET /api/v1/projects/:projectId/stream
```

Events:

```text
device.online
device.offline
telemetry.received
state.updated
command.updated
alert.triggered
alert.resolved
```

Example event:

```json
{
  "type": "telemetry.received",
  "deviceId": "...",
  "timestamp": "...",
  "data": {
    "temperature": 28.4
  }
}
```

The frontend should update cached data when realtime events arrive.

---

# 26. Frontend Routes

Suggested structure:

```text
/
 /login

/dashboard

/teams/:teamId
/teams/:teamId/projects

/projects/:projectId
/projects/:projectId/overview
/projects/:projectId/devices
/projects/:projectId/devices/:deviceId
/projects/:projectId/metrics
/projects/:projectId/controls
/projects/:projectId/alerts
/projects/:projectId/events
/projects/:projectId/settings
```

---

# 27. Main Dashboard Layout

Project overview should show:

```text
Project name

Devices
- total
- online
- offline

Telemetry
- messages today
- last message received

Alerts
- active alerts

Metric cards

Historical charts

Recent events

Device table
```

Suggested layout:

```text
┌────────────────────────────────────────────────────┐
│ Project Alpha                         Team Delta    │
├────────────────────────────────────────────────────┤
│ 4 Devices │ 3 Online │ 1 Offline │ 24.8k Messages │
├───────────────────────┬────────────────────────────┤
│ Temperature           │ Humidity                   │
│ 28.4 °C               │ 71%                        │
│ small sparkline       │ small sparkline            │
├───────────────────────┴────────────────────────────┤
│ Temperature — Last 24 Hours                        │
│                                                    │
│                    chart                           │
├────────────────────────────────────────────────────┤
│ Devices                                            │
│ ESP32-A       Online                 2 seconds ago  │
│ ESP32-B       Online                 8 seconds ago  │
│ ESP32-C       Offline                1 hour ago     │
└────────────────────────────────────────────────────┘
```

---

# 28. Device List Page

Columns:

```text
Name
Device Key
Status
Last Seen
Firmware
Actions
```

Actions:

```text
Open
Edit
Rotate secret
Disable
Delete
```

---

# 29. Device Detail Page

Sections:

## Summary

```text
Status
Last seen
Firmware
Wi-Fi RSSI
Created
```

## Latest Telemetry

Automatically show recently reported keys.

## Charts

Allow selecting:

```text
metric
time range
```

## State

Pretty-print latest state.

## Commands

Show command history and status.

## Events

Recent device-specific activity.

---

# 30. Device Registration Flow

The flow should be optimized for hackathon/Makerspace users.

Step 1:

```text
Add Device
```

Fields:

```text
Device name
Optional description
```

Step 2:

Create credentials.

Step 3:

Show:

```text
Device Key
Device Secret
MQTT Host
MQTT Port
MQTT Username
MQTT Password
Telemetry Topic
Command Topic
```

Step 4:

Provide ready-to-copy ESP32 example code.

Step 5:

Show:

```text
Waiting for device...
```

Automatically mark successful connection when telemetry/state arrives.

---

# 31. ESP32 Starter Library / Example

Provide a sample Arduino/PlatformIO project.

Suggested path:

```text
examples/
└── esp32-basic/
    ├── platformio.ini
    ├── src/
    │   └── main.cpp
    └── README.md
```

The example should:

1. connect to Wi-Fi
2. connect to MQTT
3. authenticate
4. configure Last Will
5. publish online status
6. publish sample telemetry periodically
7. subscribe to command topic
8. execute demo commands
9. send command acknowledgements
10. reconnect after network failure

Pseudo configuration:

```cpp
#define WIFI_SSID "..."
#define WIFI_PASSWORD "..."

#define MQTT_HOST "..."
#define MQTT_PORT 1883

#define DEVICE_KEY "..."
#define DEVICE_SECRET "..."
#define PROJECT_ID "..."
```

Sample telemetry:

```cpp
{
  "timestamp": 0,
  "metrics": {
    "temperature": 28.4,
    "humidity": 70
  }
}
```

Example command handling:

```text
set_led
```

Payload:

```json
{
  "enabled": true
}
```

Use this to prove two-way communication.

---

# 32. Metric Configuration UI

Metrics page should show discovered telemetry fields.

Example:

```text
temperature      Number      Configured
humidity         Number      Configured
pump_active      Boolean     Unconfigured
soil_moisture    Number      Unconfigured
```

Clicking a field should allow:

```text
Display name
Unit
Display type
Min
Max
Description
```

Do not require teams to define metrics before telemetry starts arriving.

---

# 33. Widget Types

Implement these first:

## Number Card

Example:

```text
Temperature
28.4 °C
```

## Line Chart

Historical numeric telemetry.

## Gauge

Numeric telemetry with configured minimum/maximum.

## Boolean Status

Example:

```text
Pump
ON
```

## Text

String telemetry.

More complex widgets can come later.

---

# 34. Dashboard Configuration

MVP:

Automatically generate a reasonable dashboard from configured metrics.

Example rules:

```text
number + display_type=number → stat card
number + display_type=line → line chart
number + display_type=gauge → gauge
boolean → status card
string → text card
```

Phase 2:

Allow adding/removing/reordering widgets.

Do not implement a full drag-and-drop page builder in the first milestone unless required.

---

# 35. Controls

A control definition maps UI input to a command.

Example toggle:

```json
{
  "name": "Fan",
  "controlType": "toggle",
  "command": "set_fan",
  "configuration": {
    "onPayload": {
      "enabled": true
    },
    "offPayload": {
      "enabled": false
    }
  }
}
```

Slider example:

```json
{
  "name": "Motor Speed",
  "controlType": "slider",
  "command": "set_motor_speed",
  "configuration": {
    "min": 0,
    "max": 100,
    "step": 5,
    "payloadKey": "speed"
  }
}
```

Button example:

```json
{
  "name": "Open Gate",
  "controlType": "button",
  "command": "open_gate",
  "configuration": {}
}
```

The UI must display command status:

```text
Sending...
Sent
Acknowledged
Failed
Timed out
```

---

# 36. Alert Engine

MVP alert types:

## Metric threshold

Example:

```text
temperature > 40
```

## Device offline

Example:

```text
device offline for > 5 minutes
```

Threshold alerts should support an optional duration to avoid noisy transient events.

Example:

```text
temperature > 40 for 30 seconds
```

For MVP, dashboard notifications are sufficient.

Future channels:

```text
email
SMS
push
webhook
```

---

# 37. Alert Evaluation

For each incoming telemetry value:

1. identify enabled rules for the metric
2. evaluate condition
3. apply duration/debounce logic
4. trigger alert if required
5. create `alert_event`
6. push realtime event to browser

Avoid triggering duplicate active alerts repeatedly.

Alert lifecycle:

```text
inactive
triggered
resolved
```

---

# 38. Debug Console

Create a page:

```text
/projects/:projectId/events
```

It should show a live stream.

Example:

```text
20:14:31 TELEMETRY esp32-greenhouse
temperature=28.4 humidity=71

20:14:29 COMMAND esp32-greenhouse
set_fan {"enabled":true}

20:14:29 ACK esp32-greenhouse
set_fan success

20:13:54 CONNECTED esp32-greenhouse
```

Allow filtering by:

```text
device
event type
```

Provide expandable raw JSON.

This feature is high priority because it helps teams debug prototypes.

---

# 39. Security Requirements

Implement from the start.

## Application authorization

Every project API request must verify that the current user belongs to the owning team.

Never trust IDs sent by the frontend without authorization checks.

## Device credentials

- generate cryptographically secure secrets
- display plaintext only once
- store only hashes
- allow rotation
- support disabling devices

## MQTT

Production setup must use:

```text
TLS
```

Recommended port:

```text
8883
```

Local development may use plain MQTT.

## Payload validation

Reject:

- invalid JSON
- oversized payloads
- unsupported nested telemetry
- excessive metric counts
- excessively long keys
- NaN/infinite numeric values where relevant

## Rate limits

Implement sane limits per device.

Example default:

```text
10 telemetry messages / second / device
```

Make configurable.

---

# 40. Telemetry Retention

Do not over-engineer initially.

Initial approach:

Store raw telemetry.

Add configuration constants:

```text
TELEMETRY_RETENTION_DAYS
DEVICE_EVENT_RETENTION_DAYS
```

Later add cleanup jobs.

Potential future optimization:

- hourly aggregates
- daily aggregates
- TimescaleDB
- ClickHouse
- dedicated time-series database

Do not introduce these in MVP unless load requires them.

---

# 41. Historical Query Strategy

For small ranges:

return raw points.

For large ranges:

support server-side aggregation.

Possible `resolution` values:

```text
raw
1m
5m
1h
1d
```

Aggregate numeric metrics with:

```text
avg
min
max
count
```

The MVP can initially support:

```text
raw
```

but design the endpoint so resolution can be added without breaking the API.

---

# 42. Local Development Environment

Add Docker Compose if the repository does not already provide equivalent infrastructure.

Services:

```text
postgres
mosquitto
redis optional
backend
frontend optional
```

Example:

```text
docker compose up
```

should make the development infrastructure easy to start.

Required environment variables may include:

```text
DATABASE_URL

MQTT_BROKER_URL
MQTT_USERNAME
MQTT_PASSWORD

DEVICE_OFFLINE_TIMEOUT_SECONDS

APP_URL
API_URL
```

Do not commit real secrets.

Provide:

```text
.env.example
```

---

# 43. MQTT Broker Local Configuration

Provide a development Mosquitto configuration.

Suggested directory:

```text
infra/
└── mosquitto/
    ├── mosquitto.conf
    └── passwd
```

Development configuration should be simple.

Production documentation must state:

- anonymous access disabled
- TLS enabled
- per-device topic authorization enabled
- secure passwords/secrets required

---

# 44. Backend Logging

Use structured logs.

Include fields such as:

```text
project_id
device_id
device_key
mqtt_topic
command_id
request_id
user_id
```

Never log device plaintext secrets.

---

# 45. Observability

MVP:

Provide:

```text
GET /health
GET /ready
```

Health information should cover:

```text
database
MQTT connectivity
```

Future:

Prometheus metrics such as:

```text
iot_mqtt_messages_total
iot_telemetry_points_total
iot_devices_online
iot_commands_total
iot_command_failures_total
iot_alerts_triggered_total
```

---

# 46. Testing Strategy

## Backend unit tests

Cover:

- telemetry parsing
- metric type detection
- MQTT topic parsing
- project authorization
- command serialization
- acknowledgement processing
- alert threshold evaluation
- offline detection

## Backend integration tests

Use a test database.

Test:

```text
create project
create device
store telemetry
fetch telemetry
create command
ack command
```

## MQTT integration tests

Publish MQTT messages using a test client and verify:

```text
message received
database updated
realtime event emitted
```

## Frontend tests

At minimum:

```text
critical pages render
device creation flow
telemetry chart transformation
control command flow
```

## End-to-end test

One complete automated scenario:

1. create project
2. register device
3. connect simulated MQTT client
4. publish telemetry
5. verify API returns telemetry
6. issue command
7. simulated device receives command
8. publish acknowledgement
9. verify command becomes acknowledged

This is the most important integration test.

---

# 47. Device Simulator

Build a small developer tool that behaves like an ESP32.

Suggested:

```text
tools/
└── device-simulator/
```

It should:

- connect to MQTT
- authenticate as a device
- publish telemetry on an interval
- receive commands
- acknowledge commands
- simulate disconnect/reconnect

CLI example:

```bash
go run ./tools/device-simulator \
  --device-key "... " \
  --secret "..."
```

or equivalent script using the existing project language/tooling.

This allows frontend/backend development without physical hardware.

---

# 48. Sample Device Simulation

Generate telemetry like:

```json
{
  "metrics": {
    "temperature": 28.1,
    "humidity": 70,
    "pump_active": false
  }
}
```

Temperature may drift gradually rather than be totally random.

Supported command:

```text
set_pump
```

This creates a complete demo.

---

# 49. UX Requirements

The platform is intended for students and Makerspace teams.

Prioritize:

- obvious setup
- visible connection status
- useful error messages
- copyable credentials
- sample firmware
- live debugging
- minimal configuration before first data appears

Avoid exposing unnecessary MQTT terminology on every page.

The device setup wizard can show technical details only where needed.

---

# 50. Empty States

Every major page should include useful empty states.

Example Devices page:

```text
No devices yet.

Connect an ESP32 to start sending telemetry.

[Add Device]
```

Metrics page:

```text
No telemetry metrics discovered yet.

Once your ESP32 sends telemetry, metrics will appear here automatically.
```

Events page:

```text
Waiting for device activity...
```

---

# 51. Error States

Handle explicitly:

```text
MQTT disconnected
device offline
device authentication failure
invalid telemetry
API unavailable
historical query failure
command timeout
```

Frontend must not fail silently.

---

# 52. Phase 0 — Repository Discovery

Before changing code:

- [ ] inspect repository tree
- [ ] identify existing backend
- [ ] identify existing frontend
- [ ] identify current auth implementation
- [ ] identify database and migration tooling
- [ ] identify existing team/project concepts
- [ ] identify component library
- [ ] identify current API patterns
- [ ] identify Docker setup
- [ ] run existing tests
- [ ] run existing lint/typecheck
- [ ] document anything that conflicts with this plan

Output a short implementation note before major changes:

```text
Existing architecture:
...
Changes required:
...
Plan deviations:
...
```

Do not stop solely because structure differs from this plan.

Adapt intelligently.

---

# 53. Phase 1 — Infrastructure

Goal:

Get local infrastructure running.

Tasks:

- [ ] PostgreSQL available
- [ ] MQTT broker available
- [ ] backend can connect to PostgreSQL
- [ ] backend can connect to MQTT
- [ ] `/health` works
- [ ] `.env.example` updated
- [ ] Docker Compose updated if appropriate

Acceptance criteria:

```text
Backend starts successfully.
Database migrations run successfully.
Backend establishes MQTT connection.
Health endpoint reports required dependencies.
```

---

# 54. Phase 2 — Teams and Projects

Skip or adapt if existing implementations already satisfy these requirements.

Tasks:

- [ ] team model
- [ ] memberships
- [ ] projects
- [ ] authorization middleware
- [ ] team/project CRUD APIs
- [ ] project selector UI

Acceptance criteria:

A logged-in user can create/open a project and cannot access projects belonging to unauthorized teams.

---

# 55. Phase 3 — Device Registry

Tasks:

- [ ] device database model
- [ ] secure secret generation
- [ ] secret hashing
- [ ] create device endpoint
- [ ] list devices
- [ ] device detail
- [ ] rotate secret
- [ ] disable/delete device
- [ ] setup wizard UI

Acceptance criteria:

A project user can create a device and receives credentials exactly once.

---

# 56. Phase 4 — MQTT Ingestion

Tasks:

- [ ] subscribe to required MQTT topics
- [ ] parse project/device from topic
- [ ] authenticate/resolve device
- [ ] parse telemetry packet
- [ ] validate payload
- [ ] persist telemetry
- [ ] update `last_seen_at`
- [ ] store debug event
- [ ] update device state
- [ ] handle connection/presence events

Acceptance criteria:

Publishing valid MQTT telemetry results in database rows and updated device status.

---

# 57. Phase 5 — Device Simulator

Tasks:

- [ ] simulator connects using device credentials
- [ ] publishes telemetry
- [ ] responds to command topic
- [ ] sends acknowledgement
- [ ] reconnects on failure

Acceptance criteria:

A developer can demo the complete device pipeline without an ESP32.

---

# 58. Phase 6 — Telemetry API

Tasks:

- [ ] latest-value endpoint
- [ ] historical endpoint
- [ ] project telemetry overview
- [ ] query validation
- [ ] indexes
- [ ] tests

Acceptance criteria:

Frontend can retrieve the latest values and historical series for any numeric metric.

---

# 59. Phase 7 — Realtime Updates

Tasks:

- [ ] SSE or WebSocket endpoint
- [ ] per-project subscriptions
- [ ] publish telemetry events
- [ ] publish presence events
- [ ] publish command updates
- [ ] reconnect handling in frontend

Acceptance criteria:

Dashboard values update without manual refresh when telemetry arrives.

---

# 60. Phase 8 — Dashboard UI

Tasks:

- [ ] overview page
- [ ] device summary cards
- [ ] online/offline counts
- [ ] latest metric cards
- [ ] historical charts
- [ ] recent events
- [ ] loading states
- [ ] empty states
- [ ] error states

Acceptance criteria:

A user can open a project and see current IoT status without navigating through raw API data.

---

# 61. Phase 9 — Metric Definitions

Tasks:

- [x] discover unknown telemetry keys
- [x] metrics page
- [x] configure metric names
- [x] units
- [x] data/display types
- [x] min/max values
- [x] automatically render configured widgets

Acceptance criteria:

A team can turn an arbitrary telemetry key into a useful dashboard visualization without backend changes.

---

# 62. Phase 10 — Commands and Controls

Tasks:

- [x] command persistence
- [x] MQTT command publishing
- [x] acknowledgement handling
- [x] timeout handling
- [x] controls CRUD
- [x] button control
- [x] toggle control
- [x] slider control
- [x] UI command status

Acceptance criteria:

A user can control a simulated device or ESP32 from the dashboard and see whether the command was acknowledged.

---

# 63. Phase 11 — Debug Console

Tasks:

- [x] store device events
- [x] project events endpoint
- [x] realtime event stream
- [x] event filtering
- [x] expandable JSON payloads
- [x] device-specific logs

Acceptance criteria:

A Makerspace team can diagnose whether their ESP32 is connecting, sending data, receiving commands, and returning acknowledgements.

---

# 64. Phase 12 — Alerts

Tasks:

- [x] alert rule model
- [x] threshold evaluation
- [x] offline rule
- [x] alert lifecycle
- [x] active alerts UI
- [x] alert history
- [x] realtime alert notification

Acceptance criteria:

A user can configure a threshold and see an alert trigger and later resolve.

---

# 65. Phase 13 — ESP32 Starter Project

Tasks:

- [x] Arduino or PlatformIO example
- [x] Wi-Fi connection
- [x] MQTT authentication
- [x] LWT
- [x] telemetry publishing
- [x] commands
- [x] acknowledgements
- [x] reconnect behavior
- [x] README

Acceptance criteria:

A fresh ESP32 can be connected by copying credentials and flashing the starter firmware.

---

# 66. Phase 14 — Hardening

Tasks:

- [ ] MQTT TLS production instructions
- [ ] device topic ACL strategy
- [ ] payload size limits
- [ ] message-rate limits
- [ ] API rate limiting where useful
- [ ] authorization review
- [ ] query performance review
- [ ] secret/logging audit
- [ ] telemetry retention job
- [ ] command timeout worker

---

# 67. Definition of MVP

The project is MVP-complete when this workflow works:

1. User signs in.
2. User opens a team/project.
3. User creates an ESP32 device.
4. Dashboard displays generated credentials.
5. Device simulator or ESP32 connects to MQTT.
6. Device becomes online.
7. Device publishes arbitrary telemetry.
8. Telemetry appears on dashboard in near-real-time.
9. User configures at least one metric.
10. Historical graph displays the metric.
11. User creates or uses a control.
12. Dashboard publishes a command.
13. Device receives it.
14. Device sends acknowledgement.
15. Dashboard displays acknowledged status.
16. User can inspect the interaction in the event console.
17. User can create a basic alert.

Do not expand scope significantly before this complete loop works.

---

# 68. Explicit Non-Goals for MVP

Do not prioritize these until the complete MVP workflow works:

- OTA firmware update management
- advanced analytics/AI
- mobile application
- geospatial maps
- complex nested telemetry
- drag-and-drop page builder
- custom scripting
- multi-region deployment
- dedicated time-series database
- SMS notifications
- billing
- device provisioning over Bluetooth
- digital twins
- full rule-engine DSL
- custom firmware compiler

The architecture may leave room for them.

---

# 69. Future Features

After MVP:

## OTA Firmware

```text
firmware versions
device groups
staged rollout
update status
rollback
```

## Webhooks

Allow projects to forward events.

## Integrations

Examples:

```text
Slack
Discord
email
SMS
external APIs
```

## Advanced Device Groups

Control several ESP32 devices together.

## Rules Engine

Example:

```text
IF soil_moisture < 20
AND mode == automatic
THEN send set_pump(enabled=true)
```

## Derived Metrics

Example:

```text
power = voltage * current
```

## Maps

For devices reporting GPS coordinates.

## Dashboard Sharing

Read-only public/project presentation links.

## Export

CSV/JSON telemetry exports.

---

# 70. API Error Format

Use one consistent format.

Example:

```json
{
  "error": {
    "code": "DEVICE_NOT_FOUND",
    "message": "Device not found"
  }
}
```

Validation:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid request",
    "fields": {
      "name": "Name is required"
    }
  }
}
```

Reuse an existing application convention if present.

---

# 71. Naming Rules

Project slugs:

```text
greenhouse-monitor
```

Device names are human-readable:

```text
Greenhouse Controller
```

Device keys are machine identifiers:

```text
dev_01K...
```

Metric keys should be:

```text
snake_case
```

Examples:

```text
temperature
soil_moisture
pump_active
battery_voltage
```

---

# 72. Time Handling

Use UTC everywhere in backend/database.

Frontend may display local time.

Use ISO 8601 for HTTP APIs.

For MQTT telemetry timestamp, standardize on Unix milliseconds.

Server must store:

```text
recorded_at
received_at
```

This distinguishes device time from server receive time.

---

# 73. Connection Reliability

ESP32 example code must implement:

- automatic Wi-Fi reconnect
- automatic MQTT reconnect
- exponential backoff
- non-blocking reconnect loop where practical
- resubscription after reconnect
- heartbeat or periodic telemetry
- Last Will message

Avoid examples that permanently hang if the broker is unavailable.

---

# 74. MQTT QoS

Recommended initial choices:

Telemetry:

```text
QoS 0
```

Commands:

```text
QoS 1
```

Command acknowledgements:

```text
QoS 1
```

State:

```text
QoS 1
retained where appropriate
```

Do not use retained messages for arbitrary telemetry streams.

---

# 75. Command Idempotency

Commands should have unique IDs.

ESP32 should ideally remember a small recent command-ID cache.

If duplicate QoS delivery occurs:

```text
same command ID → do not perform dangerous action twice
```

At minimum, document this pattern in the sample firmware.

---

# 76. Firmware Version

ESP32 state should include:

```json
{
  "firmwareVersion": "1.0.0"
}
```

Display firmware version on device list/detail.

This helps Makerspace mentors diagnose inconsistent devices.

---

# 77. Project Settings Page

Include:

```text
Project name
Description
MQTT connection info
Default device offline timeout
Danger zone
```

Do not expose broker administrator credentials.

---

# 78. README Requirements

Root README should explain:

1. project purpose
2. architecture
3. prerequisites
4. environment variables
5. running locally
6. database migrations
7. MQTT broker
8. starting frontend/backend
9. running simulator
10. connecting a physical ESP32
11. running tests

A new developer should be able to start the stack without asking undocumented questions.

---

# 79. Developer Commands

Where appropriate, provide consistent commands such as:

```bash
make dev
make test
make lint
make migrate
make seed
make simulator
```

If the repository uses npm scripts or another task runner instead of Make, use the existing convention.

---

# 80. Seed / Demo Data

Provide optional demo data:

```text
Team: Makerspace Demo
Project: Smart Greenhouse
Device: Greenhouse ESP32
```

Metrics:

```text
temperature
humidity
soil_moisture
pump_active
```

This makes the template immediately understandable.

---

# 81. UI Navigation

Recommended sidebar:

```text
Overview

Devices
Metrics
Controls
Alerts
Events

Project Settings
```

Team/project selector should remain accessible.

---

# 82. Recommended First Complete Demo

Build toward this concrete demo:

## Smart Greenhouse

Telemetry:

```json
{
  "metrics": {
    "temperature": 29.2,
    "humidity": 68,
    "soil_moisture": 37,
    "pump_active": false
  }
}
```

Control:

```text
Pump ON/OFF
```

Alert:

```text
soil_moisture < 20
```

Dashboard:

```text
temperature card
humidity card
soil moisture gauge
temperature chart
pump status
pump control
```

This single example exercises almost every core capability while remaining simple.

---

# 83. Agent Execution Rules

The implementation agent should work autonomously and make reasonable decisions.

Do not stop for minor ambiguity.

When uncertain:

1. inspect existing code
2. follow current project conventions
3. choose the simplest maintainable implementation
4. document assumptions
5. continue

The agent should not replace working architecture merely because this document uses different names.

---

# 84. Agent Completion Routine

After every major phase:

1. run formatter
2. run lint
3. run typecheck where applicable
4. run unit tests
5. run relevant integration tests
6. verify application starts
7. update documentation
8. summarize changed files
9. record remaining issues

Do not leave obviously broken builds between phases.

---

# 85. Implementation Priority

If time is limited, prioritize in this exact order:

```text
1. Infrastructure
2. Device registry
3. MQTT telemetry ingestion
4. Device simulator
5. Latest telemetry API
6. Dashboard
7. Historical telemetry
8. Realtime updates
9. Commands
10. Debug console
11. Metric configuration
12. Controls
13. Alerts
14. ESP32 polished starter project
15. Hardening
```

Teams gain the most value from a functioning end-to-end device path.

---

# 86. Final Acceptance Checklist

## Infrastructure

- [ ] PostgreSQL works
- [ ] MQTT broker works
- [ ] migrations work
- [ ] environment docs exist

## Authentication and access

- [ ] user authentication works
- [ ] team/project authorization enforced

## Devices

- [ ] device registration works
- [ ] credentials generated securely
- [ ] credentials shown only once
- [ ] secret rotation works
- [ ] online/offline state works

## Telemetry

- [ ] ESP32/simulator publishes telemetry
- [ ] backend validates telemetry
- [ ] telemetry persisted
- [ ] arbitrary metric keys supported
- [ ] latest values queryable
- [ ] historical values queryable

## Dashboard

- [ ] device counts shown
- [ ] latest values shown
- [ ] historical charts shown
- [ ] realtime updates work
- [ ] useful empty/error states exist

## Metrics

- [x] discovered keys visible
- [x] metric configuration works
- [x] units supported
- [x] display types supported

## Commands

- [ ] command publish works
- [ ] ESP32 receives command
- [ ] acknowledgement works
- [ ] timeout works
- [ ] command history visible

## Controls

- [ ] button works
- [ ] toggle works
- [ ] slider works

## Debugging

- [ ] events stored
- [ ] realtime events visible
- [ ] filtering works
- [ ] raw JSON visible

## Alerts

- [ ] threshold rule works
- [ ] offline rule works
- [ ] duplicate active alerts prevented
- [ ] resolve flow works

## ESP32

- [ ] starter firmware exists
- [ ] Wi-Fi reconnect works
- [ ] MQTT reconnect works
- [ ] telemetry sample works
- [ ] command sample works
- [ ] acknowledgement sample works

## Quality

- [ ] tests pass
- [ ] lint passes
- [ ] documentation updated
- [ ] no plaintext device secrets logged
- [ ] no hardcoded project-specific telemetry fields

---

# 87. Expected Final Outcome

A Makerspace team should be able to receive the template and do the following with minimal platform work:

```text
1. Create project
2. Register ESP32
3. Copy credentials into firmware
4. Flash device
5. Send sensor values
6. See device online
7. See values immediately
8. Configure how each value is displayed
9. View historical graphs
10. Add dashboard controls
11. Send commands to actuators
12. Debug communication
13. Configure simple alerts
```

A team building an entirely different IoT product should be able to repeat the same process without requesting changes to the backend.

That flexibility is the primary measure of success.
