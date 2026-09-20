package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// starOptions are the preset donation amounts (in Telegram Stars) offered by the
// "⭐ Поддержать автора" menu (spec §10).
var starOptions = []int{1, 5, 10, 15, 30, 50, 100}

const (
	donateInvoiceTitle = "Поддержка разработки NowDone"
	donateInvoiceDesc  = "Спасибо, что помогаете развивать бота! Звёзды идут на разработку новых функций."

	// donatePayloadPrefix tags our own invoices so pre-checkout / successful
	// payment updates can be recognised. Full payload: "donate:<stars>".
	donatePayloadPrefix = "donate:"

	// starsCurrency is Telegram's currency code for Stars. Such invoices carry an
	// empty provider token.
	starsCurrency = "XTR"
)

// sendDonateMenu shows the star-amount picker.
func (b *Bot) sendDonateMenu(ctx context.Context, chatID int64) {
	text := "🌟 Поддержите разработку бота!\n\n" +
		"Выберите количество звёзд ⭐, которые вы хотите отправить:"

	var rows [][]tgbotapi.InlineKeyboardButton
	var row []tgbotapi.InlineKeyboardButton
	for i, n := range starOptions {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%d ⭐", n),
			fmt.Sprintf("donate:stars:%d", n),
		))
		// 4 buttons per row stays readable on phones.
		if len(row) == 4 || i == len(starOptions)-1 {
			rows = append(rows, row)
			row = nil
		}
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	if _, err := b.send(ctx, msg); err != nil {
		b.log.Error("send donate menu", "error", err)
	}
}

// handleDonateCallback fires when the user taps a star amount; it sends a
// Telegram Stars invoice (currency "XTR", empty provider token).
func (b *Bot) handleDonateCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	parts := strings.Split(cb.Data, ":")
	if len(parts) != 3 || parts[1] != "stars" {
		b.answerCallback(ctx, cb.ID, "")
		return
	}
	stars, err := strconv.Atoi(parts[2])
	if err != nil || stars <= 0 {
		b.answerCallback(ctx, cb.ID, "Некорректная сумма.")
		return
	}
	b.answerCallback(ctx, cb.ID, "")

	invoice := tgbotapi.NewInvoice(
		cb.Message.Chat.ID,
		donateInvoiceTitle,
		donateInvoiceDesc,
		fmt.Sprintf("%s%d", donatePayloadPrefix, stars), // payload
		"",       // provider token — MUST be empty for Telegram Stars
		"donate", // start parameter
		starsCurrency,
		[]tgbotapi.LabeledPrice{{Label: fmt.Sprintf("%d ⭐", stars), Amount: stars}},
	)
	if _, err := b.send(ctx, invoice); err != nil {
		b.log.Error("send stars invoice", "stars", stars, "error", err)
		b.reply(ctx, cb.Message.Chat.ID, "Не удалось создать счёт на оплату. Попробуйте позже.")
	}
}

// handlePreCheckout must answer every pre-checkout query within ~10 seconds or
// Telegram cancels the payment. We approve any invoice we recognise.
func (b *Bot) handlePreCheckout(ctx context.Context, q *tgbotapi.PreCheckoutQuery) {
	_ = ctx
	ok := strings.HasPrefix(q.InvoicePayload, donatePayloadPrefix)
	cfg := tgbotapi.PreCheckoutConfig{PreCheckoutQueryID: q.ID, OK: ok}
	if !ok {
		cfg.ErrorMessage = "Счёт устарел. Откройте меню поддержки заново."
	}
	// Answered directly, without the retrying b.request wrapper: Telegram voids
	// the payment after ~10s, and retry backoff would blow that budget. The
	// injected http.Client still bounds a stalled call.
	if _, err := b.api.Request(cfg); err != nil {
		b.log.Error("answer pre-checkout", "error", err)
	}
}

// handlePaymentSuccess posts the thank-you message once Stars are received.
func (b *Bot) handlePaymentSuccess(ctx context.Context, msg *tgbotapi.Message) {
	sp := msg.SuccessfulPayment
	b.log.Info("stars payment received",
		"chat_id", msg.Chat.ID,
		"amount", sp.TotalAmount,
		"currency", sp.Currency,
		"payload", sp.InvoicePayload,
	)

	b.reply(ctx, msg.Chat.ID, "❤️ Огромное спасибо за вашу поддержку!\n\n"+
		"Ваши звёзды помогают мне продолжать развивать этот проект, добавлять новые функции "+
		"и делать бота ещё удобнее и полезнее. Каждый ваш вклад — это большая мотивация для меня!\n\n"+
		"Спасибо, что вы со мной! 🚀")
}
