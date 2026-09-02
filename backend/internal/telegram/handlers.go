package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"

	"nowdone/internal/models"
	"nowdone/internal/repository"
	"nowdone/internal/service"
)

// ==========================================================================
// Incoming messages
// ==========================================================================

// handleTextMessage routes a plain text message: menu buttons first, then the
// active add-task / add-note flow, then the free-form OpenAI intent parser.
func (b *Bot) handleTextMessage(ctx context.Context, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	text := strings.TrimSpace(msg.Text)

	switch text {
	case btnToday:
		b.resetToIdle(chatID)
		b.showToday(ctx, chatID, msg.From.ID)
		return
	case btnTomorrow:
		b.resetToIdle(chatID)
		b.showTomorrow(ctx, chatID, msg.From.ID)
		return
	case btnAddTask:
		b.beginAdd(chatID, modeAddTask)
		return
	case btnAddNote:
		b.beginAdd(chatID, modeAddNote)
		return
	case btnDonate:
		b.resetToIdle(chatID)
		b.sendDonateMenu(chatID)
		return
	case btnCancel:
		b.cancelAdd(chatID)
		return
	}

	if m := b.mode(chatID); m == modeAddTask || m == modeAddNote {
		// Text while composing: use it as title/description and finalize now —
		// a standalone text message is not part of an album.
		b.stageDraft(ctx, chatID, msg.From.ID, text, nil)
		b.finalizeDraft(ctx, chatID, msg.From.ID)
		return
	}

	// Idle: natural-language intent pipeline (unchanged behaviour).
	b.processNaturalLanguage(ctx, chatID, msg.From.ID, msg.Text)
}

// handleVoiceMessage downloads the voice note, transcribes it via Whisper, then
// either feeds it into the active add flow or through the intent pipeline.
func (b *Bot) handleVoiceMessage(ctx context.Context, msg *tgbotapi.Message) {
	fileURL, err := b.api.GetFileDirectURL(msg.Voice.FileID)
	if err != nil {
		b.log.Error("get voice file url", "error", err)
		b.reply(msg.Chat.ID, "Не удалось загрузить голосовое сообщение.")
		return
	}

	resp, err := http.Get(fileURL)
	if err != nil {
		b.log.Error("download voice file", "error", err)
		b.reply(msg.Chat.ID, "Не удалось загрузить голосовое сообщение.")
		return
	}
	defer resp.Body.Close()

	audioBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		b.log.Error("read voice file", "error", err)
		b.reply(msg.Chat.ID, "Не удалось прочитать голосовое сообщение.")
		return
	}

	transcript, err := b.openai.TranscribeVoice(ctx, audioBytes, "voice.ogg")
	if err != nil {
		b.log.Error("transcribe voice", "error", err)
		b.reply(msg.Chat.ID, "Не удалось распознать голосовое сообщение.")
		return
	}

	if m := b.mode(msg.Chat.ID); m == modeAddTask || m == modeAddNote {
		b.stageDraft(ctx, msg.Chat.ID, msg.From.ID, transcript, nil)
		b.finalizeDraft(ctx, msg.Chat.ID, msg.From.ID)
		return
	}
	b.processNaturalLanguage(ctx, msg.Chat.ID, msg.From.ID, transcript)
}

// handleMediaMessage accepts a photo / video / document while an add flow is
// active, uploads it to S3, and stages it on the draft. Album items arrive as
// several of these; the debounce timer in stageDraft collects them.
func (b *Bot) handleMediaMessage(ctx context.Context, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID

	if m := b.mode(chatID); m != modeAddTask && m != modeAddNote {
		b.reply(chatID, "Чтобы прикрепить файл, сначала выберите «➕ Добавить задачу» или «📝 Добавить заметку».")
		return
	}
	if b.s3 == nil {
		b.reply(chatID, "Хранилище файлов не настроено — вложение не сохранено.")
		return
	}

	att, err := b.ingestAttachment(ctx, msg)
	if err != nil {
		b.log.Error("ingest attachment", "chat_id", chatID, "error", err)
		b.reply(chatID, "Не удалось сохранить вложение. Попробуйте ещё раз.")
		return
	}

	b.stageDraft(ctx, chatID, msg.From.ID, strings.TrimSpace(msg.Caption), &att)

	// Acknowledge single-file uploads; stay quiet for albums to avoid spam.
	if msg.MediaGroupID == "" {
		b.replyPlain(chatID, "📎 Вложение добавлено. Отправьте текст (заголовок) или нажмите «Готово».")
	}
}

