package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nowdone/internal/models"
)

// TaskRepository provides CRUD access to the tasks table.
type TaskRepository struct {
	pool *pgxpool.Pool
}

func NewTaskRepository(pool *pgxpool.Pool) *TaskRepository {
	return &TaskRepository{pool: pool}
}

const taskColumns = `id, user_id, type_id, title, description, attachments, date, is_done, reminder_time, reminder_sent, is_recurring, recurrence_rule, created_at`

func scanTask(row pgx.Row) (*models.Task, error) {
	var t models.Task
	var description, attachments []byte
	var recurrenceRule []byte
	var date time.Time

	if err := row.Scan(
		&t.ID, &t.UserID, &t.TypeID, &t.Title, &description, &attachments,
		&date, &t.IsDone, &t.ReminderTime, &t.ReminderSent, &t.IsRecurring,
		&recurrenceRule, &t.CreatedAt,
	); err != nil {
		return nil, err
	}

	t.Description = models.JSONRaw(description)
	t.Date = models.NewDate(date)

	if err := json.Unmarshal(attachments, &t.Attachments); err != nil {
		return nil, fmt.Errorf("unmarshal attachments: %w", err)
	}
	if recurrenceRule != nil {
		var rule models.RecurrenceRule
		if err := json.Unmarshal(recurrenceRule, &rule); err != nil {
			return nil, fmt.Errorf("unmarshal recurrence rule: %w", err)
		}
		t.RecurrenceRule = &rule
	}
	return &t, nil
}

// ListByUserAndRange returns tasks for a user between from and to (inclusive),
// used to render the date-grouped planner view.
func (r *TaskRepository) ListByUserAndRange(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]*models.Task, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+taskColumns+` FROM tasks
		WHERE user_id = $1 AND date BETWEEN $2 AND $3
		ORDER BY date, created_at`,
		userID, from, to,
	)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	result := make([]*models.Task, 0)
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

// ListByUserAndDate returns all tasks for a single day, used by the streak worker.
func (r *TaskRepository) ListByUserAndDate(ctx context.Context, userID uuid.UUID, date time.Time) ([]*models.Task, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+taskColumns+` FROM tasks WHERE user_id = $1 AND date = $2`, userID, date)
	if err != nil {
		return nil, fmt.Errorf("list tasks by date: %w", err)
	}
	defer rows.Close()

	result := make([]*models.Task, 0)
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

func (r *TaskRepository) GetByID(ctx context.Context, userID, id uuid.UUID) (*models.Task, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+taskColumns+` FROM tasks WHERE id = $1 AND user_id = $2`, id, userID)
	t, err := scanTask(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}
	return t, nil
}

func (r *TaskRepository) Create(ctx context.Context, t *models.Task) (*models.Task, error) {
	attachments, err := json.Marshal(t.Attachments)
	if err != nil {
		return nil, fmt.Errorf("marshal attachments: %w", err)
	}
	var recurrenceRule []byte
	if t.RecurrenceRule != nil {
		recurrenceRule, err = json.Marshal(t.RecurrenceRule)
		if err != nil {
			return nil, fmt.Errorf("marshal recurrence rule: %w", err)
		}
	}

	row := r.pool.QueryRow(ctx, `
		INSERT INTO tasks (user_id, type_id, title, description, attachments, date, is_done, reminder_time, is_recurring, recurrence_rule)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING `+taskColumns,
		t.UserID, t.TypeID, t.Title, t.Description, attachments, t.Date.Time, t.IsDone, t.ReminderTime, t.IsRecurring, recurrenceRule,
	)
	return scanTask(row)
}

// Update performs a partial update. Non-nil pointer fields are applied.
type TaskUpdate struct {
	TypeID         *uuid.UUID
	ClearTypeID    bool
	Title          *string
	Description    *models.JSONRaw
	Attachments    *[]models.Attachment
	Date           *time.Time
	IsDone         *bool
	ReminderTime   *time.Time
	ClearReminder  bool
	IsRecurring    *bool
	RecurrenceRule *models.RecurrenceRule
}

