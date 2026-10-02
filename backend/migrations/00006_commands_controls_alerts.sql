-- +goose Up
CREATE TABLE device_commands (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    device_id uuid NOT NULL REFERENCES devices (id) ON DELETE CASCADE,
    command text NOT NULL,
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    status text NOT NULL,
    requested_by uuid REFERENCES users (id) ON DELETE SET NULL,
    requested_at timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz,
    acknowledged_at timestamptz,
    failed_at timestamptz,
    correlation_id text NOT NULL,
    error_message text NOT NULL DEFAULT '',
    CONSTRAINT device_commands_status_check CHECK (status IN ('pending', 'published', 'acknowledged', 'failed', 'timed_out')),
    CONSTRAINT device_commands_correlation_id_unique UNIQUE (correlation_id)
);

CREATE INDEX device_commands_device_requested_idx ON device_commands (device_id, requested_at DESC);

CREATE TABLE control_definitions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    name text NOT NULL,
    key text NOT NULL,
    control_type text NOT NULL,
    command text NOT NULL,
    configuration jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT control_definitions_project_key_unique UNIQUE (project_id, key),
    CONSTRAINT control_definitions_type_check CHECK (control_type IN ('button', 'toggle', 'slider'))
);

CREATE INDEX control_definitions_project_idx ON control_definitions (project_id, name);

CREATE TABLE alert_rules (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    device_id uuid REFERENCES devices (id) ON DELETE CASCADE,
    metric_key text NOT NULL DEFAULT '',
    name text NOT NULL,
    rule_type text NOT NULL,
    operator text NOT NULL DEFAULT '',
    threshold_value double precision,
    duration_seconds integer NOT NULL DEFAULT 0,
    enabled boolean NOT NULL DEFAULT true,
    configuration jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT alert_rules_type_check CHECK (rule_type IN ('metric_threshold', 'device_offline')),
    CONSTRAINT alert_rules_operator_check CHECK (operator IN ('', '>', '>=', '<', '<=', '==', '!=')),
    CONSTRAINT alert_rules_duration_check CHECK (duration_seconds >= 0 AND duration_seconds <= 604800)
);

CREATE INDEX alert_rules_project_idx ON alert_rules (project_id);

CREATE TABLE alert_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    alert_rule_id uuid NOT NULL REFERENCES alert_rules (id) ON DELETE CASCADE,
    device_id uuid NOT NULL REFERENCES devices (id) ON DELETE CASCADE,
    status text NOT NULL,
    message text NOT NULL,
    triggered_at timestamptz NOT NULL DEFAULT now(),
    resolved_at timestamptz,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT alert_events_status_check CHECK (status IN ('triggered', 'resolved'))
);

CREATE UNIQUE INDEX alert_events_active_idx ON alert_events (alert_rule_id, device_id) WHERE status = 'triggered';
CREATE INDEX alert_events_rule_triggered_idx ON alert_events (alert_rule_id, triggered_at DESC);

CREATE TABLE alert_breaches (
    alert_rule_id uuid NOT NULL REFERENCES alert_rules (id) ON DELETE CASCADE,
    device_id uuid NOT NULL REFERENCES devices (id) ON DELETE CASCADE,
    since timestamptz NOT NULL,
    PRIMARY KEY (alert_rule_id, device_id)
);

-- +goose Down
DROP TABLE alert_breaches;
DROP TABLE alert_events;
DROP TABLE alert_rules;
DROP TABLE control_definitions;
DROP TABLE device_commands;
