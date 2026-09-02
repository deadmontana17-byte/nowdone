// Package telegram implements the NowDone Telegram bot: deep-link auth,
// AI-powered natural language task management, a reply-keyboard menu, an
// interactive "today" task list, task/note composition with attachments,
// interactive reminders and Telegram Stars donations.
package telegram

import (
	"context"
	"log/slog"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"nowdone/internal/models"
	"nowdone/internal/repository"
	"nowdone/internal/service"
)

// Bot wraps the Telegram Bot API client and its dependencies.
type Bot struct {
	api       *tgbotapi.BotAPI
	auth      *service.AuthService
	tasks     *service.TaskService
	taskTypes *service.TaskTypeService
	notes     *service.NoteService
	users     *repository.UserRepository
	openai    *service.OpenAIService
	s3        *service.S3Service // may be nil if object storage is not configured
	log       *slog.Logger

	// state holds per-chat conversation context (add-task / add-note flows and
	// the id of the re-rendered "today" list message).
	state *stateStore
}

func New(
	api *tgbotapi.BotAPI,
	auth *service.AuthService,
	tasks *service.TaskService,
	taskTypes *service.TaskTypeService,
	notes *service.NoteService,
	users *repository.UserRepository,
	openai *service.OpenAIService,
	s3 *service.S3Service,
	log *slog.Logger,
) *Bot {
	return &Bot{
		api:       api,
		auth:      auth,
		tasks:     tasks,
		taskTypes: taskTypes,
		notes:     notes,
		users:     users,
		openai:    openai,
		s3:        s3,
		log:       log,
		state:     newStateStore(),
	}
}

// Run starts long-polling for updates and blocks until ctx is cancelled.
func (b *Bot) Run(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30

	updates := b.api.GetUpdatesChan(u)
	b.log.Info("telegram bot started", "username", b.api.Self.UserName)

	// Publish the "/" command menu once at startup.
	b.registerCommands()

	for {
		select {
		case <-ctx.Done():
			b.api.StopReceivingUpdates()
			return ctx.Err()
		case update := <-updates:
			go b.handleUpdate(ctx, update)
		}
	}
}

func (b *Bot) handleUpdate(ctx context.Context, update tgbotapi.Update) {
	defer func() {
		if r := recover(); r != nil {
			b.log.Error("panic handling update", "recover", r)
		}
	}()

	switch {
	case update.PreCheckoutQuery != nil:
		// Telegram Stars: must be answered within ~10s or the payment is voided.
		b.handlePreCheckout(ctx, update.PreCheckoutQuery)
	case update.CallbackQuery != nil:
		b.handleCallback(ctx, update.CallbackQuery)
	case update.Message == nil:
		return
	case update.Message.SuccessfulPayment != nil:
		b.handlePaymentSuccess(ctx, update.Message)
	case update.Message.IsCommand():
		b.handleCommand(ctx, update.Message)
	case update.Message.Voice != nil:
		b.handleVoiceMessage(ctx, update.Message)
	case len(update.Message.Photo) > 0 || update.Message.Video != nil || update.Message.Document != nil:
		b.handleMediaMessage(ctx, update.Message)
	case update.Message.Text != "":
		b.handleTextMessage(ctx, update.Message)
	}
}

func (b *Bot) handleCommand(ctx context.Context, msg *tgbotapi.Message) {
	switch msg.Command() {
	case "start":
		b.handleStart(ctx, msg)
	case "today":
		b.resetToIdle(msg.Chat.ID)
		b.showToday(ctx, msg.Chat.ID, msg.From.ID)
	case "tomorrow":
		b.resetToIdle(msg.Chat.ID)
		b.showTomorrow(ctx, msg.Chat.ID, msg.From.ID)
	case "add_task":
		b.beginAdd(msg.Chat.ID, modeAddTask)
	case "add_note":
		b.beginAdd(msg.Chat.ID, modeAddNote)
	case "donate":
		b.resetToIdle(msg.Chat.ID)
		b.sendDonateMenu(msg.Chat.ID)
	case "auth":
		b.reply(msg.Chat.ID, "Чтобы войти, откройте сайт NowDone и нажмите «Войти через Telegram».")
	case "reset":
		b.reply(msg.Chat.ID, "Чтобы сбросить PIN, откройте на сайте страницу входа и нажмите «Забыли PIN?».")
	default:
		b.reply(msg.Chat.ID, "Неизвестная команда. Используйте меню ниже или просто напишите/наговорите задачу 🙂")
	}
}