// ingestAttachment downloads the file behind a Telegram message and stores it in
// S3, returning the models.Attachment to persist on the task/note.
func (b *Bot) ingestAttachment(ctx context.Context, msg *tgbotapi.Message) (models.Attachment, error) {
	var fileID, name, kind string
	switch {
	case len(msg.Photo) > 0:
		fileID = msg.Photo[len(msg.Photo)-1].FileID // last entry is the largest size
		name, kind = "photo.jpg", "image"
	case msg.Video != nil:
		fileID, name, kind = msg.Video.FileID, "video.mp4", "video"
	case msg.Document != nil:
		fileID = msg.Document.FileID
		name = msg.Document.FileName
		if name == "" {
			name = "file"
		}
		kind = "file"
	default:
		return models.Attachment{}, fmt.Errorf("message carries no supported attachment")
	}

	directURL, err := b.api.GetFileDirectURL(fileID)
	if err != nil {
		return models.Attachment{}, fmt.Errorf("get file url: %w", err)
	}
	// Photos/videos have no extension in their logical name; borrow it from the
	// Telegram storage path so S3 stores a sensible content type.
	if filepath.Ext(name) == "" {
		if ext := filepath.Ext(strings.SplitN(directURL, "?", 2)[0]); ext != "" {
			name += ext
		}
	}

	resp, err := http.Get(directURL)
	if err != nil {
		return models.Attachment{}, fmt.Errorf("download file: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return models.Attachment{}, fmt.Errorf("download file: telegram returned %d", resp.StatusCode)
	}

	url, err := b.s3.Upload(ctx, name, resp.Body)
	if err != nil {
		return models.Attachment{}, fmt.Errorf("upload to s3: %w", err)
	}
	return models.Attachment{Type: kind, URL: url, Name: name}, nil
}

// ==========================================================================
// Conversation state helpers
// ==========================================================================

func (b *Bot) mode(chatID int64) convMode {
	var m convMode
	b.state.withLock(chatID, func(st *chatState) { m = st.mode })
	return m
}

func (b *Bot) resetToIdle(chatID int64) {
	b.state.withLock(chatID, func(st *chatState) {
		if st.dr != nil && st.dr.flush != nil {
			st.dr.flush.Stop()
		}
		st.dr = nil
		st.mode = modeIdle
	})
}

// beginAdd enters the add-task / add-note flow and shows the compose keyboard.
func (b *Bot) beginAdd(chatID int64, m convMode) {
	b.state.withLock(chatID, func(st *chatState) {
		if st.dr != nil && st.dr.flush != nil {
			st.dr.flush.Stop()
		}
		st.mode = m
		st.dr = &draft{}
	})

	var prompt string
	if m == modeAddTask {
		prompt = "✍️ *Создание задачи*\n\n" +
			"Просто отправьте текст с заголовком и описанием " +
			"(первая строка — заголовок, остальное — описание). " +
			"Можно также прикрепить фото или видео, или отправить голосовое сообщение.\n\n" +
			"Вы также можете отправить голосовое сообщение — я распознаю текст и создам задачу.\n\n" +
			"Относительную дату в тексте (например «завтра») я распознаю сам."
	} else {
		prompt = "✍️ *Создание заметки*\n\n" +
			"Просто отправьте текст с заголовком и описанием " +
			"(первая строка — заголовок, остальное — описание). " +
			"Можно также прикрепить фото, видео или файл.\n\n" +
			"Вы также можете отправить голосовое сообщение — я распознаю текст и создам заметку."
	}

	msg := tgbotapi.NewMessage(chatID, prompt)
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.ReplyMarkup = draftKeyboard()
	if _, err := b.api.Send(msg); err != nil {
		b.log.Error("send add prompt", "error", err)
	}
}

func (b *Bot) cancelAdd(chatID int64) {
	b.resetToIdle(chatID)
	b.replyWithMenu(chatID, "Отменено.")
}

// stageDraft applies an optional text and/or attachment to the active draft and
// (re)arms the debounce timer that finalizes it.
func (b *Bot) stageDraft(ctx context.Context, chatID, telegramID int64, text string, att *models.Attachment) {
	b.state.withLock(chatID, func(st *chatState) {
		if st.mode != modeAddTask && st.mode != modeAddNote {
			return
		}
		if st.dr == nil {
			st.dr = &draft{}
		}
		if text != "" && !st.dr.hasText {
			applyDraftText(st.dr, text)
		}
		if att != nil {
			st.dr.attachments = append(st.dr.attachments, *att)
		}
		if st.dr.flush != nil {
			st.dr.flush.Stop()
		}
		st.dr.flush = time.AfterFunc(draftFlushDelay, func() {
			b.finalizeDraft(ctx, chatID, telegramID)
		})
	})
}

// applyDraftText stores the verbatim message on the draft and fills in a
// PROVISIONAL title/description so finalizeDraft can tell an empty draft from a
// non-empty one and has something to fall back on. The authoritative parsing —
// markers, OpenAI field extraction, date/reminder resolution, reminder-phrase
// stripping — happens later in composeTask / composeNote, so a voice message
// sent through the "add task" button goes through exactly the same pipeline as
// one typed in idle chat.
func applyDraftText(d *draft, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	d.raw = text

	if pf := parseMarkedFields(text); pf.found() {
		d.title, d.description = strings.TrimSpace(pf.title), strings.TrimSpace(pf.description)
	} else {
		d.title, d.description = splitFirstLine(text)
	}
	d.hasText = true
}

// splitFirstLine is the "no markers, no OpenAI" fallback: the first line is the
// title, everything after it the description.
func splitFirstLine(text string) (title, description string) {
	text = strings.TrimSpace(text)
	parts := strings.SplitN(text, "\n", 2)
	title = strings.TrimSpace(parts[0])
	if len(parts) > 1 {
		description = strings.TrimSpace(parts[1])
	}
	return title, description
}

// finalizeDraft creates the task or note from the accumulated draft. It is safe
// to call more than once (timer + explicit "Готово"): the first call clears the
// draft, the rest are no-ops.
func (b *Bot) finalizeDraft(ctx context.Context, chatID, telegramID int64) {
	var (
		mode convMode
		d    draft
		ok   bool
	)
	b.state.withLock(chatID, func(st *chatState) {
		if (st.mode != modeAddTask && st.mode != modeAddNote) || st.dr == nil {
			return
		}
		if st.dr.flush != nil {
			st.dr.flush.Stop()
			st.dr.flush = nil
		}
		mode, d, ok = st.mode, *st.dr, true
		st.dr = nil
		st.mode = modeIdle
	})
	if !ok {
		return
	}

	user, err := b.users.GetByTelegramID(ctx, telegramID)
	if err != nil {
		b.replyWithMenu(chatID, "Сначала войдите через сайт NowDone, чтобы я знал, кому сохранять записи.")
		return
	}

	if strings.TrimSpace(d.title) == "" && len(d.attachments) == 0 {
		b.replyWithMenu(chatID, "Пустое сообщение — ничего не создал.")
		return
	}
	if strings.TrimSpace(d.title) == "" {
		d.title = "Без названия"
	}

	if mode == modeAddNote {
		b.createNoteFromDraft(ctx, chatID, user, d)
		return
	}
	b.createTaskFromDraft(ctx, chatID, user, d)
}

// createTaskFromDraft is the thin adapter between the "add task" button flow
// (which collects text + album attachments into a draft) and the shared
// creation pipeline. It runs composeTask on the verbatim message — the SAME
// parsing an idle-chat message gets — then hands the result plus the draft's
// attachments to createTask.
func (b *Bot) createTaskFromDraft(ctx context.Context, chatID int64, user *models.User, d draft) {
	var c composedTask
	if strings.TrimSpace(d.raw) != "" {
		c = b.composeTask(ctx, d.raw, user, nil)
	} else {
		// Attachment-only draft: nothing to parse, keep the provisional fields.
		c = composedTask{title: d.title, description: d.description}
	}
	if strings.TrimSpace(c.title) == "" {
		c.title = d.title // fallback provisional title set by finalizeDraft
	}
	b.createTask(ctx, chatID, user, c, d.attachments)
}

// reminderSuffix is the "\n⏰ Напоминание: …" line appended to a confirmation,
// or "" when the task has no reminder.
func reminderSuffix(reminder *time.Time, loc *time.Location) string {
	if reminder == nil {
		return ""
	}
	return fmt.Sprintf("\n⏰ Напоминание: %s", reminder.In(loc).Format("02.01.2006 15:04"))
}

// createNoteFromDraft mirrors createTaskFromDraft for notes. It shares the
// marker / first-line parsing (composeNote) but NOT the schedule pipeline: a
// note has no date and no reminder, so composeTask / resolveSchedule are never
// called here.
func (b *Bot) createNoteFromDraft(ctx context.Context, chatID int64, user *models.User, d draft) {
	title, description := d.title, d.description
	if strings.TrimSpace(d.raw) != "" {
		title, description = composeNote(d.raw)
	}
	if strings.TrimSpace(title) == "" {
		title = d.title
	}
	if strings.TrimSpace(title) == "" {
		title = "Без названия"
	}

	content := models.JSONRaw("{}")
	if description != "" {
		content = models.JSONRaw(fmt.Sprintf(`{"text":%q}`, description))
	}

	note := &models.Note{
		UserID:      user.ID,
		Title:       title,
		Content:     content,
		Attachments: d.attachments,
	}
	created, err := b.notes.Create(ctx, note)
	if err != nil {
		b.log.Error("create note from draft", "error", err)
		b.replyWithMenu(chatID, "Не удалось сохранить заметку. Попробуйте ещё раз.")
		return
	}

	b.replyWithMenu(chatID, "✅ Заметка сохранена.")
	b.sendNoteCard(chatID, created)
}

// resolveSchedule asks the OpenAI intent parser to resolve, from a free-form
// message, both an absolute date ("завтра", "в пятницу", …) and a reminder time
// ("напомни в 15:00", "с напоминанием завтра в 10 утра", …).
//
// It returns fallbackDate when no date is stated and a nil reminder when none
// is. A single ParseIntent call covers both; on any error the fallback date and
// a nil reminder are returned so task creation still succeeds.
func (b *Bot) resolveSchedule(ctx context.Context, text string, user *models.User, fallbackDate time.Time) (date time.Time, reminder *time.Time) {
	loc := user.Location()
	date = fallbackDate

	nowRef := time.Now().In(loc).Format("2006-01-02T15:04")
	intent, err := b.openai.ParseIntent(ctx, text, nowRef)
	if err != nil {
		b.log.Warn("resolve schedule: parse intent", "error", err)
		return date, nil
	}
	if intent == nil {
		return date, nil
	}

	if intent.Date != "" {
		if parsed, e := time.Parse("2006-01-02", intent.Date); e == nil {
			date = time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, loc)
		}
	}
	return date, parseModelReminder(intent.ReminderTime, loc)
}

