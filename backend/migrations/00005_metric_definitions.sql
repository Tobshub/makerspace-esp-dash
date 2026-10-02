-- +goose Up
-- Display configuration for a telemetry key. Rows are optional: telemetry is stored either way.
CREATE TABLE metric_definitions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    key text NOT NULL,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    data_type text NOT NULL,
    unit text NOT NULL DEFAULT '',
    display_type text NOT NULL,
    min_value double precision,
    max_value double precision,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT metric_definitions_project_key_unique UNIQUE (project_id, key),
    CONSTRAINT metric_definitions_data_type_check CHECK (data_type IN ('number', 'boolean', 'string')),
    CONSTRAINT metric_definitions_display_type_check CHECK (display_type IN ('number', 'line', 'gauge', 'boolean', 'status', 'text')),
    CONSTRAINT metric_definitions_range_check CHECK (min_value IS NULL OR max_value IS NULL OR min_value < max_value)
);

-- +goose Down
DROP TABLE metric_definitions;
