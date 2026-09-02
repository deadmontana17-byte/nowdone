package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"nowdone/internal/models"
	"nowdone/internal/repository"
)

// ErrAttachmentCleanup is returned when a task's attachment could not be removed
// from object storage. The task update/delete is aborted so the DB never gets
// out of sync with what actually exists in S3.
var ErrAttachmentCleanup = errors.New("attachment cleanup failed")

// TaskService implements task CRUD plus recurrence rollover: completing a
// recurring task automatically spawns the next instance per its rule.
type TaskService struct {
	tasks *repository.TaskRepository
	s3    *S3Service // nil in processes without object storage (e.g. the bot)
	log   *slog.Logger
}

// NewTaskService wires the task repository together with object storage. Pass a
// nil *S3Service where attachment cleanup is not needed/available (the bot); a
// nil logger falls back to slog.Default().
func NewTaskService(tasks *repository.TaskRepository, s3 *S3Service, log *slog.Logger) *TaskService {
	if log == nil {
		log = slog.Default()
	}
	return &TaskService{tasks: tasks, s3: s3, log: log}
}

func (s *TaskService) ListRange(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]*models.Task, error) {
	return s.tasks.ListByUserAndRange(ctx, userID, from, to)
}

// Get returns a single task owned by userID. Used by the Telegram bot to render
// a task card and to read the current status before toggling it.
func (s *TaskService) Get(ctx context.Context, userID, id uuid.UUID) (*models.Task, error) {
	return s.tasks.GetByID(ctx, userID, id)
}

func (s *TaskService) Create(ctx context.Context, t *models.Task) (*models.Task, error) {
	return s.tasks.Create(ctx, t)
}

// Update applies a partial update. When the update marks a recurring task as
// done for the first time, a new task instance is created for the next
// occurrence per recurrence_rule.
func (s *TaskService) Update(ctx context.Context, userID, id uuid.UUID, u repository.TaskUpdate) (*models.Task, error) {
	existing, err := s.tasks.GetByID(ctx, userID, id)
	if err != nil {
		return nil, err
	}

	// If the caller replaced the attachment list, purge any dropped files from
	// S3 *before* persisting. A storage failure aborts the update, so the task
	// is never saved without files that still exist in the bucket.
	if u.Attachments != nil {
		removed := removedAttachmentURLs(existing.Attachments, *u.Attachments)
		if err := s.deleteAttachments(ctx, userID, id, removed); err != nil {
			return nil, err
		}
	}

	// Reuse the row we just loaded instead of making the repository SELECT it
	// again — this is the hot path for every checkbox toggle.
	updated, err := s.tasks.UpdateFrom(ctx, existing, u)
	if err != nil {
		return nil, err
	}

	justCompleted := u.IsDone != nil && *u.IsDone && !existing.IsDone
	if justCompleted && updated.IsRecurring && updated.RecurrenceRule != nil {
		if err := s.spawnNextOccurrence(ctx, updated); err != nil {
			return nil, fmt.Errorf("spawn next occurrence: %w", err)
		}
	}

	return updated, nil
}

func (s *TaskService) Delete(ctx context.Context, userID, id uuid.UUID) error {
	// Read the attachment list first, remove the files from S3, and only then
	// drop the row. If S3 deletion fails we return the error and keep the task,
	// so a retry can finish the cleanup.
	attachments, err := s.tasks.AttachmentsByID(ctx, userID, id)
	if err != nil {
		return err
	}

	urls := make([]string, 0, len(attachments))
	for _, a := range attachments {
		if a.URL != "" {
			urls = append(urls, a.URL)
		}
	}
	if err := s.deleteAttachments(ctx, userID, id, urls); err != nil {
		return err
	}

	return s.tasks.Delete(ctx, userID, id)
}

