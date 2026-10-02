package devices

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/access"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/database"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/mqtt"
)

const (
	StatusOnline   = "online"
	StatusOffline  = "offline"
	StatusUnknown  = "unknown"
	StatusDisabled = "disabled"
)

var ErrNotFound = access.ErrNotFound

type Device struct {
	ID              string          `json:"id"`
	ProjectID       string          `json:"projectId"`
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	DeviceKey       string          `json:"deviceKey"`
	Status          string          `json:"status"`
	LastSeenAt      *time.Time      `json:"lastSeenAt"`
	FirmwareVersion string          `json:"firmwareVersion"`
	Metadata        json.RawMessage `json:"metadata"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

type Connection struct {
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	TelemetryTopic string `json:"telemetryTopic"`
	StateTopic     string `json:"stateTopic"`
	CommandTopic   string `json:"commandTopic"`
	AckTopic       string `json:"ackTopic"`
	EventsTopic    string `json:"eventsTopic"`
	StatusTopic    string `json:"statusTopic"`
}

type IssuedCredentials struct {
	Device          Device     `json:"device"`
	Credentials     creds      `json:"credentials"`
	Connection      Connection `json:"connection"`
	FirmwareSnippet string     `json:"firmwareSnippet"`
}

type creds struct {
	Username string `json:"username"`
	Secret   string `json:"secret"`
}

func GenerateSecret() (string, string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	secret := base64.RawURLEncoding.EncodeToString(buf)
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return "", "", err
	}
	return secret, string(hash), nil
}

func CheckSecret(hash, secret string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(secret)) == nil
}

func GenerateDeviceKey() (string, error) {
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "dev_" + hex.EncodeToString(buf), nil
}

func FirmwareSnippet(host string, port int, projectID, deviceKey, secret string) string {
	host = mqtt.SafeHost(host)
	return fmt.Sprintf(`// Copy into examples/esp32-basic. Do not commit these values.
// The local broker allows anonymous connections. Production checks this password over TLS.
#define MQTT_HOST "%s"
#define MQTT_PORT %d
#define DEVICE_KEY "%s"
#define DEVICE_SECRET "%s"
#define PROJECT_ID "%s"

// Publish telemetry (QoS 0): %s
// Publish state (QoS 1): %s
// Presence topic, retained: %s
//   Last Will payload: {"status":"offline"}
//   On connect payload: {"status":"online"}
// Subscribe commands: %s
// Starter command: set_led {"enabled":true} or {"enabled":false}
`, host, port, deviceKey, secret, projectID,
		mqtt.DeviceTopic(projectID, deviceKey, "telemetry"),
		mqtt.DeviceTopic(projectID, deviceKey, "state"),
		mqtt.DeviceTopic(projectID, deviceKey, "status"),
		mqtt.DeviceTopic(projectID, deviceKey, "commands"))
}

func issue(device Device, secret, host string, port int) IssuedCredentials {
	conn := Connection{
		Host:           mqtt.SafeHost(host),
		Port:           port,
		Username:       device.DeviceKey,
		Password:       secret,
		TelemetryTopic: mqtt.DeviceTopic(device.ProjectID, device.DeviceKey, "telemetry"),
		StateTopic:     mqtt.DeviceTopic(device.ProjectID, device.DeviceKey, "state"),
		CommandTopic:   mqtt.DeviceTopic(device.ProjectID, device.DeviceKey, "commands"),
		AckTopic:       mqtt.DeviceTopic(device.ProjectID, device.DeviceKey, "commands/ack"),
		EventsTopic:    mqtt.DeviceTopic(device.ProjectID, device.DeviceKey, "events"),
		StatusTopic:    mqtt.DeviceTopic(device.ProjectID, device.DeviceKey, "status"),
	}
	return IssuedCredentials{
		Device:          device,
		Credentials:     creds{Username: device.DeviceKey, Secret: secret},
		Connection:      conn,
		FirmwareSnippet: FirmwareSnippet(host, port, device.ProjectID, device.DeviceKey, secret),
	}
}

func listDevices(ctx context.Context, pool *pgxpool.Pool, projectID string) ([]Device, error) {
	rows, err := pool.Query(ctx, `
		SELECT id::text, project_id::text, name, description, device_key, status,
		       last_seen_at, firmware_version, metadata, created_at, updated_at
		FROM devices
		WHERE project_id = $1
		ORDER BY created_at DESC
	`, database.ID(projectID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Device{}
	for rows.Next() {
		device, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, device)
	}
	return out, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanDevice(row scanner) (Device, error) {
	var device Device
	var metadata []byte
	if err := row.Scan(
		&device.ID, &device.ProjectID, &device.Name, &device.Description, &device.DeviceKey,
		&device.Status, &device.LastSeenAt, &device.FirmwareVersion, &metadata,
		&device.CreatedAt, &device.UpdatedAt,
	); err != nil {
		return Device{}, err
	}
	if len(metadata) == 0 {
		metadata = []byte(`{}`)
	}
	device.Metadata = metadata
	return device, nil
}

func createDevice(ctx context.Context, pool *pgxpool.Pool, projectID, name, description string, metadata json.RawMessage, host string, port int) (IssuedCredentials, error) {
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}
	secret, hash, err := GenerateSecret()
	if err != nil {
		return IssuedCredentials{}, err
	}
	var last error
	for i := 0; i < 5; i++ {
		key, err := GenerateDeviceKey()
		if err != nil {
			return IssuedCredentials{}, err
		}
		row := pool.QueryRow(ctx, `
			INSERT INTO devices (project_id, name, description, device_key, secret_hash, status, metadata)
			VALUES ($1, $2, $3, $4, $5, 'unknown', $6)
			RETURNING id::text, project_id::text, name, description, device_key, status,
			          last_seen_at, firmware_version, metadata, created_at, updated_at
		`, database.ID(projectID), name, description, key, hash, []byte(metadata))
		device, err := scanDevice(row)
		if err == nil {
			return issue(device, secret, host, port), nil
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.ConstraintName == "devices_device_key_unique" {
			last = err
			continue
		}
		return IssuedCredentials{}, err
	}
	if last == nil {
		last = errors.New("device key exhausted")
	}
	return IssuedCredentials{}, last
}

func getDevice(ctx context.Context, pool *pgxpool.Pool, deviceID string) (Device, error) {
	row := pool.QueryRow(ctx, `
		SELECT id::text, project_id::text, name, description, device_key, status,
		       last_seen_at, firmware_version, metadata, created_at, updated_at
		FROM devices WHERE id = $1
	`, database.ID(deviceID))
	device, err := scanDevice(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Device{}, ErrNotFound
	}
	return device, err
}

func updateDevice(ctx context.Context, pool *pgxpool.Pool, deviceID, name, description string, metadata json.RawMessage) (Device, error) {
	row := pool.QueryRow(ctx, `
		UPDATE devices
		SET name = $2, description = $3, metadata = $4, updated_at = now()
		WHERE id = $1
		RETURNING id::text, project_id::text, name, description, device_key, status,
		          last_seen_at, firmware_version, metadata, created_at, updated_at
	`, database.ID(deviceID), name, description, []byte(metadata))
	device, err := scanDevice(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Device{}, ErrNotFound
	}
	return device, err
}

func disableDevice(ctx context.Context, pool *pgxpool.Pool, deviceID string) (Device, error) {
	row := pool.QueryRow(ctx, `
		UPDATE devices SET status = 'disabled', updated_at = now()
		WHERE id = $1
		RETURNING id::text, project_id::text, name, description, device_key, status,
		          last_seen_at, firmware_version, metadata, created_at, updated_at
	`, database.ID(deviceID))
	device, err := scanDevice(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Device{}, ErrNotFound
	}
	return device, err
}

func rotateSecret(ctx context.Context, pool *pgxpool.Pool, deviceID, host string, port int) (IssuedCredentials, error) {
	secret, hash, err := GenerateSecret()
	if err != nil {
		return IssuedCredentials{}, err
	}
	row := pool.QueryRow(ctx, `
		UPDATE devices SET secret_hash = $2, updated_at = now()
		WHERE id = $1
		RETURNING id::text, project_id::text, name, description, device_key, status,
		          last_seen_at, firmware_version, metadata, created_at, updated_at
	`, database.ID(deviceID), hash)
	device, err := scanDevice(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return IssuedCredentials{}, ErrNotFound
	}
	if err != nil {
		return IssuedCredentials{}, err
	}
	return issue(device, secret, host, port), nil
}

func deleteDevice(ctx context.Context, pool *pgxpool.Pool, deviceID string) error {
	tag, err := pool.Exec(ctx, `DELETE FROM devices WHERE id = $1`, database.ID(deviceID))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
