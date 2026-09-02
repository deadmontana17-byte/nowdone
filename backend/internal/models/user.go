package models

import (
	"time"

	"github.com/google/uuid"
)

// User represents an authenticated Telegram user.
type User struct {
	ID               uuid.UUID `json:"id"`
	TelegramID       int64     `json:"telegram_id"`
	TelegramUsername string    `json:"telegram_username,omitempty"`
	FirstName        string    `json:"first_name,omitempty"`
	PINHash          string    `json:"-"`
	CurrentStreak    int       `json:"current_streak"`
	MaxStreak        int       `json:"max_streak"`
	// Timezone is an IANA name (e.g. "Europe/Moscow"). Defaults to "UTC".
	// Reminder times without an explicit offset are interpreted in this zone.
	Timezone  string    `json:"timezone"`
	CreatedAt time.Time `json:"created_at"`
}

// HasPIN reports whether the user has already set up a PIN code.
func (u *User) HasPIN() bool {
	return u.PINHash != ""
}

// Location resolves the user's timezone, falling back to UTC when it is empty
// or not a name the runtime's zone database knows.
func (u *User) Location() *time.Location {
	if u.Timezone == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(u.Timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}
