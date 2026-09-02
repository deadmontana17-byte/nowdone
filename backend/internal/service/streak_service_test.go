package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"nowdone/internal/models"
)

type fakeUserStore struct {
	users   []*models.User
	updated map[uuid.UUID][2]int // userID -> {current, max}
}

func (f *fakeUserStore) AllUsers(context.Context) ([]*models.User, error) {
	return f.users, nil
}

func (f *fakeUserStore) UpdateStreak(_ context.Context, userID uuid.UUID, current, max int) error {
	if f.updated == nil {
		f.updated = map[uuid.UUID][2]int{}
	}
	f.updated[userID] = [2]int{current, max}
	return nil
}

type fakeTaskStore struct {
	byDate map[string][]*models.Task // key: "2006-01-02"
}

// ListByUserAndRange returns every fake task whose day falls in [from, to],
// stamping each with its Date so currentStreak can bucket them by day.
func (f *fakeTaskStore) ListByUserAndRange(_ context.Context, _ uuid.UUID, from, to time.Time) ([]*models.Task, error) {
	var out []*models.Task
	for key, ts := range f.byDate {
		day, err := time.Parse("2006-01-02", key)
		if err != nil {
			return nil, err
		}
		if day.Before(from) || day.After(to) {
			continue
		}
		for _, t := range ts {
			clone := *t
			clone.Date = models.NewDate(day)
			out = append(out, &clone)
		}
	}
	return out, nil
}

// dayKey returns the "2006-01-02" key for today+offset in UTC, matching how the
// service builds the date it queries.
func dayKey(offset int) string {
	n := time.Now().UTC().AddDate(0, 0, offset)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC).Format("2006-01-02")
}

func tasks(doneFlags ...bool) []*models.Task {
	out := make([]*models.Task, len(doneFlags))
	for i, d := range doneFlags {
		out[i] = &models.Task{ID: uuid.New(), IsDone: d}
	}
	return out
}

func TestCurrentStreak(t *testing.T) {
	tests := []struct {
		name       string
		createdOff int // days before today the user was created
		days       map[int][]*models.Task
		want       int
	}{
		{
			name:       "all tasks done today and previous days",
			createdOff: -30,
			days: map[int][]*models.Task{
				0:  tasks(true, true),
				-1: tasks(true),
				-2: tasks(true, true, true),
				-3: tasks(true),
			},
			want: 4,
		},
		{
			name:       "unfinished task today does not reset, today just not counted",
			createdOff: -30,
			days: map[int][]*models.Task{
				0:  tasks(true, false),
				-1: tasks(true),
				-2: tasks(true),
			},
			want: 2,
		},
		{
			name:       "empty today does not reset",
			createdOff: -30,
			days: map[int][]*models.Task{
				-1: tasks(true),
				-2: tasks(true),
			},
			want: 2,
		},
		{
			name:       "unfinished task on a past day ends the streak",
			createdOff: -30,
			days: map[int][]*models.Task{
				0:  tasks(true),
				-1: tasks(true),
				-2: tasks(true, false),
				-3: tasks(true),
			},
			want: 2,
		},
		{
			name:       "an inactive past day ends the streak",
			createdOff: -30,
			days: map[int][]*models.Task{
				0:  tasks(true),
				-1: tasks(true),
				-3: tasks(true),
			},
			want: 2,
		},
		{
			name:       "no tasks anywhere",
			createdOff: -30,
			days:       map[int][]*models.Task{},
			want:       0,
		},
		{
			name:       "streak counts the day the account was created",
			createdOff: -2,
			days: map[int][]*models.Task{
				0:  tasks(true),
				-1: tasks(true),
				-2: tasks(true),
				-3: tasks(true), // before the created_at floor, must be ignored
			},
			want: 3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			byDate := map[string][]*models.Task{}
			for off, ts := range tc.days {
				byDate[dayKey(off)] = ts
			}
			user := &models.User{
				ID:        uuid.New(),
				Timezone:  "UTC",
				CreatedAt: time.Now().UTC().AddDate(0, 0, tc.createdOff),
			}
			svc := NewStreakService(&fakeUserStore{}, &fakeTaskStore{byDate: byDate}, nil)

			got, err := svc.currentStreak(context.Background(), user)
			if err != nil {
				t.Fatalf("currentStreak: %v", err)
			}
			if got != tc.want {
				t.Fatalf("streak = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestRecalculateAllUpdatesCurrentAndMax(t *testing.T) {
	byDate := map[string][]*models.Task{
		dayKey(0):  tasks(true),
		dayKey(-1): tasks(true),
		dayKey(-2): tasks(true),
	}

	// Streak grew past the old record: both current and max move to 3.
	grower := &models.User{
		ID: uuid.New(), Timezone: "UTC",
		CreatedAt:     time.Now().UTC().AddDate(0, 0, -30),
		CurrentStreak: 1, MaxStreak: 1,
	}
	// Streak collapsed (an old broken day) but the record must be kept.
	faller := &models.User{
		ID: uuid.New(), Timezone: "UTC",
		CreatedAt:     time.Now().UTC().AddDate(0, 0, -30),
		CurrentStreak: 42, MaxStreak: 42,
	}

	users := &fakeUserStore{users: []*models.User{grower, faller}}
	svc := NewStreakService(users, &fakeTaskStore{byDate: byDate}, nil)

	if err := svc.RecalculateAll(context.Background()); err != nil {
		t.Fatalf("RecalculateAll: %v", err)
	}

	if got := users.updated[grower.ID]; got != [2]int{3, 3} {
		t.Fatalf("grower updated to %v, want [3 3]", got)
	}
	if got := users.updated[faller.ID]; got != [2]int{3, 42} {
		t.Fatalf("faller updated to %v, want [3 42]", got)
	}
}

func TestRecalculateAllSkipsWriteWhenUnchanged(t *testing.T) {
	byDate := map[string][]*models.Task{
		dayKey(0):  tasks(true),
		dayKey(-1): tasks(true),
	}
	user := &models.User{
		ID: uuid.New(), Timezone: "UTC",
		CreatedAt:     time.Now().UTC().AddDate(0, 0, -30),
		CurrentStreak: 2, MaxStreak: 5,
	}
	users := &fakeUserStore{users: []*models.User{user}}
	svc := NewStreakService(users, &fakeTaskStore{byDate: byDate}, nil)

	if err := svc.RecalculateAll(context.Background()); err != nil {
		t.Fatalf("RecalculateAll: %v", err)
	}
	if _, wrote := users.updated[user.ID]; wrote {
		t.Fatalf("expected no write when streak is unchanged, got %v", users.updated[user.ID])
	}
}

func TestStatusIndex(t *testing.T) {
	cases := map[int]int{0: 0, 1: 0, 2: 1, 9: 1, 10: 2, 19: 2, 20: 3, 59: 6, 60: 7, 100: 7, 101: 8, 500: 8}
	for streak, want := range cases {
		if got := StatusIndex(streak); got != want {
			t.Errorf("StatusIndex(%d) = %d, want %d", streak, got, want)
		}
	}
}
