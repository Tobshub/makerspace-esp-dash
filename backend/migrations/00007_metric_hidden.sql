-- +goose Up
ALTER TABLE metric_definitions
    ADD COLUMN hidden boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE metric_definitions
    DROP COLUMN hidden;
