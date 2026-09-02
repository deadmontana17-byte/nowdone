package models

import (
	"time"

	"github.com/google/uuid"
)

// Attachment is a single file (image/video/document) attached to a task or note.
type Attachment struct {
	Type string `json:"type"` // "image" | "video" | "file"
	URL  string `json:"url"`
	Name string `json:"name"`
}

// RecurrenceRule describes how a recurring task repeats.
// Frequency: "daily" | "weekdays" | "weekly" | "monthly" | "yearly".
//   - "weekdays" repeats every Monday–Friday (Interval/WeekDays ignored).
//   - "yearly" repeats on the same month/day, Interval years apart.
//
// WeekDays is used only when Frequency == "weekly" (0=Sunday..6=Saturday).
type RecurrenceRule struct {
	Frequency string `json:"frequency"`
	Interval  int    `json:"interval,omitempty"` // every N days/weeks/months/years, default 1
	WeekDays  []int  `json:"week_days,omitempty"`
}

// Task is a single to-do item, optionally recurring, optionally reminder-backed.
type Task struct {
	ID              uuid.UUID       `json:"id"`
	UserID          uuid.UUID       `json:"user_id"`
	TypeID          *uuid.UUID      `json:"type_id,omitempty"`
	Title           string          `json:"title"`
	Description     JSONRaw         `json:"description"`
	Attachments     []Attachment    `json:"attachments"`
	Date            Date            `json:"date"`
	IsDone          bool            `json:"is_done"`
	ReminderTime    *time.Time      `json:"reminder_time,omitempty"`
	ReminderSent    bool            `json:"-"`
	IsRecurring     bool            `json:"is_recurring"`
	RecurrenceRule  *RecurrenceRule `json:"recurrence_rule,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
}
