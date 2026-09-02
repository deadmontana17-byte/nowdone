package service

import (
	"testing"
	"time"

	"nowdone/internal/models"
)

func date(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestNextOccurrenceDate(t *testing.T) {
	tests := []struct {
		name string
		from string
		rule *models.RecurrenceRule
		want string // "" means zero time (no next occurrence)
	}{
		{
			name: "daily",
			from: "2026-09-01",
			rule: &models.RecurrenceRule{Frequency: "daily"},
			want: "2026-09-02",
		},
		{
			name: "weekdays from a Friday skips the weekend to Monday",
			from: "2026-09-04", // Friday
			rule: &models.RecurrenceRule{Frequency: "weekdays"},
			want: "2026-09-07", // Monday
		},
		{
			name: "weekdays from a Saturday lands on Monday",
			from: "2026-09-05", // Saturday
			rule: &models.RecurrenceRule{Frequency: "weekdays"},
			want: "2026-09-07",
		},
		{
			name: "weekdays from a Tuesday is the next day",
			from: "2026-09-01", // Tuesday
			rule: &models.RecurrenceRule{Frequency: "weekdays"},
			want: "2026-09-02",
		},
		{
			name: "yearly keeps the same month and day",
			from: "2026-09-01",
			rule: &models.RecurrenceRule{Frequency: "yearly"},
			want: "2027-09-01",
		},
		{
			name: "yearly honours a multi-year interval",
			from: "2026-09-01",
			rule: &models.RecurrenceRule{Frequency: "yearly", Interval: 2},
			want: "2028-09-01",
		},
		{
			name: "unknown frequency yields no next occurrence",
			from: "2026-09-01",
			rule: &models.RecurrenceRule{Frequency: "fortnightly"},
			want: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := nextOccurrenceDate(date(tc.from), tc.rule)
			if tc.want == "" {
				if !got.IsZero() {
					t.Fatalf("got %s, want zero time", got.Format("2006-01-02"))
				}
				return
			}
			if got.Format("2006-01-02") != tc.want {
				t.Fatalf("got %s, want %s", got.Format("2006-01-02"), tc.want)
			}
		})
	}
}
