package telegram

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"nowdone/internal/models"
)

func TestTaskListCache_HitMissExpiryInvalidate(t *testing.T) {
	c := newTaskListCache(40 * time.Millisecond)
	user := uuid.New()
	day := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)

	if _, ok := c.get(user, day); ok {
		t.Fatal("empty cache must miss")
	}

	c.put(user, day, []*models.Task{{Title: "one"}})
	got, ok := c.get(user, day)
	if !ok || len(got) != 1 || got[0].Title != "one" {
		t.Fatalf("want single cached task, got ok=%v tasks=%v", ok, got)
	}

	// A different day for the same user is a separate entry.
	if _, ok := c.get(user, day.AddDate(0, 0, 1)); ok {
		t.Fatal("other day must miss")
	}

	c.invalidate(user)
	if _, ok := c.get(user, day); ok {
		t.Fatal("must miss after invalidate")
	}

	c.put(user, day, nil)
	time.Sleep(60 * time.Millisecond)
	if _, ok := c.get(user, day); ok {
		t.Fatal("must miss after TTL")
	}
}

func TestTaskListCache_NilSafe(t *testing.T) {
	var c *taskListCache
	c.put(uuid.New(), time.Now(), nil) // must not panic
	c.invalidate(uuid.New())
	if _, ok := c.get(uuid.New(), time.Now()); ok {
		t.Fatal("nil cache must always miss")
	}
}
