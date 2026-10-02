package events

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/database"
)

// Execer is the subset of a pool or transaction used to insert one event.
type Execer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// Insert stores one debug event. Payload must be JSON.
func Insert(ctx context.Context, db Execer, projectID, deviceID, eventType, topic string, payload []byte) error {
	if !json.Valid(payload) {
		return errors.New("event payload must be json")
	}
	_, err := db.Exec(ctx, `
		INSERT INTO device_events (project_id, device_id, event_type, topic, payload)
		VALUES ($1, $2, $3, $4, $5)
	`, database.ID(projectID), database.ID(deviceID), eventType, topic, payload)
	return err
}
