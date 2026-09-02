package models

import (
	"time"

	"github.com/google/uuid"
)

// Note is a global (not date-bound) rich-text note, optionally PIN-hidden.
type Note struct {
	ID          uuid.UUID    `json:"id"`
	UserID      uuid.UUID    `json:"user_id"`
	Title       string       `json:"title"`
	Content     JSONRaw      `json:"content"`
	Attachments []Attachment `json:"attachments"`
	IsHidden    bool         `json:"is_hidden"`
	CreatedAt   time.Time    `json:"created_at"`
}