// parseModelReminder turns the model's "YYYY-MM-DDTHH:MM" local wall-clock value
// into a concrete instant in loc. Returns nil for an empty/"none" value, an
// unparseable value, or one that is in the past / absurdly far in the future.
func parseModelReminder(raw string, loc *time.Location) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "none") || strings.EqualFold(raw, "null") {
		return nil
	}

	var when time.Time
	var ok bool
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02T15:04:05", "2006-01-02 15:04"} {
		if t, err := time.ParseInLocation(layout, raw, loc); err == nil {
			when, ok = t, true
			break
		}
	}
	if !ok {
		// A value that already carries an offset (rare, but the model may do it).
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			when, ok = t, true
		}
	}
	if !ok {
		return nil
	}

	now := time.Now()
	if when.Before(now.Add(-2*time.Minute)) || when.After(now.AddDate(2, 0, 0)) {
		return nil
	}
	return &when
}

// ==========================================================================
// Unified message → record pipeline (used by every entry point)
// ==========================================================================
//
// There used to be two divergent code paths:
//
//   - idle chat        : processNaturalLanguage → parseMarkedFields /
//                        handleCreateIntent (full OpenAI field extraction,
//                        stripReminderPhrase, …)
//   - "add task" button: stageDraft → applyDraftText (first-line split only) →
//                        createTaskFromDraft (date/reminder only)
//
// so a voice note sent through the button missed OpenAI title/description
// splitting and every fix that lived in handleCreateIntent. Both now funnel
// through composeTask + createTask, so the parsing is identical no matter how
// the message arrived (typed or voice, with or without the button).

// composedTask is the fully-resolved set of fields for a new task, independent
// of how the originating message was delivered.
type composedTask struct {
	title       string
	description string
	date        time.Time  // zero → caller substitutes "today"
	reminder    *time.Time // nil → no reminder
}

