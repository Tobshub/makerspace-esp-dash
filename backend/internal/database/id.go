package database

import "github.com/jackc/pgx/v5/pgtype"

// ID is a UUID string that pgx can send to a uuid column.
type ID string

func (id ID) UUIDValue() (pgtype.UUID, error) {
	if id == "" {
		return pgtype.UUID{}, nil
	}
	var parsed pgtype.UUID
	if err := parsed.Scan(string(id)); err != nil {
		return pgtype.UUID{}, err
	}
	return parsed, nil
}