// Update loads the row, applies the partial update and persists it. Callers that
// already hold the current row (e.g. TaskService, which reads it to diff
// attachments) should use UpdateFrom to skip the extra SELECT.
func (r *TaskRepository) Update(ctx context.Context, userID, id uuid.UUID, u TaskUpdate) (*models.Task, error) {
	existing, err := r.GetByID(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return r.UpdateFrom(ctx, existing, u)
}

// UpdateFrom applies u on top of an already-loaded row and writes it back in a
// single UPDATE. prev must be the current row for its own (id, user_id); it is
// not mutated.
func (r *TaskRepository) UpdateFrom(ctx context.Context, prev *models.Task, u TaskUpdate) (*models.Task, error) {
	next := *prev // shallow copy: applyTaskUpdate only replaces fields, never mutates in place
	applyTaskUpdate(&next, u)

	attachments, err := json.Marshal(next.Attachments)
	if err != nil {
		return nil, fmt.Errorf("marshal attachments: %w", err)
	}
	var recurrenceRule []byte
	if next.RecurrenceRule != nil {
		recurrenceRule, err = json.Marshal(next.RecurrenceRule)
		if err != nil {
			return nil, fmt.Errorf("marshal recurrence rule: %w", err)
		}
	}

	row := r.pool.QueryRow(ctx, `
		UPDATE tasks SET
			type_id = $1, title = $2, description = $3, attachments = $4, date = $5,
			is_done = $6, reminder_time = $7, reminder_sent = $8, is_recurring = $9, recurrence_rule = $10
		WHERE id = $11 AND user_id = $12
		RETURNING `+taskColumns,
		next.TypeID, next.Title, next.Description, attachments, next.Date.Time,
		next.IsDone, next.ReminderTime, next.ReminderSent, next.IsRecurring, recurrenceRule,
		prev.ID, prev.UserID,
	)
	t, err := scanTask(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}

// applyTaskUpdate mutates t in place with the non-nil fields of u.
func applyTaskUpdate(t *models.Task, u TaskUpdate) {
	if u.ClearTypeID {
		t.TypeID = nil
	} else if u.TypeID != nil {
		t.TypeID = u.TypeID
	}
	if u.Title != nil {
		t.Title = *u.Title
	}
	if u.Description != nil {
		t.Description = *u.Description
	}
	if u.Attachments != nil {
		t.Attachments = *u.Attachments
	}
	if u.Date != nil {
		t.Date = models.NewDate(*u.Date)
	}
	if u.IsDone != nil {
		t.IsDone = *u.IsDone
	}
	if u.ClearReminder {
		t.ReminderTime = nil
		t.ReminderSent = false
	} else if u.ReminderTime != nil {
		t.ReminderTime = u.ReminderTime
		t.ReminderSent = false
	}
	if u.IsRecurring != nil {
		t.IsRecurring = *u.IsRecurring
	}
	if u.RecurrenceRule != nil {
		t.RecurrenceRule = u.RecurrenceRule
	}
}

func (r *TaskRepository) Delete(ctx context.Context, userID, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM tasks WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// AttachmentsByID returns just the attachment list for one task. Used before
// deleting a task so its files can be purged from object storage first.
func (r *TaskRepository) AttachmentsByID(ctx context.Context, userID, id uuid.UUID) ([]models.Attachment, error) {
	var raw []byte
	err := r.pool.QueryRow(ctx,
		`SELECT attachments FROM tasks WHERE id = $1 AND user_id = $2`, id, userID,
	).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get task attachments: %w", err)
	}

	var attachments []models.Attachment
	if err := json.Unmarshal(raw, &attachments); err != nil {
		return nil, fmt.Errorf("unmarshal attachments: %w", err)
	}
	return attachments, nil
}

// OtherTaskReferencesURL reports whether any of the user's tasks *other than
// excludeID* still has an attachment with the given URL. Recurring tasks spawn
// copies that share the same attachment URLs, so a file must not be deleted from
// S3 while another task points at it.
func (r *TaskRepository) OtherTaskReferencesURL(ctx context.Context, userID, excludeID uuid.UUID, url string) (bool, error) {
	needle, err := json.Marshal([]map[string]string{{"url": url}})
	if err != nil {
		return false, fmt.Errorf("marshal attachment needle: %w", err)
	}

	var exists bool
	err = r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM tasks
			WHERE user_id = $1 AND id <> $2 AND attachments @> $3::jsonb
		)`,
		userID, excludeID, string(needle),
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check attachment url references: %w", err)
	}
	return exists, nil
}

// DuePendingReminders returns tasks whose reminder_time has passed and has not
// yet been sent, used by the reminder worker.
func (r *TaskRepository) DuePendingReminders(ctx context.Context, now time.Time) ([]*models.Task, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+taskColumns+` FROM tasks
		WHERE reminder_time IS NOT NULL AND reminder_sent = false AND reminder_time <= $1 AND is_done = false`,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("list due reminders: %w", err)
	}
	defer rows.Close()

	result := make([]*models.Task, 0)
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

func (r *TaskRepository) MarkReminderSent(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE tasks SET reminder_sent = true WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("mark reminder sent: %w", err)
	}
	return nil
}
