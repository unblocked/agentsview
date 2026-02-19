package sync_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/wesm/agentsview/internal/db"
	"github.com/wesm/agentsview/internal/sync"
)

// Timestamp constants for test data.
const (
	tsZero    = "2024-01-01T00:00:00Z"
	tsZeroS5  = "2024-01-01T00:00:05Z"
	tsEarly   = "2024-01-01T10:00:00Z"
	tsEarlyS1 = "2024-01-01T10:00:01Z"
	tsEarlyS5 = "2024-01-01T10:00:05Z"
)

// --- Assertion Helpers ---

func assertSessionState(t *testing.T, database *db.DB, sessionID string, check func(*db.Session)) {
	t.Helper()
	sess, err := database.GetSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("GetSession(%q): %v", sessionID, err)
	}
	if sess == nil {
		t.Fatalf("Session %q not found", sessionID)
	}
	if check != nil {
		check(sess)
	}
}

func runSyncAndAssert(t *testing.T, engine *sync.Engine, wantSynced, wantSkipped int) sync.SyncStats {
	t.Helper()
	stats := engine.SyncAll(nil)
	if stats.Synced != wantSynced {
		t.Fatalf("Synced: got %d, want %d", stats.Synced, wantSynced)
	}
	if stats.Skipped != wantSkipped {
		t.Fatalf("Skipped: got %d, want %d", stats.Skipped, wantSkipped)
	}
	return stats
}

func clearSessionHash(t *testing.T, database *db.DB, sessionID string) {
	t.Helper()
	err := database.Update(func(tx *sql.Tx) error {
		_, err := tx.Exec("UPDATE sessions SET file_hash = NULL WHERE id = ?", sessionID)
		return err
	})
	if err != nil {
		t.Fatalf("failed to clear hash for %s: %v", sessionID, err)
	}
}

// assertHashRoundTrip clears the session hash, verifies the
// precondition, runs SyncSingleSession to recompute the hash,
// and asserts the hash is stored and a subsequent SyncAll skips.
func (e *testEnv) assertHashRoundTrip(
	t *testing.T, sessionID string,
) {
	t.Helper()

	clearSessionHash(t, e.db, sessionID)

	_, preHash, preOK := e.db.GetSessionFileInfo(sessionID)
	if !preOK {
		t.Fatal("session file info missing after hash clear")
	}
	if preHash != "" {
		t.Fatalf(
			"precondition failed: hash = %q, want empty",
			preHash,
		)
	}

	if err := e.engine.SyncSingleSession(sessionID); err != nil {
		t.Fatalf("SyncSingleSession: %v", err)
	}

	_, hash, ok := e.db.GetSessionFileInfo(sessionID)
	if !ok {
		t.Fatal("session file info not found")
	}
	if hash == "" {
		t.Error("SyncSingleSession did not store file hash")
	}

	runSyncAndAssert(t, e.engine, 0, 1)
}

// assertMessageRoles verifies that a session's messages have
// the expected roles in order.
func assertMessageRoles(
	t *testing.T, database *db.DB,
	sessionID string, wantRoles ...string,
) {
	t.Helper()
	msgs, err := database.GetAllMessages(
		context.Background(), sessionID,
	)
	if err != nil {
		t.Fatalf("GetAllMessages(%q): %v", sessionID, err)
	}
	if len(msgs) != len(wantRoles) {
		t.Fatalf("got %d messages, want %d",
			len(msgs), len(wantRoles))
	}
	for i, want := range wantRoles {
		if msgs[i].Role != want {
			t.Errorf("msgs[%d].Role = %q, want %q",
				i, msgs[i].Role, want)
		}
	}
}

// updateSessionProject fetches the session, updates its
// Project field, and upserts it back. Reduces boilerplate
// for tests that need to override a single field.
func (e *testEnv) updateSessionProject(
	t *testing.T, sessionID, project string,
) {
	t.Helper()
	sess, err := e.db.GetSessionFull(
		context.Background(), sessionID,
	)
	if err != nil {
		t.Fatalf("GetSessionFull: %v", err)
	}
	if sess == nil {
		t.Fatalf("session %q not found", sessionID)
	}
	sess.Project = project
	if err := e.db.UpsertSession(*sess); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}
}