// composeTask turns one free-form message (typed or transcribed) into task
// fields, applying — in this order — every rule that previously only ran on the
// idle-chat path:
//
//  1. explicit «Заголовок»/«Описание» markers win (parseMarkedFields); the date
//     and reminder are then resolved from the whole message (resolveSchedule).
//  2. otherwise OpenAI ParseIntent splits title/description and returns the date
//     and reminder in the same call (old handleCreateIntent behaviour).
//  3. failing both, the first line is the title and the rest the description.
//
// In branches 1 and 2, once a reminder time is known the phrase that produced it
// is removed from the description (stripReminderPhrase) so it is not duplicated.
//
// preParsed is an Intent the caller already fetched (processNaturalLanguage
// needs one to choose the action); passing it avoids a second OpenAI round-trip.
func (b *Bot) composeTask(ctx context.Context, raw string, user *models.User, preParsed *service.Intent) composedTask {
	loc := user.Location()
	today := dateOnly(time.Now().In(loc))
	raw = strings.TrimSpace(raw)

	// 1. Hand-split with markers.
	if pf := parseMarkedFields(raw); pf.found() {
		date, reminder := b.resolveSchedule(ctx, raw, user, today)
		desc := strings.TrimSpace(pf.description)
		if reminder != nil {
			desc = stripReminderPhrase(desc)
		}
		return composedTask{title: strings.TrimSpace(pf.title), description: desc, date: date, reminder: reminder}
	}

	// 2. OpenAI field extraction (reusing preParsed when the caller has it).
	intent := preParsed
	if intent == nil {
		nowRef := time.Now().In(loc).Format("2006-01-02T15:04")
		if got, err := b.openai.ParseIntent(ctx, raw, nowRef); err != nil {
			b.log.Warn("compose task: parse intent", "error", err)
		} else {
			intent = got
		}
	}
	if intent != nil && strings.TrimSpace(intent.Title) != "" {
		date := today
		if intent.Date != "" {
			if parsed, err := time.Parse("2006-01-02", intent.Date); err == nil {
				date = time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, loc)
			}
		}
		reminder := parseModelReminder(intent.ReminderTime, loc)
		desc := strings.TrimSpace(intent.Description)
		if reminder != nil {
			desc = stripReminderPhrase(desc)
		}
		return composedTask{title: strings.TrimSpace(intent.Title), description: desc, date: date, reminder: reminder}
	}

	// 3. Plain first-line split.
	title, desc := splitFirstLine(raw)
	return composedTask{title: title, description: desc, date: today}
}

// composeNote is composeTask's counterpart for notes: markers are honoured, but
// a note is timeless so no date/reminder is resolved and OpenAI is not called.
func composeNote(raw string) (title, description string) {
	raw = strings.TrimSpace(raw)
	if pf := parseMarkedFields(raw); pf.found() {
		return strings.TrimSpace(pf.title), strings.TrimSpace(pf.description)
	}
	return splitFirstLine(raw)
}

// createTask is the single persistence + reply sink for task creation. Every
// path (idle markers, idle OpenAI "create" intent, the "add task" button) ends
// here, so the confirmation, the task card and the list refresh are identical.
func (b *Bot) createTask(ctx context.Context, chatID int64, user *models.User, c composedTask, attachments []models.Attachment) {
	loc := user.Location()

	title := strings.TrimSpace(c.title)
	if title == "" {
		title = "Без названия"
	}
	date := c.date
	if date.IsZero() {
		date = dateOnly(time.Now().In(loc))
	}

	desc := models.JSONRaw("{}")
	if c.description != "" {
		desc = models.JSONRaw(fmt.Sprintf(`{"text":%q}`, c.description))
	}

	task := &models.Task{
		UserID:       user.ID,
		Title:        title,
		Description:  desc,
		Attachments:  attachments,
		Date:         models.NewDate(date),
		ReminderTime: c.reminder,
	}
	created, err := b.tasks.Create(ctx, task)
	if err != nil {
		b.log.Error("create task", "error", err)
		b.replyWithMenu(chatID, "Не удалось создать задачу. Попробуйте ещё раз.")
		return
	}

	b.replyWithMenu(chatID, fmt.Sprintf("✅ Задача добавлена на %s.%s",
		date.Format("02.01.2006"), reminderSuffix(c.reminder, loc)))
	b.sendTaskCard(chatID, loc, created)
	// Keep whatever task list is currently on screen (today or tomorrow) in sync.
	b.refreshTaskList(ctx, chatID, user)
}

// ==========================================================================
// Task list — "сегодня" / "завтра" (spec §3)
// ==========================================================================

func (b *Bot) showToday(ctx context.Context, chatID, telegramID int64) {
	b.showRelativeDay(ctx, chatID, telegramID, 0)
}

func (b *Bot) showTomorrow(ctx context.Context, chatID, telegramID int64) {
	b.showRelativeDay(ctx, chatID, telegramID, 1)
}

// showRelativeDay renders the task list for today (+0) or tomorrow (+1),
// relative to the current date in the user's timezone.
func (b *Bot) showRelativeDay(ctx context.Context, chatID, telegramID int64, offsetDays int) {
	user, err := b.users.GetByTelegramID(ctx, telegramID)
	if err != nil {
		b.replyWithMenu(chatID, "Сначала войдите через сайт NowDone.")
		return
	}
	date := dateOnly(time.Now().In(user.Location())).AddDate(0, 0, offsetDays)
	b.renderTaskList(ctx, chatID, user, date, false)
}

// refreshTaskList re-renders the tracked list message in place after a task
// mutation, for the day it currently shows. No-op if no list is on screen.
func (b *Bot) refreshTaskList(ctx context.Context, chatID int64, user *models.User) {
	var (
		has  bool
		date time.Time
	)
	b.state.withLock(chatID, func(st *chatState) {
		has = st.listMsgID != 0
		date = st.listDate
	})
	if !has {
		return
	}
	b.renderTaskList(ctx, chatID, user, date, true)
}

