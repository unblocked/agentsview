package db

import (
	"context"
	"testing"
)

func TestPruneFilterZeroValue(t *testing.T) {
	// zero value of PruneFilter
	f := PruneFilter{}

	// Fix verification: HasFilters() should return false for zero value
	if f.HasFilters() {
		t.Error("HasFilters() returned true for zero value (fix failed)")
	}

	// Setup DB to see what it finds
	d := testDB(t)
	
	// Insert a session with 0 messages
	insertSession(t, d, "s1", "p", func(s *Session) {
		s.MessageCount = 0
	})
	// Insert a session with 5 messages
	insertSession(t, d, "s2", "p", func(s *Session) {
		s.MessageCount = 5
	})

	// Find candidates with zero-value filter
	// Should now return error as at least one filter is required
	_, err := d.FindPruneCandidates(f)
	if err == nil {
		t.Fatal("FindPruneCandidates should error on empty filter")
	}
	if err.Error() != "at least one filter is required" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSessionFilterDateFields(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	// Session on June 1
	insertSession(t, d, "s1", "proj", func(s *Session) {
		s.StartedAt = strPtr("2024-06-01T10:00:00Z")
		s.EndedAt = strPtr("2024-06-01T11:00:00Z")
		s.MessageCount = 5
	})
	// Session on June 2
	insertSession(t, d, "s2", "proj", func(s *Session) {
		s.StartedAt = strPtr("2024-06-02T10:00:00Z")
		s.EndedAt = strPtr("2024-06-02T11:00:00Z")
		s.MessageCount = 15
	})
	// Session on June 3
	insertSession(t, d, "s3", "proj", func(s *Session) {
		s.StartedAt = strPtr("2024-06-03T10:00:00Z")
		s.EndedAt = strPtr("2024-06-03T11:00:00Z")
		s.MessageCount = 25
	})

	tests := []struct {
		name   string
		filter SessionFilter
		want   int
	}{
		{
			name:   "ExactDate",
			filter: SessionFilter{Date: "2024-06-01", Limit: 100},
			want:   1,
		},
		{
			name: "DateRange",
			filter: SessionFilter{
				DateFrom: "2024-06-01",
				DateTo:   "2024-06-02",
				Limit:    100,
			},
			want: 2,
		},
		{
			name: "DateFrom",
			filter: SessionFilter{
				DateFrom: "2024-06-02",
				Limit:    100,
			},
			want: 2,
		},
		{
			name: "DateTo",
			filter: SessionFilter{
				DateTo: "2024-06-01",
				Limit:  100,
			},
			want: 1,
		},
		{
			name: "MinMessages",
			filter: SessionFilter{
				MinMessages: 10,
				Limit:       100,
			},
			want: 2,
		},
		{
			name: "MaxMessages",
			filter: SessionFilter{
				MaxMessages: 10,
				Limit:       100,
			},
			want: 1,
		},
		{
			name: "CombinedDateAndMessages",
			filter: SessionFilter{
				DateFrom:    "2024-06-02",
				MinMessages: 20,
				Limit:       100,
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page, err := d.ListSessions(ctx, tt.filter)
			if err != nil {
				t.Fatalf("ListSessions: %v", err)
			}
			if len(page.Sessions) != tt.want {
				t.Errorf("got %d sessions, want %d",
					len(page.Sessions), tt.want)
			}
		})
	}
}
