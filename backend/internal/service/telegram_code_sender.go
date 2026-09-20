package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"nowdone/pkg/retry"
)

// resetCodePolicy bounds the PIN-reset code send. Delivering it twice is
// harmless (same code, same 5-minute window), so unlike the bot's chat messages
// it can retry on any transient error. Each attempt gets the full 60s the HTTP
// client allows; backoff is 1s, 2s.
var resetCodePolicy = retry.Policy{
	Attempts:       3,
	BaseDelay:      1 * time.Second,
	MaxDelay:       4 * time.Second,
	AttemptTimeout: 60 * time.Second,
}

// TelegramCodeSender delivers one-time codes straight to a user's Telegram chat
// via the Bot API, without any bot /start deep-link round-trip. Used by the API
// process for the PIN-reset flow.
type TelegramCodeSender struct {
	bot *tgbotapi.BotAPI
	log *slog.Logger
}

func NewTelegramCodeSender(bot *tgbotapi.BotAPI, log *slog.Logger) *TelegramCodeSender {
	return &TelegramCodeSender{bot: bot, log: log}
}

// SendResetCode pushes the PIN-reset code to the given chat. For a private
// (one-on-one) chat the Telegram chat ID equals the user's Telegram ID, so the
// caller can pass user.TelegramID here.
//
// The call goes through the same Cloudflare Worker as the bot, so it gets the
// same treatment: bounded retries with exponential backoff on transient network
// failures, a hard per-attempt deadline (tgbotapi has no context support, so the
// blocking Send runs in a goroutine we can abandon), and one timing log line per
// attempt. ctx is the caller's own request context, so a stuck Telegram response
// aborts at the caller's deadline instead of retrying for up to 3 minutes.
func (t *TelegramCodeSender) SendResetCode(ctx context.Context, chatID int64, code string) error {
	text := fmt.Sprintf("🔑 Код для сброса PIN в NowDone: *%s*\n\nВведите его на сайте в течение 5 минут.", code)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdown

	start := time.Now()
	_, err := retry.DoValue(ctx, resetCodePolicy, func(ctx context.Context) (tgbotapi.Message, error) {
		return sendWithDeadline(ctx, func() (tgbotapi.Message, error) { return t.bot.Send(msg) })
	})
	t.logSend(start, err)
	if err != nil {
		return fmt.Errorf("send reset code message: %w", err)
	}
	return nil
}

func (t *TelegramCodeSender) logSend(start time.Time, err error) {
	if t.log == nil {
		return
	}
	elapsed := time.Since(start)
	if err != nil {
		t.log.Error("telegram reset code send failed", "elapsed_ms", elapsed.Milliseconds(), "error", err)
		return
	}
	t.log.Info("telegram reset code sent", "elapsed_ms", elapsed.Milliseconds())
}

// sendWithDeadline runs fn (a blocking, non-cancellable tgbotapi call) in a
// goroutine and returns as soon as ctx is done, so the retry layer's
// per-attempt timeout is actually enforced. An abandoned goroutine still
// finishes on its own, bounded by the HTTP client's 60s timeout.
func sendWithDeadline(ctx context.Context, fn func() (tgbotapi.Message, error)) (tgbotapi.Message, error) {
	type result struct {
		msg tgbotapi.Message
		err error
	}
	ch := make(chan result, 1)
	go func() {
		m, e := fn()
		ch <- result{msg: m, err: e}
	}()

	select {
	case <-ctx.Done():
		return tgbotapi.Message{}, ctx.Err()
	case r := <-ch:
		return r.msg, r.err
	}
}
