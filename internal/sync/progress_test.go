package sync

import (
	"math"
	"testing"
)

func TestSyncStats_RecordSkip(t *testing.T) {
	var s SyncStats
	s.RecordSkip()
	s.RecordSkip()
	assertSyncStats(t, s, 2, 0)
}

func TestSyncStats_RecordSynced(t *testing.T) {
	var s SyncStats
	s.RecordSynced(5)
	s.RecordSynced(3)
	assertSyncStats(t, s, 0, 8)
}

func TestProgress_Percent(t *testing.T) {
	tests := []struct {
		name string
		p    Progress
		want float64
	}{
		{
			name: "zero total",
			p:    Progress{SessionsTotal: 0, SessionsDone: 0},
			want: 0,
		},
		{
			name: "half done",
			p:    Progress{SessionsTotal: 10, SessionsDone: 5},
			want: 50,
		},
		{
			name: "all done",
			p:    Progress{SessionsTotal: 4, SessionsDone: 4},
			want: 100,
		},
		{
			name: "one third",
			p:    Progress{SessionsTotal: 3, SessionsDone: 1},
			want: 33.333333,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.p.Percent()
			assertFloatEqual(t, got, tt.want)
		})
	}
}

func assertSyncStats(t *testing.T, s SyncStats, wantSkipped, wantSynced int) {
	t.Helper()
	if s.Skipped != wantSkipped {
		t.Errorf("Skipped = %d, want %d", s.Skipped, wantSkipped)
	}
	if s.Synced != wantSynced {
		t.Errorf("Synced = %d, want %d", s.Synced, wantSynced)
	}
}

func assertFloatEqual(t *testing.T, got, want float64) {
	t.Helper()
	const epsilon = 1e-4

	if math.Abs(got-want) > epsilon {
		t.Errorf("got %f, want %f", got, want)
	}
}