// greetingText is the welcome message shown on a plain /start: a short intro,
// then a bulleted tour of what the bot can do, then a hint about free-form input.
const greetingText = "👋 *Привет! Это NowDone — твой помощник по задачам.*\n\n" +
	"Я помогу держать дела под контролем: планировать день, не забывать о важном и хранить заметки.\n\n" +
	"*Что я умею:*\n" +
	"📋 Показывать задачи на сегодня и на завтра\n" +
	"➕ Создавать задачи — текстом, голосом или с фото\n" +
	"📝 Сохранять заметки с вложениями\n" +
	"⏰ Напоминать в нужное время («напомни завтра в 10:00»)\n" +
	"✅ Отмечать выполненное и удалять лишнее\n\n" +
	"💡 Можно просто написать или наговорить, что нужно сделать — я сам пойму дату и время.\n\n" +
	"Начни с меню ниже 👇"

// handleStart processes /start, /start auth_{code} and /start reset_{code}.
func (b *Bot) handleStart(ctx context.Context, msg *tgbotapi.Message) {
	arg := msg.CommandArguments()
	if arg == "" {
		// Plain /start: greeting + persistent menu (spec §1, §2).
		b.replyWithMenu(msg.Chat.ID, greetingText)
		return
	}

	var (
		code    string
		purpose models.AuthCodePurpose
	)
	switch {
	case strings.HasPrefix(arg, "auth_"):
		code = strings.TrimPrefix(arg, "auth_")
		purpose = models.PurposeAuth
	case strings.HasPrefix(arg, "reset_"):
		code = strings.TrimPrefix(arg, "reset_")
		purpose = models.PurposeReset
	default:
		b.reply(msg.Chat.ID, "Ссылка недействительна.")
		return
	}

	from := msg.From
	user, err := b.auth.ConfirmCodeFromBot(ctx, code, purpose, from.ID, from.UserName, from.FirstName, msg.Chat.ID)
	if err != nil {
		b.log.Warn("confirm code failed", "error", err)
		b.reply(msg.Chat.ID, "Код недействителен или истёк. Попробуйте получить новый на сайте.")
		return
	}
	_ = user

	verb := "входа"
	if purpose == models.PurposeReset {
		verb = "сброса PIN"
	}
	b.replyWithMenu(msg.Chat.ID, "Ваш код для "+verb+": *"+code+"*\n\nВведите его на сайте NowDone.")
}

// reply sends a Markdown text message.
func (b *Bot) reply(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdown
	if _, err := b.api.Send(msg); err != nil {
		b.log.Error("send message", "error", err)
	}
}

// replyPlain sends text with no parse mode — use it whenever the message
// interpolates user-controlled strings (task/note titles) that could otherwise
// break Markdown parsing.
func (b *Bot) replyPlain(chatID int64, text string) {
	if _, err := b.api.Send(tgbotapi.NewMessage(chatID, text)); err != nil {
		b.log.Error("send message", "error", err)
	}
}

// replyWithMenu sends text and (re)shows the persistent reply keyboard. Used at
// the end of a flow to bring the user back to the main menu.
func (b *Bot) replyWithMenu(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.ReplyMarkup = mainMenuKeyboard()
	if _, err := b.api.Send(msg); err != nil {
		b.log.Error("send message", "error", err)
	}
}

// deleteMessage best-effort removes a message; a failure (already deleted, too
// old) is only logged at debug level.
func (b *Bot) deleteMessage(chatID int64, msgID int) {
	if msgID == 0 {
		return
	}
	if _, err := b.api.Request(tgbotapi.NewDeleteMessage(chatID, msgID)); err != nil {
		b.log.Debug("delete message", "chat_id", chatID, "msg_id", msgID, "error", err)
	}
}
