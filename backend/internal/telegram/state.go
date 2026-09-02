package telegram

import (
	"sync"
	"time"

	"nowdone/internal/models"
)

// draftFlushDelay is how long the bot waits after the last piece of a task/note
// draft before creating it. It exists so an album (several photos sent as one
// Telegram "media group") is collected into a single task instead of one task
// per photo. A plain text or voice message finalizes the draft immediately.
const draftFlushDelay = 1500 * time.Millisecond

// convMode is the per-chat conversation mode for the multi-step flows started
// from the reply-keyboard menu.
type convMode int

const (
	modeIdle convMode = iota
	modeAddTask
	modeAddNote
)

// draft accumulates the parts of a task/note being composed across one or more
// Telegram messages: the first line of the first text is the title, the rest is
// the description, and every photo/video/document becomes an attachment.
type draft struct {
	title       string
	description string
	attachments []models.Attachment
	hasText     bool

	// raw is the first text / voice transcript / caption verbatim. It is used to
	// resolve a relative date and a reminder time via OpenAI, since a phrase like
	// "напомни в 15:00" may sit outside the title/description that markers carve
	// out.
	raw string

	// flush finalizes the draft draftFlushDelay after the last message; it is
	// re-armed on every new message.
	flush *time.Timer
}

// chatState is everything the bot remembers between updates for one chat.
type chatState struct {
	mode convMode
	dr   *draft

	// listMsgID is the id of the single task-list message the bot keeps
	// re-rendering in place (so the chat is not flooded with copies), and
	// listDate is the day that message currently shows — "today" or "tomorrow",
	// depending on which menu button the user pressed. listDate lets a mutation
	// (toggle/delete/create) refresh the list for the right day.
	listMsgID int
	listDate  time.Time
}

// stateStore is a concurrency-safe chatID -> chatState map. Every update is
// handled in its own goroutine (and album photos arrive almost simultaneously),
// so all access goes through the mutex.
type stateStore struct {
	mu sync.Mutex
	m  map[int64]*chatState
}

func newStateStore() *stateStore {
	return &stateStore{m: make(map[int64]*chatState)}
}

// withLock runs fn while holding the store lock, creating the chatState on first
// use. Callers must not retain the *chatState after fn returns.
func (s *stateStore) withLock(chatID int64, fn func(st *chatState)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.m[chatID]
	if !ok {
		st = &chatState{mode: modeIdle}
		s.m[chatID] = st
	}
	fn(st)
}
