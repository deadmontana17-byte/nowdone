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
// stamping each with its Date so currentLevel can bucket them by day.
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

func TestCurrentLevel(t *testing.T) {
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
			want: 5, // start at 1, +1 per completed day (-3,-2,-1,0)
		},
		{
			name:       "unfinished task today never subtracts, just skips the add",
			createdOff: -30,
			days: map[int][]*models.Task{
				0:  tasks(true, false),
				-1: tasks(true),
				-2: tasks(true),
			},
			want: 3, // 1 + (-2 done) + (-1 done); today unfinished: no change
		},
		{
			name:       "empty today does not change the level",
			createdOff: -30,
			days: map[int][]*models.Task{
				-1: tasks(true),
				-2: tasks(true),
			},
			want: 3, // 1 + (-2 done) + (-1 done); today empty: no change
		},
		{
			name:       "unfinished task on a past day subtracts one, not the whole level",
			createdOff: -30,
			days: map[int][]*models.Task{
				0:  tasks(true),
				-1: tasks(true),
				-2: tasks(true, false),
				-3: tasks(true),
			},
			want: 3, // 1 +1(-3) -1(-2) +1(-1) +1(0)
		},
		{
			name:       "a past day with no tasks is neutral",
			createdOff: -30,
			days: map[int][]*models.Task{
				0:  tasks(true),
				-1: tasks(true),
				-3: tasks(true), // -2 has no tasks: neutral, not a subtraction
			},
			want: 4, // 1 +1(-3) +0(-2) +1(-1) +1(0)
		},
		{
			name:       "no tasks anywhere stays at the floor level",
			createdOff: -30,
			days:       map[int][]*models.Task{},
			want:       1,
		},
		{
			name:       "level counts the day the account was created",
			createdOff: -2,
			days: map[int][]*models.Task{
				0:  tasks(true),
				-1: tasks(true),
				-2: tasks(true),
				-3: tasks(true), // before the created_at floor, must be ignored
			},
			want: 4, // 1 +1(-2) +1(-1) +1(0)
		},
		{
			name:       "level floors at 1 instead of going negative",
			createdOff: -10,
			days: map[int][]*models.Task{
				-1: tasks(true, false),
				-2: tasks(true, false),
				-3: tasks(true, false),
				-4: tasks(true, false),
				-5: tasks(true, false),
			},
			want: 1,
		},
		{
			name:       "level caps at 10 instead of climbing forever",
			createdOff: -15,
			days: func() map[int][]*models.Task {
				m := map[int][]*models.Task{}
				for i := -15; i <= 0; i++ {
					m[i] = tasks(true)
				}
				return m
			}(),
			want: 10,
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

			got, err := svc.currentLevel(context.Background(), user)
			if err != nil {
				t.Fatalf("currentLevel: %v", err)
			}
			if got != tc.want {
				t.Fatalf("level = %d, want %d", got, tc.want)
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
	// Computed level: 1 (floor) +1(-2) +1(-1) +1(0) = 4.

	// Level grew past the old record: both current and max move to 4.
	grower := &models.User{
		ID: uuid.New(), Timezone: "UTC",
		CreatedAt:     time.Now().UTC().AddDate(0, 0, -30),
		CurrentStreak: 1, MaxStreak: 1,
	}
	// Level fell back (an old missed day) but the record must be kept.
	faller := &models.User{
		ID: uuid.New(), Timezone: "UTC",
		CreatedAt:     time.Now().UTC().AddDate(0, 0, -30),
		CurrentStreak: 9, MaxStreak: 9,
	}

	users := &fakeUserStore{users: []*models.User{grower, faller}}
	svc := NewStreakService(users, &fakeTaskStore{byDate: byDate}, nil)

	if err := svc.RecalculateAll(context.Background()); err != nil {
		t.Fatalf("RecalculateAll: %v", err)
	}

	if got := users.updated[grower.ID]; got != [2]int{4, 4} {
		t.Fatalf("grower updated to %v, want [4 4]", got)
	}
	if got := users.updated[faller.ID]; got != [2]int{4, 9} {
		t.Fatalf("faller updated to %v, want [4 9]", got)
	}
}

func TestRecalculateAllSkipsWriteWhenUnchanged(t *testing.T) {
	byDate := map[string][]*models.Task{
		dayKey(0):  tasks(true),
		dayKey(-1): tasks(true),
	}
	// Computed level: 1 (floor) +1(-1) +1(0) = 3.
	user := &models.User{
		ID: uuid.New(), Timezone: "UTC",
		CreatedAt:     time.Now().UTC().AddDate(0, 0, -30),
		CurrentStreak: 3, MaxStreak: 5,
	}
	users := &fakeUserStore{users: []*models.User{user}}
	svc := NewStreakService(users, &fakeTaskStore{byDate: byDate}, nil)

	if err := svc.RecalculateAll(context.Background()); err != nil {
		t.Fatalf("RecalculateAll: %v", err)
	}
	if _, wrote := users.updated[user.ID]; wrote {
		t.Fatalf("expected no write when level is unchanged, got %v", users.updated[user.ID])
	}
}

func TestStatusIndex(t *testing.T) {
	cases := map[int]int{0: 0, 1: 0, 2: 1, 5: 4, 9: 8, 10: 9, 11: 9, 100: 9}
	for level, want := range cases {
		if got := StatusIndex(level); got != want {
			t.Errorf("StatusIndex(%d) = %d, want %d", level, got, want)
		}
	}
}