// renderTaskList sends or updates the single task-list message for the given
// day. When inPlace is true it edits the tracked message; otherwise it drops the
// old one and posts a fresh list, so copies never stack up. The day it shows is
// remembered in chatState so mutations refresh the right list.
func (b *Bot) renderTaskList(ctx context.Context, chatID int64, user *models.User, date time.Time, inPlace bool) {
	date = dateOnly(date)

	tasks, err := b.tasks.ListRange(ctx, user.ID, date, date)
	if err != nil {
		b.log.Error("list tasks for day", "date", date.Format("2006-01-02"), "error", err)
		b.replyWithMenu(chatID, "Не удалось загрузить задачи.")
		return
	}

	text := "📋 *" + dayLabel(date, user.Location()) + "* · " + date.Format("02.01.2006")
	if len(tasks) == 0 {
		text += "\n\nНа этот день задач нет. Добавьте задачу через меню «➕ Добавить задачу»."
	} else {
		done := 0
		for _, t := range tasks {
			if t.IsDone {
				done++
			}
		}
		text += fmt.Sprintf("\n\nВыполнено: %d из %d\nНажмите на название задачи, чтобы открыть карточку.", done, len(tasks))
	}
	markup := taskListKeyboard(tasks)

	var prevID int
	b.state.withLock(chatID, func(st *chatState) { prevID = st.listMsgID })

	if prevID != 0 {
		if inPlace {
			edit := tgbotapi.NewEditMessageTextAndMarkup(chatID, prevID, text, markup)
			edit.ParseMode = tgbotapi.ModeMarkdown
			if _, err := b.api.Send(edit); err == nil || strings.Contains(err.Error(), "not modified") {
				b.state.withLock(chatID, func(st *chatState) { st.listDate = date })
				return
			}
			// Old message is gone / too old — fall through to a fresh post.
		}
		b.deleteMessage(chatID, prevID)
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.ReplyMarkup = markup
	sent, err := b.api.Send(msg)
	if err != nil {
		b.log.Error("send task list", "error", err)
		return
	}
	b.state.withLock(chatID, func(st *chatState) {
		st.listMsgID = sent.MessageID
		st.listDate = date
	})
}

// dayLabel names a date relative to "now" in loc: "Задачи на сегодня" /
// "Задачи на завтра" / "Задачи на DD.MM.YYYY".
func dayLabel(date time.Time, loc *time.Location) string {
	today := dateOnly(time.Now().In(loc))
	switch {
	case sameDate(date, today):
		return "Задачи на сегодня"
	case sameDate(date, today.AddDate(0, 0, 1)):
		return "Задачи на завтра"
	default:
		return "Задачи на " + date.Format("02.01.2006")
	}
}

// ==========================================================================
// Task / note card (spec §4, §1)
// ==========================================================================

// tgCaptionLimit is a safe cap (in runes) for a media caption; Telegram's hard
// limit is 1024 UTF-16 units, so we stay comfortably under it.
const tgCaptionLimit = 1000

// sendTaskCard shows a task. When the task has an image (or video) attachment
// the card is sent as that media with the card text as caption (Problem 1);
// otherwise it is a plain text message. Any remaining attachments are listed as
// links. loc is the viewer's timezone, used to render the reminder time.
func (b *Bot) sendTaskCard(chatID int64, loc *time.Location, task *models.Task) {
	markup := taskCardKeyboard(task.ID, task.IsDone)

	if url, kind := primaryMedia(task.Attachments); url != "" {
		caption := truncateRunes(renderTaskCardText(task, loc, url), tgCaptionLimit)
		// Fallback text keeps the primary file as a link, so a card is never left
		// with neither the media nor a link to it (Problem 2).
		fallback := truncateRunes(renderTaskCardText(task, loc, ""), tgCaptionLimit)
		b.sendMediaCard(chatID, url, kind, caption, fallback, &markup)
		return
	}

	msg := tgbotapi.NewMessage(chatID, renderTaskCardText(task, loc, ""))
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.DisableWebPagePreview = true
	msg.ReplyMarkup = markup
	if _, err := b.api.Send(msg); err != nil {
		b.log.Error("send task card", "error", err)
	}
}

// renderTaskCardText builds the card body. skipURL, when set, is the attachment
// shown inline as media and therefore omitted from the "Вложения" links list.
func renderTaskCardText(task *models.Task, loc *time.Location, skipURL string) string {
	var sb strings.Builder
	sb.WriteString("*" + mdEscape(task.Title) + "*\n")
	sb.WriteString("🗓 " + task.Date.Time.Format("02.01.2006") + "\n")
	if task.IsDone {
		sb.WriteString("Статус: ✅ выполнено\n")
	} else {
		sb.WriteString("Статус: ⬜ не выполнено\n")
	}
	if task.ReminderTime != nil {
		when := *task.ReminderTime
		if loc != nil {
			when = when.In(loc)
		}
		sb.WriteString("⏰ Напоминание: " + when.Format("02.01.2006 15:04") + "\n")
	}
	if desc := plainText(task.Description); desc != "" {
		sb.WriteString("\n" + mdEscape(desc) + "\n")
	}
	sb.WriteString(attachmentsBlock(task.Attachments, skipURL))
	return sb.String()
}

// sendNoteCard is the note equivalent of sendTaskCard (no inline keyboard).
func (b *Bot) sendNoteCard(chatID int64, note *models.Note) {
	if url, kind := primaryMedia(note.Attachments); url != "" {
		caption := truncateRunes(renderNoteCardText(note, url), tgCaptionLimit)
		fallback := truncateRunes(renderNoteCardText(note, ""), tgCaptionLimit)
		b.sendMediaCard(chatID, url, kind, caption, fallback, nil)
		return
	}

	msg := tgbotapi.NewMessage(chatID, renderNoteCardText(note, ""))
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.DisableWebPagePreview = true
	if _, err := b.api.Send(msg); err != nil {
		b.log.Error("send note card", "error", err)
	}
}

func renderNoteCardText(note *models.Note, skipURL string) string {
	var sb strings.Builder
	sb.WriteString("📝 *" + mdEscape(note.Title) + "*\n")
	if txt := plainText(note.Content); txt != "" {
		sb.WriteString("\n" + mdEscape(txt) + "\n")
	}
	sb.WriteString(attachmentsBlock(note.Attachments, skipURL))
	return sb.String()
}

// sendMediaCard sends a card as a photo or video message: the card text goes in
// the caption and the inline keyboard (if any) is attached. On any Telegram
// rejection (bad URL, private bucket, unsupported file) it falls back to a plain
// text message built from fallbackText — which, unlike caption, still lists the
// primary file as a link, so the user is never left with neither the image nor a
// link to it (Problem 2).
func (b *Bot) sendMediaCard(chatID int64, url, kind, caption, fallbackText string, markup *tgbotapi.InlineKeyboardMarkup) {
	var media tgbotapi.Chattable
	if kind == "video" {
		v := tgbotapi.NewVideo(chatID, tgbotapi.FileURL(url))
		v.Caption = caption
		v.ParseMode = tgbotapi.ModeMarkdown
		if markup != nil {
			v.ReplyMarkup = *markup
		}
		media = v
	} else { // "image"
		p := tgbotapi.NewPhoto(chatID, tgbotapi.FileURL(url))
		p.Caption = caption
		p.ParseMode = tgbotapi.ModeMarkdown
		if markup != nil {
			p.ReplyMarkup = *markup
		}
		media = p
	}

	if _, err := b.api.Send(media); err != nil {
		b.log.Error("send media card, falling back to text", "kind", kind, "url", url, "error", err)
		text := fallbackText
		if strings.TrimSpace(text) == "" {
			text = caption
		}
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = tgbotapi.ModeMarkdown
		msg.DisableWebPagePreview = true
		if markup != nil {
			msg.ReplyMarkup = *markup
		}
		if _, err := b.api.Send(msg); err != nil {
			b.log.Error("send media card text fallback", "error", err)
		}
	}
}

// primaryMedia picks the attachment to render inline in a card: the first image,
// then the first video. The URL is normalised to an absolute https URL so
// Telegram can fetch it (Problem 2). Returns "", "" when there is nothing to
// inline.
func primaryMedia(atts []models.Attachment) (url, kind string) {
	for _, a := range atts {
		if a.URL != "" && attachmentIsImage(a) {
			return ensureAbsoluteURL(a.URL), "image"
		}
	}
	for _, a := range atts {
		if a.URL != "" && attachmentIsVideo(a) {
			return ensureAbsoluteURL(a.URL), "video"
		}
	}
	return "", ""
}

// ensureAbsoluteURL upgrades a stored attachment URL to an absolute https URL.
// A misconfigured S3_PUBLIC_BASE_URL can yield protocol-relative ("//host/key")
// or scheme-less ("host/bucket/key") values that Telegram rejects as media and
// that render as broken Markdown links; this makes them usable again.
func ensureAbsoluteURL(raw string) string {
	raw = strings.TrimSpace(raw)
	switch {
	case raw == "":
		return ""
	case strings.HasPrefix(raw, "http://"), strings.HasPrefix(raw, "https://"):
		return raw
	case strings.HasPrefix(raw, "//"):
		return "https:" + raw
	default:
		return "https://" + strings.TrimLeft(raw, "/")
	}
}

// attachmentIsImage / attachmentIsVideo classify an attachment whether Type
// holds the normalised category ("image"), a raw MIME type ("image/jpeg"), or
// nothing useful — in which case the file extension in the URL/name is used.
func attachmentIsImage(a models.Attachment) bool {
	t := strings.ToLower(strings.TrimSpace(a.Type))
	if t == "image" || t == "photo" || t == "img" || strings.HasPrefix(t, "image/") {
		return true
	}
	return hasExt(a.URL, imageExts) || hasExt(a.Name, imageExts)
}

func attachmentIsVideo(a models.Attachment) bool {
	t := strings.ToLower(strings.TrimSpace(a.Type))
	if t == "video" || strings.HasPrefix(t, "video/") {
		return true
	}
	return hasExt(a.URL, videoExts) || hasExt(a.Name, videoExts)
}

var (
	imageExts = []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".heic", ".heif"}
	videoExts = []string{".mp4", ".mov", ".webm", ".m4v", ".avi", ".mkv"}
)

