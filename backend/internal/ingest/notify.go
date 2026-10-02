package ingest

import (
	"encoding/json"
	"time"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/devices"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/realtime"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/telemetry"
)

func (s *Service) notifyChange(device deviceRow, ch change, when time.Time) {
	if s == nil || s.notify == nil {
		return
	}
	if ch.status == devices.StatusOnline && device.Status != devices.StatusOnline {
		s.notify(device.ProjectID, realtime.Event{
			Type:      realtime.DeviceOnline,
			DeviceID:  device.ID,
			Timestamp: when,
			Data:      map[string]string{"status": devices.StatusOnline},
		})
	}
	if ch.status == devices.StatusOffline && device.Status != devices.StatusOffline {
		s.notify(device.ProjectID, realtime.Event{
			Type:      realtime.DeviceOffline,
			DeviceID:  device.ID,
			Timestamp: when,
			Data:      map[string]string{"status": devices.StatusOffline},
		})
	}
	if len(ch.points) > 0 {
		s.notify(device.ProjectID, realtime.Event{
			Type:      realtime.TelemetryReceived,
			DeviceID:  device.ID,
			Timestamp: when,
			Data:      metricData(ch.points),
		})
	}
	if len(ch.state) > 0 {
		s.notify(device.ProjectID, realtime.Event{
			Type:      realtime.StateUpdated,
			DeviceID:  device.ID,
			Timestamp: when,
			Data:      json.RawMessage(ch.state),
		})
	}
	if ch.eventType == "command_ack" && json.Valid(ch.payload) {
		s.notify(device.ProjectID, realtime.Event{
			Type:      realtime.CommandUpdated,
			DeviceID:  device.ID,
			Timestamp: when,
			Data:      json.RawMessage(append([]byte(nil), ch.payload...)),
		})
	}
}

func metricData(points []telemetry.Point) map[string]any {
	data := make(map[string]any, len(points))
	for _, point := range points {
		switch {
		case point.Numeric != nil:
			data[point.Key] = *point.Numeric
		case point.Boolean != nil:
			data[point.Key] = *point.Boolean
		case point.Text != nil:
			data[point.Key] = *point.Text
		}
	}
	return data
}
