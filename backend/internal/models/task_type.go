package models

import (
	"time"

	"github.com/google/uuid"
)

// TaskType is a user-defined category for tasks, e.g. "💪 Спорт".
type TaskType struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Emoji     string    `json:"emoji"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}