// hasExt reports whether s (a URL or filename) ends — ignoring any query string
// or fragment — with one of exts.
func hasExt(s string, exts []string) bool {
	s = strings.ToLower(s)
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}
	for _, e := range exts {
		if strings.HasSuffix(s, e) {
			return true
		}
	}
	return false
}

// attachmentsBlock renders the "📎 Вложения" section as Markdown links (spec §8),
// skipping skipURL (the file already shown inline as media).
func attachmentsBlock(atts []models.Attachment, skipURL string) string {
	var lines []string
	for _, a := range atts {
		url := ensureAbsoluteURL(a.URL)
		if url == "" || url == skipURL {
			continue
		}
		name := a.Name
		if name == "" {
			name = "файл"
		}
		lines = append(lines, fmt.Sprintf("• [%s](%s)", mdEscape(name), url))
	}
	if len(lines) == 0 {
		return ""
	}
	return "\n📎 *Вложения:*\n" + strings.Join(lines, "\n") + "\n"
}

// showBackToList re-renders the task list after "⬅️ Назад к задачам": the day it
// last showed, or today if it was never opened in this chat.
func (b *Bot) showBackToList(ctx context.Context, chatID int64, user *models.User) {
	var (
		has  bool
		date time.Time
	)
	b.state.withLock(chatID, func(st *chatState) {
		has = st.listMsgID != 0
		date = st.listDate
	})
	if !has {
		date = dateOnly(time.Now().In(user.Location()))
	}
	b.renderTaskList(ctx, chatID, user, date, true)
}

// ==========================================================================
// Callback router
// ==========================================================================

// handleCallback dispatches every inline-button press by its data namespace.
func (b *Bot) handleCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	data := cb.Data
	switch {
	case data == "list:refresh":
		b.answerCallback(cb.ID, "")
		if user, err := b.users.GetByTelegramID(ctx, cb.From.ID); err == nil {
			b.refreshTaskList(ctx, cb.Message.Chat.ID, user)
		}
	case strings.HasPrefix(data, "task:"):
		b.handleTaskListCallback(ctx, cb)
	case strings.HasPrefix(data, "card:"):
		b.handleTaskCardCallback(ctx, cb)
	case strings.HasPrefix(data, "remind:"):
		b.handleReminderCallback(ctx, cb)
	case strings.HasPrefix(data, "donate:"):
		b.handleDonateCallback(ctx, cb)
	case strings.HasPrefix(data, "done:"), strings.HasPrefix(data, "snooze:"):
		// Legacy reminder buttons from messages sent before the "remind:" rename.
		b.handleLegacyReminderCallback(ctx, cb)
	default:
		b.answerCallback(cb.ID, "")
	}
}

