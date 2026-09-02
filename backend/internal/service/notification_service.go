package service

import (
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"nowdone/internal/models"
)

// NotificationService sends task reminders to Telegram with inline
// "Выполнить" / "Отложить" buttons.
type NotificationService struct {
	bot *tgbotapi.BotAPI
}

func NewNotificationService(bot *tgbotapi.BotAPI) *NotificationService {
	return &NotificationService{bot: bot}
}

// SendReminder pushes a reminder message for a task to the user's chat.
// localTime, when non-empty, is the reminder time already formatted in the
// user's timezone (e.g. "18:30") and is shown in the message.
func (n *NotificationService) SendReminder(chatID int64, task *models.Task, localTime string) error {
	text := fmt.Sprintf("⏰ Напоминание: *%s*", tgEscape(task.Title))
	if localTime != "" {
		text = fmt.Sprintf("⏰ Напоминание на %s: *%s*", localTime, tgEscape(task.Title))
	}

	// Callback data is namespaced "remind:<action>:<taskID>" so the bot's single
	// callback router can tell reminder buttons apart from task-list / card /
	// donation buttons. The bot also still accepts the legacy "done:" / "snooze:"
	// prefixes for reminders sent before this change.
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Выполнить", "remind:done:"+task.ID.String()),
			tgbotapi.NewInlineKeyboardButtonData("⏰ Отложить на 1 час", "remind:snooze:"+task.ID.String()),
		),
	)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.ReplyMarkup = keyboard

	_, err := n.bot.Send(msg)
	if err != nil {
		return fmt.Errorf("send reminder: %w", err)
	}
	return nil
}

// tgEscape escapes Markdown special characters so task titles can't break
// message formatting.
var tgEscapeReplacer = strings.NewReplacer("_", "\\_", "*", "\\*", "[", "\\[", "`", "\\`")

func tgEscape(s string) string {
	return tgEscapeReplacer.Replace(s)
}
