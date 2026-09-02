// Package worker hosts the cron-driven background jobs: streak recalculation
// and reminder dispatch.
package worker

import (
	"context"
	"log/slog"
	"time"

	"nowdone/internal/repository"
	"nowdone/internal/service"
)

// ReminderWorker polls for due, unsent task reminders and pushes them to
// Telegram with "Выполнить" / "Отложить" inline buttons.
type ReminderWorker struct {
	tasks   *repository.TaskRepository
	users   *repository.UserRepository
	notify  *service.NotificationService
	log     *slog.Logger
}

func NewReminderWorker(tasks *repository.TaskRepository, users *repository.UserRepository, notify *service.NotificationService, log *slog.Logger) *ReminderWorker {
	return &ReminderWorker{tasks: tasks, users: users, notify: notify, log: log}
}

// Run checks for due reminders and sends them. Intended to be called every
// minute by the cron scheduler.
//
// reminder_time is stored as an absolute instant (TIMESTAMPTZ), so the "is it
// due yet?" comparison against the current UTC instant is already timezone-
// correct — a reminder saved for 18:00 Europe/Moscow is stored as 15:00Z and
// fires at 15:00Z. The user's timezone is only needed to render the local time
// back to them.
func (w *ReminderWorker) Run(ctx context.Context) {
	now := time.Now().UTC()
	due, err := w.tasks.DuePendingReminders(ctx, now)
	if err != nil {
		w.log.Error("list due reminders", "error", err)
		return
	}

	for _, task := range due {
		user, err := w.users.GetByID(ctx, task.UserID)
		if err != nil {
			w.log.Error("get user for reminder", "task_id", task.ID, "error", err)
			continue
		}

		localTime := ""
		if task.ReminderTime != nil {
			loc := user.Location()
			localTime = task.ReminderTime.In(loc).Format("15:04")
			w.log.Info("sending reminder",
				"task_id", task.ID,
				"user_tz", user.Timezone,
				"reminder_utc", task.ReminderTime.UTC().Format(time.RFC3339),
				"reminder_local", task.ReminderTime.In(loc).Format("2006-01-02 15:04 MST"),
			)
		}

		if err := w.notify.SendReminder(user.TelegramID, task, localTime); err != nil {
			w.log.Error("send reminder", "task_id", task.ID, "error", err)
			continue
		}

		if err := w.tasks.MarkReminderSent(ctx, task.ID); err != nil {
			w.log.Error("mark reminder sent", "task_id", task.ID, "error", err)
		}
	}
}
