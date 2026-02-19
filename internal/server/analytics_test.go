package server_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/wesm/agentsview/internal/db"
)

// seedAnalyticsEnv populates the test env with sessions and
// messages suitable for analytics endpoint tests.
func seedAnalyticsEnv(t *testing.T, te *testEnv) {
	t.Helper()

	sessions := []struct {
		id      string
		project string
		agent   string
		started string
		msgs    int
	}{
		{"a1", "alpha", "claude", "2024-06-01T09:00:00Z", 10},
		{"a2", "alpha", "codex", "2024-06-01T14:00:00Z", 20},
		{"b1", "beta", "claude", "2024-06-02T10:00:00Z", 30},
	}

	for _, s := range sessions {
		ended := s.started // close enough for tests
		first := "Hello"
		sess := db.Session{
			ID:           s.id,
			Project:      s.project,
			Machine:      "test",
			Agent:        s.agent,
			MessageCount: s.msgs,
			StartedAt:    &s.started,
			EndedAt:      &ended,
			FirstMessage: &first,
		}
		if err := te.db.UpsertSession(sess); err != nil {
			t.Fatalf("seeding session %s: %v", s.id, err)
		}

		msgs := make([]db.Message, s.msgs)
		for i := range s.msgs {
			role := "user"
			if i%2 == 1 {
				role = "assistant"
			}
			msgs[i] = db.Message{
				SessionID:     s.id,
				Ordinal:       i,
				Role:          role,
				Content:       fmt.Sprintf("msg %d", i),
				ContentLength: 5,
				Timestamp:     s.started,
			}
		}
		if err := te.db.ReplaceSessionMessages(
			s.id, msgs,
		); err != nil {
			t.Fatalf("seeding messages for %s: %v", s.id, err)
		}
	}
}

func TestAnalyticsSummary(t *testing.T) {
	te := setup(t)
	seedAnalyticsEnv(t, te)

	t.Run("OK", func(t *testing.T) {
		w := te.get(t,
			"/api/v1/analytics/summary?from=2024-06-01&to=2024-06-03&timezone=UTC")
		assertStatus(t, w, http.StatusOK)

		resp := decode[db.AnalyticsSummary](t, w)
		if resp.TotalSessions != 3 {
			t.Errorf("TotalSessions = %d, want 3",
				resp.TotalSessions)
		}
		if resp.TotalMessages != 60 {
			t.Errorf("TotalMessages = %d, want 60",
				resp.TotalMessages)
		}
		if resp.ActiveProjects != 2 {
			t.Errorf("ActiveProjects = %d, want 2",
				resp.ActiveProjects)
		}
	})

	t.Run("DefaultDateRange", func(t *testing.T) {
		w := te.get(t, "/api/v1/analytics/summary")
		assertStatus(t, w, http.StatusOK)
		// Should not error — defaults to last 30 days
	})

	t.Run("InvalidTimezone", func(t *testing.T) {
		w := te.get(t,
			"/api/v1/analytics/summary?timezone=Fake/Zone")
		assertStatus(t, w, http.StatusBadRequest)
	})
}

func TestAnalyticsActivity(t *testing.T) {
	te := setup(t)
	seedAnalyticsEnv(t, te)

	t.Run("DayGranularity", func(t *testing.T) {
		w := te.get(t,
			"/api/v1/analytics/activity?from=2024-06-01&to=2024-06-03&granularity=day")
		assertStatus(t, w, http.StatusOK)

		resp := decode[db.ActivityResponse](t, w)
		if resp.Granularity != "day" {
			t.Errorf("Granularity = %q, want day",
				resp.Granularity)
		}
		if len(resp.Series) == 0 {
			t.Fatal("expected non-empty series")
		}
	})

	t.Run("WeekGranularity", func(t *testing.T) {
		w := te.get(t,
			"/api/v1/analytics/activity?from=2024-06-01&to=2024-06-03&granularity=week")
		assertStatus(t, w, http.StatusOK)
	})

	t.Run("DefaultGranularity", func(t *testing.T) {
		w := te.get(t,
			"/api/v1/analytics/activity?from=2024-06-01&to=2024-06-03")
		assertStatus(t, w, http.StatusOK)

		resp := decode[db.ActivityResponse](t, w)
		if resp.Granularity != "day" {
			t.Errorf("default granularity = %q, want day",
				resp.Granularity)
		}
	})

	t.Run("InvalidGranularity", func(t *testing.T) {
		w := te.get(t,
			"/api/v1/analytics/activity?granularity=hour")
		assertStatus(t, w, http.StatusBadRequest)
	})
}

func TestAnalyticsHeatmap(t *testing.T) {
	te := setup(t)
	seedAnalyticsEnv(t, te)

	t.Run("MessageMetric", func(t *testing.T) {
		w := te.get(t,
			"/api/v1/analytics/heatmap?from=2024-06-01&to=2024-06-03&metric=messages")
		assertStatus(t, w, http.StatusOK)

		resp := decode[db.HeatmapResponse](t, w)
		if resp.Metric != "messages" {
			t.Errorf("Metric = %q, want messages", resp.Metric)
		}
		if len(resp.Entries) != 3 {
			t.Errorf("len(Entries) = %d, want 3",
				len(resp.Entries))
		}
	})

	t.Run("SessionMetric", func(t *testing.T) {
		w := te.get(t,
			"/api/v1/analytics/heatmap?from=2024-06-01&to=2024-06-03&metric=sessions")
		assertStatus(t, w, http.StatusOK)

		resp := decode[db.HeatmapResponse](t, w)
		if resp.Metric != "sessions" {
			t.Errorf("Metric = %q, want sessions", resp.Metric)
		}
	})

	t.Run("DefaultMetric", func(t *testing.T) {
		w := te.get(t,
			"/api/v1/analytics/heatmap?from=2024-06-01&to=2024-06-03")
		assertStatus(t, w, http.StatusOK)

		resp := decode[db.HeatmapResponse](t, w)
		if resp.Metric != "messages" {
			t.Errorf("default metric = %q, want messages",
				resp.Metric)
		}
	})

	t.Run("InvalidMetric", func(t *testing.T) {
		w := te.get(t,
			"/api/v1/analytics/heatmap?metric=bytes")
		assertStatus(t, w, http.StatusBadRequest)
	})
}

func TestAnalyticsProjects(t *testing.T) {
	te := setup(t)
	seedAnalyticsEnv(t, te)

	t.Run("OK", func(t *testing.T) {
		w := te.get(t,
			"/api/v1/analytics/projects?from=2024-06-01&to=2024-06-03")
		assertStatus(t, w, http.StatusOK)

		resp := decode[db.ProjectsAnalyticsResponse](t, w)
		if len(resp.Projects) != 2 {
			t.Fatalf("len(Projects) = %d, want 2",
				len(resp.Projects))
		}
		// Sorted by messages desc: beta (30) > alpha (30)
		// Both are 30 — either order is fine, just check counts
		total := 0
		for _, p := range resp.Projects {
			total += p.Messages
		}
		if total != 60 {
			t.Errorf("total messages = %d, want 60", total)
		}
	})

	t.Run("MachineFilter", func(t *testing.T) {
		w := te.get(t,
			"/api/v1/analytics/projects?from=2024-06-01&to=2024-06-03&machine=nonexistent")
		assertStatus(t, w, http.StatusOK)

		resp := decode[db.ProjectsAnalyticsResponse](t, w)
		if len(resp.Projects) != 0 {
			t.Errorf("len(Projects) = %d, want 0",
				len(resp.Projects))
		}
	})
}
