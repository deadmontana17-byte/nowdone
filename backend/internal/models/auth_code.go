package models

import (
	"time"

	"github.com/google/uuid"
)

// AuthCodePurpose distinguishes login codes from PIN-reset codes.
type AuthCodePurpose string

const (
	PurposeAuth  AuthCodePurpose = "auth"
	PurposeReset AuthCodePurpose = "reset"
)

// AuthCode is a one-time 6-digit code exchanged via the Telegram bot deep link.
type AuthCode struct {
	ID             uuid.UUID       `json:"id"`
	Code           string          `json:"-"`
	Purpose        AuthCodePurpose `json:"purpose"`
	UserID         *uuid.UUID      `json:"user_id,omitempty"`
	TelegramChatID *int64          `json:"-"`
	IsUsed         bool            `json:"-"`
	ExpiresAt      time.Time       `json:"expires_at"`
	CreatedAt      time.Time       `json:"created_at"`
}

// IsExpired reports whether the code can no longer be redeemed.
func (a *AuthCode) IsExpired() bool {
	return time.Now().After(a.ExpiresAt)
}

// Confirmed reports whether the bot has already delivered the code to a chat,
// meaning it is ready to be exchanged on the website.
func (a *AuthCode) Confirmed() bool {
	return a.TelegramChatID != nil
}
