package models

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// JSONRaw wraps arbitrary JSON (e.g. Editor.js document) stored in a jsonb column.
type JSONRaw json.RawMessage

// Value implements driver.Valuer for pgx/database-sql writes.
func (j JSONRaw) Value() (driver.Value, error) {
	if len(j) == 0 {
		return "{}", nil
	}
	return string(j), nil
}

// Scan implements sql.Scanner for pgx/database-sql reads.
func (j *JSONRaw) Scan(src interface{}) error {
	if src == nil {
		*j = JSONRaw("{}")
		return nil
	}
	switch v := src.(type) {
	case []byte:
		*j = JSONRaw(append([]byte(nil), v...))
	case string:
		*j = JSONRaw(v)
	default:
		return fmt.Errorf("unsupported Scan type for JSONRaw: %T", src)
	}
	return nil
}

// MarshalJSON passes the raw bytes through unchanged.
func (j JSONRaw) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("{}"), nil
	}
	return j, nil
}

// UnmarshalJSON stores the raw bytes unchanged (compacted).
func (j *JSONRaw) UnmarshalJSON(data []byte) error {
	var buf bytes.Buffer
	if err := json.Compact(&buf, data); err != nil {
		return err
	}
	*j = JSONRaw(buf.Bytes())
	return nil
}

// Date is a calendar date (no time component), stored as PostgreSQL `date`.
type Date struct {
	time.Time
}

const dateLayout = "2006-01-02"

// MarshalJSON renders the date as "YYYY-MM-DD".
func (d Date) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.Format(dateLayout) + `"`), nil
}

// UnmarshalJSON parses a "YYYY-MM-DD" string.
func (d *Date) UnmarshalJSON(data []byte) error {
	s := string(bytes.Trim(data, `"`))
	if s == "null" || s == "" {
		return nil
	}
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return fmt.Errorf("invalid date %q: %w", s, err)
	}
	d.Time = t
	return nil
}

// Value implements driver.Valuer, sending the date to Postgres as `date`.
func (d Date) Value() (driver.Value, error) {
	return d.Time, nil
}

// Scan implements sql.Scanner for reading a Postgres `date` column.
func (d *Date) Scan(src interface{}) error {
	switch v := src.(type) {
	case time.Time:
		d.Time = v
	case nil:
		d.Time = time.Time{}
	default:
		return fmt.Errorf("unsupported Scan type for Date: %T", src)
	}
	return nil
}

// NewDate truncates a time.Time to midnight, used when constructing dates in code.
func NewDate(t time.Time) Date {
	return Date{time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())}
}
