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

// StreakService recalculates every user's current/max streak. It is driven by
// the cron worker once per hour (per the backend rules). The calculation is a
// full recompute from scratch, so re-running it any number of times a day
// converges on the same value.
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
		current, err := s.currentStreak(ctx, user)
		if err != nil {
			s.log.Error("compute streak", "user_id", user.ID, "error", err)
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
		s.log.Info("streak updated",
			"user_id", user.ID,
			"current_streak", current,
			"max_streak", newMax,
			"previous_current", user.CurrentStreak,
		)
	}

	return nil
}

// currentStreak counts consecutive days, ending today (in the user's timezone),
// on which the user completed every task planned for that day:
//
//   - A day whose tasks are all done adds 1 to the streak.
//   - The first past day with an unfinished task ends the streak.
//   - The first past day with no tasks at all also ends it — an inactive day
//     breaks the chain.
//   - Today is only counted once all of today's tasks are done, but a still
//     unfinished or empty today does not reset the streak: the day is not over,
//     so the scan just continues with yesterday.
//
// The scan is bounded by the user's created_at date and by maxStreakLookbackDays.
func (s *StreakService) currentStreak(ctx context.Context, user *models.User) (int, error) {
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

	streak := 0
	for day := today; !day.Before(floor); day = day.AddDate(0, 0, -1) {
		dayTasks := byDay[day.Format("2006-01-02")]
		isToday := day.Equal(today)

		if len(dayTasks) == 0 {
			if isToday {
				continue // today may simply have no tasks yet — keep scanning
			}
			break // an inactive past day breaks the streak
		}

		if allDone(dayTasks) {
			streak++
			continue
		}

		// The day has an unfinished task.
		if isToday {
			continue // today is not over yet — keep counting earlier days
		}
		break
	}

	return streak, nil
}

func allDone(tasks []*models.Task) bool {
	for _, t := range tasks {
		if !t.IsDone {
			return false
		}
	}
	return true
}

// StatusIndex maps a current streak to the character/progress category index
// (0..8) described in the spec: 1 day, 1-9, 10-19, ..., 60-100, 100+.
func StatusIndex(currentStreak int) int {
	switch {
	case currentStreak <= 1:
		return 0
	case currentStreak < 10:
		return 1
	case currentStreak < 20:
		return 2
	case currentStreak < 30:
		return 3
	case currentStreak < 40:
		return 4
	case currentStreak < 50:
		return 5
	case currentStreak < 60:
		return 6
	case currentStreak <= 100:
		return 7
	default:
		return 8
	}
}