// handleTaskListCallback handles the three per-task buttons in the "today" list.
func (b *Bot) handleTaskListCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	_, action, id, ok := parseCallback(cb.Data)
	if !ok || id == uuid.Nil {
		b.answerCallback(cb.ID, "")
		return
	}
	chatID := cb.Message.Chat.ID

	user, err := b.users.GetByTelegramID(ctx, cb.From.ID)
	if err != nil {
		b.answerCallback(cb.ID, "Вы не авторизованы.")
		return
	}

	switch action {
	case "open":
		b.answerCallback(cb.ID, "")
		task, err := b.tasks.Get(ctx, user.ID, id)
		if err != nil {
			b.replyPlain(chatID, "Задача не найдена — возможно, она уже удалена.")
			b.refreshTaskList(ctx, chatID, user)
			return
		}
		b.sendTaskCard(chatID, user.Location(), task)

	case "toggle":
		task, err := b.tasks.Get(ctx, user.ID, id)
		if err != nil {
			b.answerCallback(cb.ID, "Задача не найдена.")
			b.refreshTaskList(ctx, chatID, user)
			return
		}
		done := !task.IsDone
		if _, err := b.tasks.Update(ctx, user.ID, id, repository.TaskUpdate{IsDone: &done}); err != nil {
			b.log.Error("toggle task from list", "task_id", id, "error", err)
			b.answerCallback(cb.ID, "Не удалось обновить статус.")
			return
		}
		if done {
			b.answerCallback(cb.ID, "Отмечено выполненной ✅")
			b.replyPlain(chatID, fmt.Sprintf("✅ Задача «%s» выполнена!", task.Title))
		} else {
			b.answerCallback(cb.ID, "Отметка снята")
			b.replyPlain(chatID, fmt.Sprintf("⬜ Задача «%s» снова активна.", task.Title))
		}
		b.refreshTaskList(ctx, chatID, user)

	case "del":
		title := "задача"
		if task, err := b.tasks.Get(ctx, user.ID, id); err == nil {
			title = task.Title
		}
		if err := b.tasks.Delete(ctx, user.ID, id); err != nil {
			b.log.Error("delete task from list", "task_id", id, "error", err)
			b.answerCallback(cb.ID, "Не удалось удалить.")
			return
		}
		b.answerCallback(cb.ID, "Удалено 🗑")
		b.replyPlain(chatID, fmt.Sprintf("🗑 Задача «%s» удалена.", title))
		b.refreshTaskList(ctx, chatID, user)

	default:
		b.answerCallback(cb.ID, "")
	}
}

// handleTaskCardCallback handles the buttons under a single task card.
func (b *Bot) handleTaskCardCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	_, action, id, ok := parseCallback(cb.Data)
	if !ok || id == uuid.Nil {
		b.answerCallback(cb.ID, "")
		return
	}
	chatID := cb.Message.Chat.ID
	msgID := cb.Message.MessageID

	user, err := b.users.GetByTelegramID(ctx, cb.From.ID)
	if err != nil {
		b.answerCallback(cb.ID, "Вы не авторизованы.")
		return
	}

	switch action {
	case "toggle":
		task, err := b.tasks.Get(ctx, user.ID, id)
		if err != nil {
			b.answerCallback(cb.ID, "Задача не найдена.")
			return
		}
		done := !task.IsDone
		updated, err := b.tasks.Update(ctx, user.ID, id, repository.TaskUpdate{IsDone: &done})
		if err != nil {
			b.log.Error("toggle task from card", "task_id", id, "error", err)
			b.answerCallback(cb.ID, "Не удалось обновить статус.")
			return
		}
		b.answerCallback(cb.ID, "Готово")
		loc := user.Location()
		markup := taskCardKeyboard(updated.ID, updated.IsDone)
		// A card with an image is a photo/video message: it has a caption, not
		// text, so it must be edited with editMessageCaption, not editMessageText.
		if len(cb.Message.Photo) > 0 || cb.Message.Video != nil {
			primaryURL, _ := primaryMedia(updated.Attachments)
			caption := truncateRunes(renderTaskCardText(updated, loc, primaryURL), tgCaptionLimit)
			edit := tgbotapi.NewEditMessageCaption(chatID, msgID, caption)
			edit.ParseMode = tgbotapi.ModeMarkdown
			edit.ReplyMarkup = &markup
			if _, err := b.api.Send(edit); err != nil {
				b.log.Debug("edit card caption", "error", err)
			}
		} else {
			edit := tgbotapi.NewEditMessageTextAndMarkup(chatID, msgID, renderTaskCardText(updated, loc, ""), markup)
			edit.ParseMode = tgbotapi.ModeMarkdown
			edit.DisableWebPagePreview = true
			if _, err := b.api.Send(edit); err != nil {
				b.log.Debug("edit card", "error", err)
			}
		}
		b.refreshTaskList(ctx, chatID, user)

	case "del":
		if err := b.tasks.Delete(ctx, user.ID, id); err != nil {
			b.log.Error("delete task from card", "task_id", id, "error", err)
			b.answerCallback(cb.ID, "Не удалось удалить.")
			return
		}
		b.answerCallback(cb.ID, "Удалено 🗑")
		// The card may be a photo message (caption, not text), so drop it and
		// post a plain confirmation instead of editing in place.
		b.deleteMessage(chatID, msgID)
		b.replyPlain(chatID, "🗑 Задача удалена.")
		b.refreshTaskList(ctx, chatID, user)

	case "back":
		b.answerCallback(cb.ID, "")
		b.deleteMessage(chatID, msgID)
		b.showBackToList(ctx, chatID, user)

	default:
		b.answerCallback(cb.ID, "")
	}
}

func (b *Bot) answerCallback(callbackID, text string) {
	callback := tgbotapi.NewCallback(callbackID, text)
	if _, err := b.api.Request(callback); err != nil {
		b.log.Error("answer callback", "error", err)
	}
}

// ==========================================================================
// Natural-language pipeline — idle chat / voice without the menu button
// ==========================================================================

