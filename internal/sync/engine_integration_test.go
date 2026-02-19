package sync_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wesm/agentsview/internal/db"
	"github.com/wesm/agentsview/internal/sync"
)

type testEnv struct {
	claudeDir string
	codexDir  string
	db        *db.DB
	engine    *sync.Engine
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := &testEnv{
		claudeDir: t.TempDir(),
		codexDir:  t.TempDir(),
	}

	dbPath := filepath.Join(t.TempDir(), "test.db")
	var err error
	env.db, err = db.Open(dbPath)
	if err != nil {
		t.Fatalf("Open DB: %v", err)
	}
	t.Cleanup(func() { env.db.Close() })

	env.engine = sync.NewEngine(
		env.db, env.claudeDir, env.codexDir, "local",
	)
	return env
}

// writeClaudeSession creates a JSONL session file under the
// Claude projects directory and returns the full file path.
func (e *testEnv) writeClaudeSession(
	t *testing.T, projName, filename, content string,
) string {
	t.Helper()
	dir := filepath.Join(e.claudeDir, projName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

// writeCodexSession creates a JSONL session file under the
// Codex date-based directory and returns the full file path.
func (e *testEnv) writeCodexSession(
	t *testing.T, dayPath, filename, content string,
) string {
	t.Helper()
	dir := filepath.Join(e.codexDir, dayPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestSyncEngineIntegration(t *testing.T) {
	env := setupTestEnv(t)

	content := NewSessionBuilder().
		AddClaudeUser("2024-01-01T10:00:00Z", "Hello", "/Users/wesm/code/my-app").
		AddClaudeAssistant("2024-01-01T10:00:05Z", "Hi there!").
		String()

	env.writeClaudeSession(
		t, "-Users-wesm-code-my-app",
		"test-session.jsonl", content,
	)

	// First sync should parse
	stats := runSyncAndAssert(t, env.engine, 1, 0)
	if stats.TotalSessions != 1 {
		t.Errorf("total = %d, want 1", stats.TotalSessions)
	}

	// Verify session was stored
	assertSessionState(t, env.db, "test-session", func(sess *db.Session) {
		if sess.Project != "my_app" {
			t.Errorf("project = %q, want %q",
				sess.Project, "my_app")
		}
		if sess.MessageCount != 2 {
			t.Errorf("message_count = %d, want 2",
				sess.MessageCount)
		}
	})

	// Verify messages
	msgs, err := env.db.GetAllMessages(
		context.Background(), "test-session",
	)
	if err != nil {
		t.Fatalf("GetAllMessages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}
	if msgs[0].Role != "user" {
		t.Errorf("msg[0].Role = %q", msgs[0].Role)
	}
	if msgs[1].Role != "assistant" {
		t.Errorf("msg[1].Role = %q", msgs[1].Role)
	}

	// Second sync should skip (unchanged files)
	runSyncAndAssert(t, env.engine, 0, 1)

	// FindSourceFile
	src := env.engine.FindSourceFile("test-session")
	if src == "" {
		t.Error("FindSourceFile returned empty")
	}
}

func TestSyncEngineCodex(t *testing.T) {
	env := setupTestEnv(t)

	content := NewSessionBuilder().
		AddCodexMeta("2024-01-01T10:00:00Z", "test-uuid", "/home/user/code/api", "user").
		AddCodexMessage("2024-01-01T10:00:01Z", "user", "Add tests").
		AddCodexMessage("2024-01-01T10:00:05Z", "assistant", "Adding test coverage.").
		String()

	env.writeCodexSession(
		t, filepath.Join("2024", "01", "15"),
		"rollout-20240115-test-uuid.jsonl", content,
	)

	stats := env.engine.SyncAll(nil)
	if stats.TotalSessions != 1 {
		t.Errorf("total = %d, want 1", stats.TotalSessions)
	}

	assertSessionState(t, env.db, "codex:test-uuid", func(sess *db.Session) {
		if sess.Agent != "codex" {
			t.Errorf("agent = %q", sess.Agent)
		}
		if sess.Project != "api" {
			t.Errorf("project = %q", sess.Project)
		}
	})
}

func TestSyncEngineProgress(t *testing.T) {
	env := setupTestEnv(t)

	msg := NewSessionBuilder().
		AddClaudeUser("2024-01-01T00:00:00Z", "msg").
		String()

	for _, name := range []string{"a", "b", "c"} {
		env.writeClaudeSession(
			t, "test-proj", name+".jsonl", msg,
		)
	}

	var progressCalls int
	env.engine.SyncAll(func(p sync.Progress) {
		progressCalls++
	})

	if progressCalls == 0 {
		t.Error("expected progress callbacks")
	}
}

func TestSyncEngineHashSkip(t *testing.T) {
	env := setupTestEnv(t)

	content := NewSessionBuilder().
		AddClaudeUser("2024-01-01T00:00:00Z", "msg1").
		String()

	path := env.writeClaudeSession(
		t, "test-proj", "hash-test.jsonl", content,
	)

	// First sync
	runSyncAndAssert(t, env.engine, 1, 0)

	// Verify hash was stored
	size, hash, ok := env.db.GetSessionFileInfo("hash-test")
	if !ok {
		t.Fatal("file info not stored")
	}
	if hash == "" {
		t.Fatal("hash not stored")
	}
	if size == 0 {
		t.Fatal("size not stored")
	}

	// Second sync — unchanged content → skipped
	runSyncAndAssert(t, env.engine, 0, 1)

	// Overwrite with same-size but different content.
	different := NewSessionBuilder().
		AddClaudeUser("2024-01-01T00:00:00Z", "msg2").
		String()

	if len(different) != len(content) {
		for len(different) < len(content) {
			different += " "
		}
		different = different[:len(content)]
	}
	os.WriteFile(path, []byte(different), 0o644)

	// Third sync — same size, different hash → re-synced
	runSyncAndAssert(t, env.engine, 1, 0)
}

func TestSyncEngineTombstone(t *testing.T) {
	env := setupTestEnv(t)

	// Write malformed content that produces 0 valid messages
	path := env.writeClaudeSession(
		t, "test-proj", "tombstone-test.jsonl",
		"not json at all\x00\x01",
	)

	// First sync — 0 valid messages
	stats := env.engine.SyncAll(nil)
	if stats.TotalSessions != 1 {
		t.Fatalf("total = %d, want 1", stats.TotalSessions)
	}

	// Second sync — unchanged, should be skipped
	runSyncAndAssert(t, env.engine, 0, 1)

	// Touch file (change mtime) but keep same content
	time.Sleep(10 * time.Millisecond)
	os.Chtimes(path, time.Now(), time.Now())

	// Third sync — mtime changed but hash same → still skipped
	runSyncAndAssert(t, env.engine, 0, 1)
}

func TestSyncEngineFileAppend(t *testing.T) {
	env := setupTestEnv(t)

	initial := NewSessionBuilder().
		AddClaudeUser("2024-01-01T00:00:00Z", "first").
		String()

	path := env.writeClaudeSession(
		t, "test-proj", "append-test.jsonl", initial,
	)

	// First sync
	runSyncAndAssert(t, env.engine, 1, 0)

	assertSessionState(t, env.db, "append-test", func(sess *db.Session) {
		if sess.MessageCount != 1 {
			t.Fatalf("initial message_count = %d, want 1",
				sess.MessageCount)
		}
	})

	// Append a new message (changes size and hash)
	appended := initial + NewSessionBuilder().
		AddClaudeAssistant("2024-01-01T00:00:05Z", "reply").
		String()

	os.WriteFile(path, []byte(appended), 0o644)

	// Re-sync — different size → re-synced
	runSyncAndAssert(t, env.engine, 1, 0)

	assertSessionState(t, env.db, "append-test", func(sess *db.Session) {
		if sess.MessageCount != 2 {
			t.Errorf("updated message_count = %d, want 2",
				sess.MessageCount)
		}
	})
}

func TestSyncSingleSessionHash(t *testing.T) {
	env := setupTestEnv(t)

	content := NewSessionBuilder().
		AddClaudeUser("2024-01-01T00:00:00Z", "hello").
		AddClaudeAssistant("2024-01-01T00:00:05Z", "hi").
		String()

	env.writeClaudeSession(
		t, "test-proj", "single-hash.jsonl", content,
	)

	// Initial full sync to discover the session
	env.engine.SyncAll(nil)

	// Clear hash to simulate pre-hash DB state
	clearSessionHash(t, env.db, "single-hash")

	// Verify precondition: hash is cleared
	_, preHash, preOK := env.db.GetSessionFileInfo(
		"single-hash",
	)
	if !preOK {
		t.Fatal("session file info missing after hash clear")
	}
	if preHash != "" {
		t.Fatalf("precondition failed: hash = %q, want empty",
			preHash)
	}

	// SyncSingleSession should compute and store hash
	err := env.engine.SyncSingleSession("single-hash")
	if err != nil {
		t.Fatalf("SyncSingleSession: %v", err)
	}

	_, hash, ok := env.db.GetSessionFileInfo("single-hash")
	if !ok {
		t.Fatal("session file info not found")
	}
	if hash == "" {
		t.Error("SyncSingleSession did not store file hash")
	}

	// Subsequent SyncAll should skip (hash matches)
	runSyncAndAssert(t, env.engine, 0, 1)
}

func TestSyncSingleSessionHashCodex(t *testing.T) {
	env := setupTestEnv(t)

	uuid := "a1b2c3d4-1234-5678-9abc-def012345678"
	content := NewSessionBuilder().
		AddCodexMeta("2024-01-01T10:00:00Z", uuid, "/home/user/code/api", "user").
		AddCodexMessage("2024-01-01T10:00:01Z", "user", "Add tests").
		AddCodexMessage("2024-01-01T10:00:05Z", "assistant", "Adding test coverage.").
		String()

	env.writeCodexSession(
		t, filepath.Join("2024", "01", "15"),
		"rollout-20240115-"+uuid+".jsonl", content,
	)

	sessionID := "codex:" + uuid

	// Initial full sync to discover the session
	env.engine.SyncAll(nil)

	assertSessionState(t, env.db, sessionID, nil)

	// Clear hash to simulate pre-hash DB state
	clearSessionHash(t, env.db, sessionID)

	// Verify precondition: hash is cleared
	_, preHash, preOK := env.db.GetSessionFileInfo(sessionID)
	if !preOK {
		t.Fatal("session file info missing after hash clear")
	}
	if preHash != "" {
		t.Fatalf("precondition failed: hash = %q, want empty",
			preHash)
	}

	// SyncSingleSession should compute and store hash
	err := env.engine.SyncSingleSession(sessionID)
	if err != nil {
		t.Fatalf("SyncSingleSession: %v", err)
	}

	_, hash, ok := env.db.GetSessionFileInfo(sessionID)
	if !ok {
		t.Fatal("session file info not found")
	}
	if hash == "" {
		t.Error(
			"SyncSingleSession did not store file hash " +
				"for codex session",
		)
	}

	// Subsequent SyncAll should skip (hash matches)
	runSyncAndAssert(t, env.engine, 0, 1)
}

func TestSyncEngineTombstoneClearOnMtimeChange(t *testing.T) {
	env := setupTestEnv(t)

	// Write something that produces 0 messages but parses OK
	path := env.writeClaudeSession(
		t, "test-proj", "tombstone-clear.jsonl", "garbage\n",
	)

	// First sync
	env.engine.SyncAll(nil)

	// Replace with valid content
	valid := NewSessionBuilder().
		AddClaudeUser("2024-01-01T00:00:00Z", "hello").
		AddClaudeAssistant("2024-01-01T00:00:05Z", "hi").
		String()

	os.WriteFile(path, []byte(valid), 0o644)

	// Re-sync — content changed (different size) → re-synced
	runSyncAndAssert(t, env.engine, 1, 0)

	assertSessionState(t, env.db, "tombstone-clear", func(sess *db.Session) {
		if sess.MessageCount != 2 {
			t.Errorf("message_count = %d, want 2",
				sess.MessageCount)
		}
	})
}

func TestSyncSingleSessionProjectFallback(t *testing.T) {
	env := setupTestEnv(t)

	// 1. Create a session in a directory "default-proj"
	content := NewSessionBuilder().
		AddClaudeUser("2024-01-01T00:00:00Z", "hello").
		String()

	env.writeClaudeSession(
		t, "default-proj", "fallback-test.jsonl", content,
	)

	// 2. Initial sync - should get "default-proj"
	env.engine.SyncAll(nil)

	assertSessionState(t, env.db, "fallback-test", func(sess *db.Session) {
		if sess.Project != "default_proj" {
			t.Fatalf("initial project = %q, want %q", sess.Project, "default_proj")
		}
	})

	sess, err := env.db.GetSessionFull(context.Background(), "fallback-test")
	if err != nil {
		t.Fatalf("GetSessionFull: %v", err)
	}
	if sess == nil {
		t.Fatal("session not found")
	}

	// 3. Manually update project to "custom-proj"
	// This simulates a user override or a previous state we want to preserve
	err = env.db.UpsertSession(db.Session{
		ID:           sess.ID,
		Project:      "custom_proj", // <--- CHANGED
		Machine:      sess.Machine,
		Agent:        sess.Agent,
		MessageCount: sess.MessageCount,
		FilePath:     sess.FilePath,
		FileSize:     sess.FileSize,
		FileMtime:    sess.FileMtime,
		FileHash:     sess.FileHash,
		FirstMessage: sess.FirstMessage,
		StartedAt:    sess.StartedAt,
		EndedAt:      sess.EndedAt,
		CreatedAt:    sess.CreatedAt,
	})
	if err != nil {
		t.Fatalf("manual update: %v", err)
	}

	// Verify manual update worked
	assertSessionState(t, env.db, "fallback-test", func(sess *db.Session) {
		if sess.Project != "custom_proj" {
			t.Fatalf("manual update failed, project = %q", sess.Project)
		}
	})

	// 4. Run SyncSingleSession
	// This should NOT revert to "default-proj" if we fix the bug
	err = env.engine.SyncSingleSession("fallback-test")
	if err != nil {
		t.Fatalf("SyncSingleSession: %v", err)
	}

	// 5. Verify project is still "custom-proj"
	assertSessionState(t, env.db, "fallback-test", func(sess *db.Session) {
		if sess.Project != "custom_proj" {
			t.Errorf("regression: project reverted to %q, want %q", sess.Project, "custom_proj")
		}
	})

	// ==========================================
	// New test cases for Empty and Bad Project
	// ==========================================

	// Case A: Empty Project -> Should fall back to directory
	err = env.db.UpsertSession(db.Session{
		ID:           sess.ID,
		Project:      "", // Empty
		Machine:      sess.Machine,
		Agent:        sess.Agent,
		MessageCount: sess.MessageCount,
		FilePath:     sess.FilePath,
		FileSize:     sess.FileSize,
		FileMtime:    sess.FileMtime,
		FileHash:     sess.FileHash,
		FirstMessage: sess.FirstMessage,
		StartedAt:    sess.StartedAt,
		EndedAt:      sess.EndedAt,
		CreatedAt:    sess.CreatedAt,
	})
	if err != nil {
		t.Fatalf("manual update empty: %v", err)
	}

	err = env.engine.SyncSingleSession("fallback-test")
	if err != nil {
		t.Fatalf("SyncSingleSession (empty): %v", err)
	}

	assertSessionState(t, env.db, "fallback-test", func(sess *db.Session) {
		if sess.Project != "default_proj" {
			t.Errorf("empty project fallback: got %q, want %q", sess.Project, "default_proj")
		}
	})

	// Case B: Project needing reparse -> Should fall back to directory
	badProject := "_Users_wesm_bad"
	err = env.db.UpsertSession(db.Session{
		ID:           sess.ID,
		Project:      badProject,
		Machine:      sess.Machine,
		Agent:        sess.Agent,
		MessageCount: sess.MessageCount,
		FilePath:     sess.FilePath,
		FileSize:     sess.FileSize,
		FileMtime:    sess.FileMtime,
		FileHash:     sess.FileHash,
		FirstMessage: sess.FirstMessage,
		StartedAt:    sess.StartedAt,
		EndedAt:      sess.EndedAt,
		CreatedAt:    sess.CreatedAt,
	})
	if err != nil {
		t.Fatalf("manual update bad: %v", err)
	}

	err = env.engine.SyncSingleSession("fallback-test")
	if err != nil {
		t.Fatalf("SyncSingleSession (bad): %v", err)
	}

	assertSessionState(t, env.db, "fallback-test", func(sess *db.Session) {
		if sess.Project != "default_proj" {
			t.Errorf("bad project fallback: got %q, want %q", sess.Project, "default_proj")
		}
	})
}

func TestSyncEngineNoTrailingNewline(t *testing.T) {
	env := setupTestEnv(t)

	content := NewSessionBuilder().
		AddClaudeUser("2024-01-01T10:00:00Z", "Hello").
		StringNoTrailingNewline()

	env.writeClaudeSession(
		t, "test-proj", "no-newline.jsonl", content,
	)

	// Sync should succeed
	runSyncAndAssert(t, env.engine, 1, 0)

	assertSessionState(t, env.db, "no-newline", func(sess *db.Session) {
		if sess.MessageCount != 1 {
			t.Errorf("message_count = %d, want 1", sess.MessageCount)
		}
	})
}

func TestSyncEngineCodexNoTrailingNewline(t *testing.T) {
	env := setupTestEnv(t)

	uuid := "b2c3d4e5-2345-6789-0abc-def123456789"
	content := NewSessionBuilder().
		AddCodexMeta("2024-01-01T10:00:00Z", uuid, "/home/user/code/api", "user").
		AddCodexMessage("2024-01-01T10:00:01Z", "user", "Hello").
		StringNoTrailingNewline()

	env.writeCodexSession(
		t, filepath.Join("2024", "01", "15"),
		"rollout-20240115-"+uuid+".jsonl", content,
	)

	// Sync should succeed
	runSyncAndAssert(t, env.engine, 1, 0)

	assertSessionState(t, env.db, "codex:"+uuid, func(sess *db.Session) {
		if sess.MessageCount != 1 {
			t.Errorf("message_count = %d, want 1", sess.MessageCount)
		}
	})
}
