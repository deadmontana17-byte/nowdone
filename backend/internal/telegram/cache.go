package telegram

import (
	"sync"
	"time"

	"github.com/google/uuid"

	"nowdone/internal/models"
)

// taskListTTL is how long the bot serves a day's task list from memory instead
// of querying Postgres. It is short enough that a change made on the website
// shows up within a few seconds, and long enough to absorb the burst of reads a
// single "toggle a checkbox → re-render the list" interaction produces (the
// list is rendered, then refreshed, often several times per second). Every
// mutation the bot itself performs calls invalidate first, so the bot's own
// writes are always reflected immediately.
const taskListTTL = 8 * time.Second

// cacheSweepThreshold bounds map growth: once this many entries accumulate, a
// put also drops expired ones. Chats are keyed by (user, day) so a busy bot with
// many users would otherwise keep stale days around until process restart.
const cacheSweepThreshold = 1024

type taskListKey struct {
	user uuid.UUID
	day  string // "2006-01-02", the day the list is for
}

type taskListEntry struct {
	tasks   []*models.Task
	expires time.Time
}

// taskListCache is a tiny TTL cache for renderTaskList. A nil *taskListCache is
// safe to use — every method is a no-op / miss — so callers need no nil checks.
type taskListCache struct {
	mu  sync.Mutex
	ttl time.Duration
	m   map[taskListKey]taskListEntry
}

func newTaskListCache(ttl time.Duration) *taskListCache {
	return &taskListCache{ttl: ttl, m: make(map[taskListKey]taskListEntry)}
}

func (c *taskListCache) get(user uuid.UUID, day time.Time) ([]*models.Task, bool) {
	if c == nil {
		return nil, false
	}
	key := taskListKey{user: user, day: day.Format("2006-01-02")}

	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.m[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expires) {
		delete(c.m, key)
		return nil, false
	}
	return entry.tasks, true
}

func (c *taskListCache) put(user uuid.UUID, day time.Time, tasks []*models.Task) {
	if c == nil {
		return
	}
	key := taskListKey{user: user, day: day.Format("2006-01-02")}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = taskListEntry{tasks: tasks, expires: time.Now().Add(c.ttl)}

	if len(c.m) > cacheSweepThreshold {
		now := time.Now()
		for k, e := range c.m {
			if now.After(e.expires) {
				delete(c.m, k)
			}
		}
	}
}

// invalidate drops every cached day for one user. Call it right before any
// task create / toggle / delete / reschedule so the next render reflects the
// change instead of a stale snapshot.
func (c *taskListCache) invalidate(user uuid.UUID) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.m {
		if k.user == user {
			delete(c.m, k)
		}
	}
}
