package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"nowdone/internal/repository"
)

// handleReminderCallback processes the inline buttons attached to reminder
// messages (spec §9):
//
//	remind:done:{taskID}   — mark the task done
//	remind:snooze:{taskID} — push the reminder 1 hour forward
//
// Both actions delete the original reminder message so the chat is not left
// with a stale prompt, then post a fresh confirmation.
func (b *Bot) handleReminderCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	_, action, taskID, ok := parseCallback(cb.Data)
	if !ok {
		b.answerCallback(cb.ID, "")
		return
	}

	chatID := cb.Message.Chat.ID
	reminderMsgID := cb.Message.MessageID

	user, err := b.users.GetByTelegramID(ctx, cb.From.ID)
	if err != nil {
		b.answerCallback(cb.ID, "Вы не авторизованы.")
		return
	}

	task, err := b.tasks.Get(ctx, user.ID, taskID)
	if err != nil {
		b.answerCallback(cb.ID, "Задача не найдена.")
		b.deleteMessage(chatID, reminderMsgID) // drop the dangling reminder
		return
	}

	switch action {
	case "done":
		done := true
		if _, err := b.tasks.Update(ctx, user.ID, taskID, repository.TaskUpdate{IsDone: &done}); err != nil {
			b.log.Error("reminder: mark done", "task_id", taskID, "error", err)
			b.answerCallback(cb.ID, "Не удалось отметить выполненной.")
			return
		}
		b.answerCallback(cb.ID, "Готово ✅")
		b.deleteMessage(chatID, reminderMsgID)
		b.replyPlain(chatID, fmt.Sprintf("✅ Задача «%s» помечена как выполненная!", task.Title))
		b.refreshTaskList(ctx, chatID, user)

	case "snooze":
		// Setting ReminderTime also clears reminder_sent, so the worker will
		// pick the task up again in an hour.
		next := time.Now().Add(time.Hour)
		if _, err := b.tasks.Update(ctx, user.ID, taskID, repository.TaskUpdate{ReminderTime: &next}); err != nil {
			b.log.Error("reminder: snooze", "task_id", taskID, "error", err)
			b.answerCallback(cb.ID, "Не удалось отложить.")
			return
		}
		b.answerCallback(cb.ID, "Отложено на час ⏰")
		b.deleteMessage(chatID, reminderMsgID)
		b.replyPlain(chatID, "⏰ Напоминание отложено на 1 час. Я напомню снова!")

	default:
		b.answerCallback(cb.ID, "")
	}
}

// handleLegacyReminderCallback adapts pre-rename reminder buttons ("done:{id}" /
// "snooze:{id}") to the current handler, so reminders sent before this release
// keep working from chat history.
func (b *Bot) handleLegacyReminderCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	parts := strings.SplitN(cb.Data, ":", 2)
	if len(parts) != 2 {
		b.answerCallback(cb.ID, "")
		return
	}
	cb.Data = "remind:" + parts[0] + ":" + parts[1]
	b.handleReminderCallback(ctx, cb)
}
