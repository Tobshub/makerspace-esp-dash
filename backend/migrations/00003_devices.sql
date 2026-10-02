-- +goose Up
-- description is stored as its own column because device registration asks for one.
CREATE TABLE devices (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    device_key text NOT NULL,
    secret_hash text NOT NULL,
    status text NOT NULL DEFAULT 'unknown',
    last_seen_at timestamptz,
    firmware_version text NOT NULL DEFAULT '',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT devices_device_key_unique UNIQUE (device_key),
    CONSTRAINT devices_status_check CHECK (status IN ('online', 'offline', 'unknown', 'disabled'))
);

CREATE INDEX devices_project_id_idx ON devices (project_id);
CREATE INDEX devices_last_seen_at_idx ON devices (last_seen_at);

-- +goose Down
DROP TABLE devices;
