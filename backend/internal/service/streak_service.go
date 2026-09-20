package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"nowdone/internal/models"
)

// MoscowLocation is UTC+3. Kept for the cron scheduler's location; the streak
// calculation itself now works in each user's own timezone.
var MoscowLocation = time.FixedZone("MSK", 3*60*60)

// maxStreakLookbackDays caps how far back RecalculateAll will walk for a single
// user, a safety bound against a bad created_at (e.g. in the future) turning the
// day-by-day scan into a very long loop.
const maxStreakLookbackDays = 2000

// minLevel/maxLevel bound the character level: 10 named characters (see
// StatusIndex), level 1 ("Зелёный") through level 10 ("Бог планирования").
const (
	minLevel = 1
	maxLevel = 10
)

// streakUserStore is the slice of UserRepository the streak worker needs. Keeping
// it an interface lets the calculation be unit-tested with fakes.
type streakUserStore interface {
	AllUsers(ctx context.Context) ([]*models.User, error)
	UpdateStreak(ctx context.Context, userID uuid.UUID, current, max int) error
}

// streakTaskStore is the slice of TaskRepository the streak worker needs.
type streakTaskStore interface {
	ListByUserAndRange(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]*models.Task, error)
}

// StreakService recalculates every user's character level. It is driven by
// the cron worker once per hour (per the backend rules). The calculation is a
// full recompute from scratch, so re-running it any number of times a day
// converges on the same value.
//
// The level (stored in the `current_streak` column, kept for schema
// compatibility) moves +1 for each past day where every task was completed
// and -1 for each past day with tasks left undone; a day with no tasks at all
// is neutral. `max_streak` stores the highest level ever reached — the
// player's record — and never decreases.
type StreakService struct {
	users streakUserStore
	tasks streakTaskStore
	log   *slog.Logger
}

func NewStreakService(users streakUserStore, tasks streakTaskStore, log *slog.Logger) *StreakService {
	if log == nil {
		log = slog.Default()
	}
	return &StreakService{users: users, tasks: tasks, log: log}
}

// RecalculateAll recomputes current_streak / max_streak for every user.
func (s *StreakService) RecalculateAll(ctx context.Context) error {
	users, err := s.users.AllUsers(ctx)
	if err != nil {
		return fmt.Errorf("list users: %w", err)
	}

	for _, user := range users {
		current, err := s.currentLevel(ctx, user)
		if err != nil {
			s.log.Error("compute level", "user_id", user.ID, "error", err)
			continue
		}

		newMax := user.MaxStreak
		if current > newMax {
			newMax = current
		}

		if current == user.CurrentStreak && newMax == user.MaxStreak {
			continue // nothing changed, skip the write
		}

		if err := s.users.UpdateStreak(ctx, user.ID, current, newMax); err != nil {
			s.log.Error("update streak", "user_id", user.ID, "error", err)
			continue
		}
		s.log.Info("level updated",
			"user_id", user.ID,
			"level", current,
			"max_level", newMax,
			"previous_level", user.CurrentStreak,
		)
	}

	return nil
}

// currentLevel walks every day from the user's created_at date (in their own
// timezone) up to today and turns it into a character level:
//
//   - A past day whose tasks are all done adds 1 to the level.
//   - A past day with at least one unfinished task subtracts 1.
//   - A past day with no tasks at all is neutral — nothing to miss.
//   - Today only ever adds (once all of today's tasks are done); an unfinished
//     or empty today never subtracts, since the day is not over yet.
//
// The level is clamped to [minLevel, maxLevel] at every step, so a long bad
// streak bottoms out at level 1 instead of going negative, and one good day
// is always enough to move up from the floor.
//
// The scan is bounded by the user's created_at date and by maxStreakLookbackDays.
func (s *StreakService) currentLevel(ctx context.Context, user *models.User) (int, error) {
	loc := user.Location()
	now := time.Now().In(loc)

	// Date keys are stored in the `date` column as calendar dates; the repository
	// queries pass a UTC-midnight time.Time, so match that here.
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	createdLocal := user.CreatedAt.In(loc)
	floor := time.Date(createdLocal.Year(), createdLocal.Month(), createdLocal.Day(), 0, 0, 0, 0, time.UTC)

	// Clamp the window so a bad created_at (e.g. far in the past) can't turn the
	// range query into a huge scan; the day-by-day walk was already bounded by
	// maxStreakLookbackDays.
	if earliest := today.AddDate(0, 0, -maxStreakLookbackDays); floor.Before(earliest) {
		floor = earliest
	}

	// One query for the whole window instead of one per day. This was the
	// reminder worker's dominant cost: O(users × days) round-trips per run.
	tasks, err := s.tasks.ListByUserAndRange(ctx, user.ID, floor, today)
	if err != nil {
		return 0, fmt.Errorf("list tasks %s..%s: %w", floor.Format("2006-01-02"), today.Format("2006-01-02"), err)
	}

	byDay := make(map[string][]*models.Task, len(tasks))
	for _, t := range tasks {
		key := t.Date.Time.Format("2006-01-02")
		byDay[key] = append(byDay[key], t)
	}

	level := minLevel
	for day := floor; !day.After(today); day = day.AddDate(0, 0, 1) {
		dayTasks := byDay[day.Format("2006-01-02")]
		isToday := day.Equal(today)

		switch {
		case len(dayTasks) > 0 && allDone(dayTasks):
			level++
		case !isToday && len(dayTasks) > 0:
			level-- // unfinished past day: missed
		}
		// !isToday && len(dayTasks) == 0: neutral, nothing to miss.
		// isToday: an unfinished or empty today never subtracts.

		if level < minLevel {
			level = minLevel
		} else if level > maxLevel {
			level = maxLevel
		}
	}

	return level, nil
}

func allDone(tasks []*models.Task) bool {
	for _, t := range tasks {
		if !t.IsDone {
			return false
		}
	}
	return true
}

// StatusIndex maps a character level (1..10) to its character image/name
// index (0..9) — level 1 is index 0 ("Зелёный") through level 10 is index 9
// ("Бог планирования"). Clamped so an out-of-range value never panics on a
// slice/asset lookup.
func StatusIndex(level int) int {
	switch {
	case level <= minLevel:
		return 0
	case level >= maxLevel:
		return maxLevel - 1
	default:
		return level - 1
	}
}
