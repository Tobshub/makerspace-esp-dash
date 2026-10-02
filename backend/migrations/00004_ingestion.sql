-- +goose Up
-- One row per scalar metric value. Unknown keys are stored; metric definitions are a later phase.
CREATE TABLE telemetry (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    device_id uuid NOT NULL REFERENCES devices (id) ON DELETE CASCADE,
    metric_key text NOT NULL,
    numeric_value double precision,
    boolean_value boolean,
    string_value text,
    recorded_at timestamptz NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT telemetry_one_value CHECK (num_nonnulls(numeric_value, boolean_value, string_value) = 1)
);

CREATE INDEX telemetry_device_metric_recorded_idx ON telemetry (device_id, metric_key, recorded_at DESC);
CREATE INDEX telemetry_project_recorded_idx ON telemetry (project_id, recorded_at DESC);
CREATE INDEX telemetry_metric_recorded_idx ON telemetry (metric_key, recorded_at DESC);

CREATE TABLE device_state (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id uuid NOT NULL REFERENCES devices (id) ON DELETE CASCADE,
    state jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT device_state_device_id_unique UNIQUE (device_id)
);

CREATE TABLE device_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    device_id uuid NOT NULL REFERENCES devices (id) ON DELETE CASCADE,
    event_type text NOT NULL,
    topic text NOT NULL DEFAULT '',
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX device_events_device_created_idx ON device_events (device_id, created_at DESC);
CREATE INDEX device_events_project_created_idx ON device_events (project_id, created_at DESC);

-- +goose Down
DROP TABLE device_events;
DROP TABLE device_state;
DROP TABLE telemetry;