// processNaturalLanguage handles a free-form message sent while idle. Task
// CREATION is delegated to the shared composeTask + createTask pair (identical
// to the "add task" button flow); only list / update / delete / reschedule stay
// here, since those have no counterpart in the compose flow.
func (b *Bot) processNaturalLanguage(ctx context.Context, chatID, telegramID int64, text string) {
	user, err := b.users.GetByTelegramID(ctx, telegramID)
	if err != nil {
		b.reply(chatID, "Сначала войдите через сайт NowDone, чтобы я знал, кому принадлежат задачи.")
		return
	}

	// Explicit «Заголовок»/«Описание» markers → the user hand-split the fields:
	// go straight to the shared pipeline (composeTask resolves date + reminder).
	if pf := parseMarkedFields(text); pf.found() {
		b.createTask(ctx, chatID, user, b.composeTask(ctx, text, user, nil), nil)
		return
	}

	nowRef := time.Now().In(user.Location()).Format("2006-01-02T15:04")
	intent, err := b.openai.ParseIntent(ctx, text, nowRef)
	if err != nil {
		b.log.Error("parse intent", "error", err)
		b.reply(chatID, "Не получилось понять запрос. Попробуйте переформулировать.")
		return
	}

	switch intent.Action {
	case "create":
		// Feed the intent we already fetched into the shared pipeline so there is
		// no second OpenAI call and stripReminderPhrase / field-splitting run.
		b.createTask(ctx, chatID, user, b.composeTask(ctx, text, user, intent), nil)
	case "list":
		b.handleListIntent(ctx, chatID, user, intent)
	case "update_status", "delete", "reschedule":
		b.handleMutateIntent(ctx, chatID, user, intent)
	default:
		b.reply(chatID, "Не понял, что нужно сделать. Уточните запрос.")
	}
}

func (b *Bot) handleListIntent(ctx context.Context, chatID int64, user *models.User, intent *service.Intent) {
	date := time.Now()
	if intent.Date != "" {
		if parsed, err := time.Parse("2006-01-02", intent.Date); err == nil {
			date = parsed
		}
	}

	tasks, err := b.tasks.ListRange(ctx, user.ID, date, date)
	if err != nil {
		b.log.Error("list tasks from bot", "error", err)
		b.reply(chatID, "Не удалось загрузить задачи.")
		return
	}
	if len(tasks) == 0 {
		b.reply(chatID, "На "+date.Format("02.01.2006")+" задач нет.")
		return
	}

	var sb strings.Builder
	sb.WriteString("📋 Задачи на " + date.Format("02.01.2006") + ":\n")
	for _, t := range tasks {
		status := "⬜"
		if t.IsDone {
			status = "✅"
		}
		sb.WriteString(fmt.Sprintf("%s %s\n", status, t.Title))
	}
	b.reply(chatID, sb.String())
}

func (b *Bot) handleMutateIntent(ctx context.Context, chatID int64, user *models.User, intent *service.Intent) {
	from := time.Now().AddDate(0, 0, -7)
	to := time.Now().AddDate(0, 0, 7)

	tasks, err := b.tasks.ListRange(ctx, user.ID, from, to)
	if err != nil {
		b.log.Error("list tasks for mutate", "error", err)
		b.reply(chatID, "Не удалось найти задачу.")
		return
	}

	var match *models.Task
	for _, t := range tasks {
		if strings.Contains(strings.ToLower(t.Title), strings.ToLower(intent.Title)) {
			match = t
			break
		}
	}
	if match == nil {
		b.reply(chatID, "Не нашёл задачу с похожим названием.")
		return
	}

	switch intent.Action {
	case "update_status":
		done := intent.Status == "done"
		if _, err := b.tasks.Update(ctx, user.ID, match.ID, repository.TaskUpdate{IsDone: &done}); err != nil {
			b.log.Error("update task status from bot", "error", err)
			b.reply(chatID, "Не удалось обновить статус задачи.")
			return
		}
		b.reply(chatID, "Обновлено: *"+match.Title+"*")
	case "delete":
		if err := b.tasks.Delete(ctx, user.ID, match.ID); err != nil {
			b.log.Error("delete task from bot", "error", err)
			b.reply(chatID, "Не удалось удалить задачу.")
			return
		}
		b.reply(chatID, "Удалено: *"+match.Title+"*")
	case "reschedule":
		if intent.NewDate == "" {
			b.reply(chatID, "Не указана новая дата.")
			return
		}
		newDate, err := time.Parse("2006-01-02", intent.NewDate)
		if err != nil {
			b.reply(chatID, "Некорректная новая дата.")
			return
		}
		if _, err := b.tasks.Update(ctx, user.ID, match.ID, repository.TaskUpdate{Date: &newDate}); err != nil {
			b.log.Error("reschedule task from bot", "error", err)
			b.reply(chatID, "Не удалось перенести задачу.")
			return
		}
		b.reply(chatID, "Перенесено на "+newDate.Format("02.01.2006")+": *"+match.Title+"*")
	}
	b.refreshTaskList(ctx, chatID, user)
}

// ==========================================================================
// small helpers
// ==========================================================================

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func sameDate(a, c time.Time) bool {
	ay, am, ad := a.Date()
	cy, cm, cd := c.Date()
	return ay == cy && am == cm && ad == cd
}

// mdEscape escapes the Telegram Markdown (v1) metacharacters so user-supplied
// titles / descriptions cannot break message formatting.
var mdReplacer = strings.NewReplacer("_", "\\_", "*", "\\*", "[", "\\[", "]", "\\]", "`", "\\`")

func mdEscape(s string) string { return mdReplacer.Replace(s) }

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

// plainText extracts readable text from a task description / note content jsonb
// value. It understands the bot's own {"text":"…"} shape and the web app's
// Editor.js document ({"blocks":[{"data":{"text":"…"}}]}).
func plainText(raw models.JSONRaw) string {
	if len(raw) == 0 {
		return ""
	}

	var simple struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &simple); err == nil && simple.Text != "" {
		return simple.Text
	}

	var doc struct {
		Blocks []struct {
			Data struct {
				Text  string   `json:"text"`
				Items []string `json:"items"`
			} `json:"data"`
		} `json:"blocks"`
	}
	if err := json.Unmarshal(raw, &doc); err == nil && len(doc.Blocks) > 0 {
		var parts []string
		for _, blk := range doc.Blocks {
			if t := strings.TrimSpace(htmlTagRe.ReplaceAllString(blk.Data.Text, "")); t != "" {
				parts = append(parts, t)
			}
			for _, it := range blk.Data.Items {
				if t := strings.TrimSpace(htmlTagRe.ReplaceAllString(it, "")); t != "" {
					parts = append(parts, "• "+t)
				}
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}
