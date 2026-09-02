package service

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// TelegramCodeSender delivers one-time codes straight to a user's Telegram chat
// via the Bot API, without any bot /start deep-link round-trip. Used by the API
// process for the PIN-reset flow.
type TelegramCodeSender struct {
	bot *tgbotapi.BotAPI
}

func NewTelegramCodeSender(bot *tgbotapi.BotAPI) *TelegramCodeSender {
	return &TelegramCodeSender{bot: bot}
}

// SendResetCode pushes the PIN-reset code to the given chat. For a private
// (one-on-one) chat the Telegram chat ID equals the user's Telegram ID, so the
// caller can pass user.TelegramID here.
func (t *TelegramCodeSender) SendResetCode(chatID int64, code string) error {
	text := fmt.Sprintf("🔑 Код для сброса PIN в NowDone: *%s*\n\nВведите его на сайте в течение 5 минут.", code)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdown
	if _, err := t.bot.Send(msg); err != nil {
		return fmt.Errorf("send reset code message: %w", err)
	}
	return nil
}