// deleteAttachments removes the given attachment URLs from object storage.
//
//   - A file still referenced by another of the user's tasks (recurring-task
//     copies share attachment URLs) is kept.
//   - A URL that is not one of our S3 objects, or a file already gone, counts
//     as done.
//   - Any real storage error aborts and is returned as ErrAttachmentCleanup so
//     the caller does not touch the DB.
func (s *TaskService) deleteAttachments(ctx context.Context, userID, taskID uuid.UUID, urls []string) error {
	if len(urls) == 0 {
		return nil
	}
	if s.s3 == nil {
		s.log.Warn("attachment cleanup skipped: no object storage in this process",
			"task_id", taskID, "count", len(urls))
		return nil
	}

	for _, url := range urls {
		key, ok := s.s3.KeyFromURL(url)
		if !ok {
			s.log.Warn("attachment URL not recognised as an S3 object, skipping",
				"task_id", taskID, "url", url)
			continue
		}

		stillUsed, err := s.tasks.OtherTaskReferencesURL(ctx, userID, taskID, url)
		if err != nil {
			s.log.Error("check attachment references", "task_id", taskID, "key", key, "error", err)
			return fmt.Errorf("%w: %v", ErrAttachmentCleanup, err)
		}
		if stillUsed {
			s.log.Info("attachment kept: still referenced by another task",
				"task_id", taskID, "key", key)
			continue
		}

		if err := s.s3.DeleteObject(ctx, key); err != nil {
			s.log.Error("delete attachment from S3", "task_id", taskID, "key", key, "error", err)
			return fmt.Errorf("%w: key %s: %v", ErrAttachmentCleanup, key, err)
		}
		s.log.Info("attachment deleted from S3", "task_id", taskID, "key", key)
	}
	return nil
}

// removedAttachmentURLs returns the (de-duplicated) URLs present in old but not
// in next — i.e. the files the caller detached.
func removedAttachmentURLs(old, next []models.Attachment) []string {
	keep := make(map[string]struct{}, len(next))
	for _, a := range next {
		keep[a.URL] = struct{}{}
	}

	var removed []string
	seen := make(map[string]struct{})
	for _, a := range old {
		if a.URL == "" {
			continue
		}
		if _, stillThere := keep[a.URL]; stillThere {
			continue
		}
		if _, dup := seen[a.URL]; dup {
			continue
		}
		seen[a.URL] = struct{}{}
		removed = append(removed, a.URL)
	}
	return removed
}

func (s *TaskService) spawnNextOccurrence(ctx context.Context, t *models.Task) error {
	next := nextOccurrenceDate(t.Date.Time, t.RecurrenceRule)
	if next.IsZero() {
		return nil
	}

	newTask := &models.Task{
		UserID:         t.UserID,
		TypeID:         t.TypeID,
		Title:          t.Title,
		Description:    t.Description,
		Attachments:    t.Attachments,
		Date:           models.NewDate(next),
		IsDone:         false,
		IsRecurring:    true,
		RecurrenceRule: t.RecurrenceRule,
	}
	if t.ReminderTime != nil {
		diff := t.ReminderTime.Sub(t.Date.Time)
		reminderTime := next.Add(diff)
		newTask.ReminderTime = &reminderTime
	}

	_, err := s.tasks.Create(ctx, newTask)
	return err
}

// nextOccurrenceDate computes the next date after `from` according to rule.
// Returns zero time if the rule is not understood.
func nextOccurrenceDate(from time.Time, rule *models.RecurrenceRule) time.Time {
	interval := rule.Interval
	if interval < 1 {
		interval = 1
	}

	switch rule.Frequency {
	case "daily":
		return from.AddDate(0, 0, interval)
	case "weekdays":
		// Next Monday–Friday strictly after `from`; interval is not used.
		next := from.AddDate(0, 0, 1)
		for next.Weekday() == time.Saturday || next.Weekday() == time.Sunday {
			next = next.AddDate(0, 0, 1)
		}
		return next
	case "monthly":
		return from.AddDate(0, interval, 0)
	case "yearly":
		return from.AddDate(interval, 0, 0)
	case "weekly":
		if len(rule.WeekDays) == 0 {
			return from.AddDate(0, 0, 7*interval)
		}
		// Find the next matching weekday after `from`, searching up to 7*interval+7 days ahead.
		for i := 1; i <= 7*interval+7; i++ {
			candidate := from.AddDate(0, 0, i)
			for _, wd := range rule.WeekDays {
				if int(candidate.Weekday()) == wd {
					return candidate
				}
			}
		}
		return time.Time{}
	default:
		return time.Time{}
	}
}
