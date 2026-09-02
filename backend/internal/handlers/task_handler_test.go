package handlers

import (
	"testing"
	"time"
	_ "time/tzdata" // ensure LoadLocation works without a system zoneinfo db
)

func TestParseReminderTime(t *testing.T) {
	moscow, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatalf("load Europe/Moscow: %v", err)
	}

	tests := []struct {
		name    string
		raw     string
		loc     *time.Location
		wantNil bool
		wantUTC string // expected RFC3339 in UTC
		wantErr bool
	}{
		{
			name:    "empty is no reminder",
			raw:     "  ",
			loc:     moscow,
			wantNil: true,
		},
		{
			name:    "naive wall-clock is read in the user's timezone",
			raw:     "2026-09-01T18:30",
			loc:     moscow,
			wantUTC: "2026-09-01T15:30:00Z", // MSK is UTC+3
		},
		{
			name:    "naive wall-clock with seconds",
			raw:     "2026-09-01T18:30:45",
			loc:     moscow,
			wantUTC: "2026-09-01T15:30:45Z",
		},
		{
			name:    "same wall-clock in UTC stays as-is",
			raw:     "2026-09-01T18:30",
			loc:     time.UTC,
			wantUTC: "2026-09-01T18:30:00Z",
		},
		{
			name:    "explicit offset is trusted regardless of user's timezone",
			raw:     "2026-09-01T18:30:00+02:00",
			loc:     moscow,
			wantUTC: "2026-09-01T16:30:00Z",
		},
		{
			name:    "Z timestamp is trusted",
			raw:     "2026-09-01T18:30:00Z",
			loc:     moscow,
			wantUTC: "2026-09-01T18:30:00Z",
		},
		{
			name:    "garbage is rejected",
			raw:     "not-a-time",
			loc:     moscow,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseReminderTime(tc.raw, tc.loc)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantNil {
				if got != nil {
					t.Fatalf("expected nil, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected a time, got nil")
			}
			if gotUTC := got.UTC().Format(time.RFC3339); gotUTC != tc.wantUTC {
				t.Fatalf("got %s, want %s", gotUTC, tc.wantUTC)
			}
		})
	}
}
