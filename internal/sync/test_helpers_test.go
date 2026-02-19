package sync_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"github.com/wesm/agentsview/internal/db"
	"github.com/wesm/agentsview/internal/sync"
)

// --- Session Builder ---

type SessionBuilder struct {
	lines []string
}

func NewSessionBuilder() *SessionBuilder {
	return &SessionBuilder{}
}

func (b *SessionBuilder) AddClaudeUser(timestamp, content string, cwd ...string) *SessionBuilder {
	m := map[string]any{
		"type":      "user",
		"timestamp": timestamp,
		"message": map[string]any{
			"content": content,
		},
	}
	if len(cwd) > 0 {
		m["cwd"] = cwd[0]
	}
	line, _ := json.Marshal(m)
	b.lines = append(b.lines, string(line))
	return b
}

func (b *SessionBuilder) AddClaudeAssistant(timestamp, text string) *SessionBuilder {
	m := map[string]any{
		"type":      "assistant",
		"timestamp": timestamp,
		"message": map[string]any{
			"content": []map[string]string{
				{
					"type": "text",
					"text": text,
				},
			},
		},
	}
	line, _ := json.Marshal(m)
	b.lines = append(b.lines, string(line))
	return b
}

func (b *SessionBuilder) AddCodexMeta(timestamp, id, cwd, originator string) *SessionBuilder {
	m := map[string]any{
		"type":      "session_meta",
		"timestamp": timestamp,
		"payload": map[string]any{
			"id":         id,
			"cwd":        cwd,
			"originator": originator,
		},
	}
	line, _ := json.Marshal(m)
	b.lines = append(b.lines, string(line))
	return b
}

func (b *SessionBuilder) AddCodexMessage(timestamp, role, text string) *SessionBuilder {
	contentType := "output_text"
	if role == "user" {
		contentType = "input_text"
	}
	m := map[string]any{
		"type":      "response_item",
		"timestamp": timestamp,
		"payload": map[string]any{
			"role": role,
			"content": []map[string]string{
				{
					"type": contentType,
					"text": text,
				},
			},
		},
	}
	line, _ := json.Marshal(m)
	b.lines = append(b.lines, string(line))
	return b
}

func (b *SessionBuilder) AddRaw(line string) *SessionBuilder {
	b.lines = append(b.lines, line)
	return b
}

func (b *SessionBuilder) String() string {
	return strings.Join(b.lines, "\n") + "\n"
}

func (b *SessionBuilder) StringNoTrailingNewline() string {
	return strings.Join(b.lines, "\n")
}

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
