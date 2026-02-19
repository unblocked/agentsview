package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wesm/agentsview/internal/config"
	"github.com/wesm/agentsview/internal/db"
	"github.com/wesm/agentsview/internal/sync"
)

// setupInternal creates a Server for internal testing.
// It bypasses the public New() wrapper logic to focus on internal components if needed,
// but uses New() to ensure correct initialization.
func setupInternal(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	claudeDir := filepath.Join(dir, "claude")
	codexDir := filepath.Join(dir, "codex")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("creating claude dir: %v", err)
	}
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatalf("creating codex dir: %v", err)
	}

	cfg := config.Config{
		Host:         "127.0.0.1",
		Port:         0,
		DataDir:      dir,
		DBPath:       dbPath,
		WriteTimeout: 30 * time.Second,
	}
	engine := sync.NewEngine(
		database, claudeDir, codexDir, "test",
	)
	return New(cfg, database, engine)
}

func TestHandlers_Internal_DeadlineExceeded(t *testing.T) {
	s := setupInternal(t)

	// Seed a session just in case handlers check for existence before context.
	// We'll use the public methods on db to seed.
	started := "2025-01-15T10:00:00Z"
	sess := db.Session{
		ID:        "s1",
		Project:   "test-proj",
		StartedAt: &started,
	}
	if err := s.db.UpsertSession(sess); err != nil {
		t.Fatalf("seeding session: %v", err)
	}

	tests := []struct {
		name    string
		handler func(http.ResponseWriter, *http.Request)
	}{
		{"ListSessions", s.handleListSessions},
		{"GetSession", s.handleGetSession},
		{"GetMessages", s.handleGetMessages},
		{"GetMinimap", s.handleGetMinimap},
		{"GetStats", s.handleGetStats},
		{"ListProjects", s.handleListProjects},
		{"ListMachines", s.handleListMachines},
		{"Search", s.handleSearch},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "Search" && !s.db.HasFTS() {
				t.Skip("skipping search test: no FTS support")
			}
			// Create a context that is already past its deadline
			ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Hour))
			defer cancel()

			req := httptest.NewRequest("GET", "/?q=test", nil)
			req.SetPathValue("id", "s1")
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()

			// Call handler directly, bypassing middleware
			tt.handler(w, req)

			// Expect 504 Gateway Timeout
			if w.Code != http.StatusGatewayTimeout {
				t.Errorf("expected status 504, got %d. Body: %s", w.Code, w.Body.String())
			}

			// Verify Content-Type is JSON (except for empty responses, but errors should be JSON)
			if contentType := w.Header().Get("Content-Type"); contentType != "application/json" {
				t.Errorf("expected Content-Type application/json, got %s", contentType)
			}
		})
	}
}
