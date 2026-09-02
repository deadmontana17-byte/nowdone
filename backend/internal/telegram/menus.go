package telegram

import (
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"

	"nowdone/internal/models"
)

// Reply-keyboard button captions. Incoming text messages are matched against
// these (exact match) before the natural-language pipeline runs.
const (
	btnToday    = "📋 Задачи на сегодня"
	btnTomorrow = "📅 Задачи на завтра"
	btnAddTask  = "➕ Добавить задачу"
	btnAddNote  = "📝 Добавить заметку"
	btnDonate   = "⭐ Поддержать автора"
	btnCancel   = "❌ Отмена"
)

// mainMenuKeyboard is the persistent reply keyboard shown after /start and after
// every completed action (spec §2). "Сегодня" and "Завтра" sit together on the
// top row.
func mainMenuKeyboard() tgbotapi.ReplyKeyboardMarkup {
	kb := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(btnToday),
			tgbotapi.NewKeyboardButton(btnTomorrow),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(btnAddTask),
			tgbotapi.NewKeyboardButton(btnAddNote),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(btnDonate),
		),
	)
	kb.ResizeKeyboard = true
	return kb
}

// draftKeyboard replaces the main menu while the user composes a task/note.
// There is no "Готово" button — sending any text / photo / voice creates the
// record; the only action offered is cancelling.
func draftKeyboard() tgbotapi.ReplyKeyboardMarkup {
	kb := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(btnCancel),
		),
	)
	kb.ResizeKeyboard = true
	return kb
}

// registerCommands publishes the "/" command menu. Command names must be
// lowercase latin, so the Russian captions live in the descriptions.
func (b *Bot) registerCommands() {
	cmds := tgbotapi.NewSetMyCommands(
		tgbotapi.BotCommand{Command: "today", Description: "Задачи на сегодня"},
		tgbotapi.BotCommand{Command: "tomorrow", Description: "Задачи на завтра"},
		tgbotapi.BotCommand{Command: "add_task", Description: "Добавить задачу"},
		tgbotapi.BotCommand{Command: "add_note", Description: "Добавить заметку"},
		tgbotapi.BotCommand{Command: "donate", Description: "⭐ Поддержать автора"},
	)
	if _, err := b.api.Request(cmds); err != nil {
		b.log.Error("set my commands", "error", err)
	}
}

// --- inline keyboards -------------------------------------------------------

// taskListKeyboard renders the "today" list: one row per task with three
// buttons — title (opens the card), status (toggles), trash (deletes) — plus a
// trailing manual-refresh row so the markup always has at least one row.
func taskListKeyboard(tasks []*models.Task) tgbotapi.InlineKeyboardMarkup {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(tasks)+1)
	for _, t := range tasks {
		id := t.ID.String()
		status := "⬜"
		if t.IsDone {
			status = "✅"
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(truncateRunes(t.Title, 40), "task:open:"+id),
			tgbotapi.NewInlineKeyboardButtonData(status, "task:toggle:"+id),
			tgbotapi.NewInlineKeyboardButtonData("🗑", "task:del:"+id),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔄 Обновить", "list:refresh"),
	))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// taskCardKeyboard is the button set under a single task card (spec §4).
func taskCardKeyboard(id uuid.UUID, done bool) tgbotapi.InlineKeyboardMarkup {
	s := id.String()
	toggle := "✅ Отметить выполненной"
	if done {
		toggle = "↩️ Снять отметку"
	}
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(toggle, "card:toggle:"+s),
			tgbotapi.NewInlineKeyboardButtonData("🗑 Удалить", "card:del:"+s),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Назад к задачам", "card:back:"+s),
		),
	)
}

// --- callback data helpers ------------------------------------------------

// parseCallback splits "domain:action:uuid" callback data. id is uuid.Nil when
// the third segment is missing or not a UUID (e.g. "list:refresh",
// "donate:stars:10"). ok is false only when there is no "domain:action" at all.
func parseCallback(data string) (domain, action string, id uuid.UUID, ok bool) {
	parts := strings.Split(data, ":")
	if len(parts) < 2 {
		return "", "", uuid.Nil, false
	}
	domain, action = parts[0], parts[1]
	if len(parts) >= 3 {
		if parsed, err := uuid.Parse(parts[2]); err == nil {
			id = parsed
		}
	}
	return domain, action, id, true
}

// truncateRunes shortens s to at most n runes, appending an ellipsis when cut.
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n < 1 {
		return "…"
	}
	return string(r[:n-1]) + "…"
}
