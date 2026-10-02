// Package ingest turns MQTT device publishes into telemetry, state, presence, and debug events.
// The path is subscriber, parser, validator, then this service, then the database.
package ingest
