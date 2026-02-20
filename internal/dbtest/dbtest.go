// Package dbtest provides shared test helpers for database
// setup and session seeding across test packages.
package dbtest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wesm/agentsview/internal/db"
)

// Ptr returns a pointer to v.
func Ptr[T any](v T) *T { return &v }

// WriteTestFile creates a file at path with the given content,
// creating parent directories as needed. Fails the test on
// any error.
func WriteTestFile(
	t *testing.T, path string, content []byte,
) {
	t.Helper()
	if err := os.MkdirAll(
		filepath.Dir(path), 0o755,
	); err != nil {
		t.Fatalf("MkdirAll %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

// OpenTestDB creates a temporary SQLite database for testing.
// The database is automatically closed when the test completes.
func OpenTestDB(t *testing.T) *db.DB {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	d, err := db.Open(path)
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// SeedMessages inserts messages into the database, failing the
// test on error.
func SeedMessages(t *testing.T, d *db.DB, msgs ...db.Message) {
	t.Helper()
	if err := d.InsertMessages(msgs); err != nil {
		t.Fatalf("SeedMessages: %v", err)
	}
}

// UserMsg creates a user message for the given session.
func UserMsg(
	sid string, ordinal int, content string,
) db.Message {
	return db.Message{
		SessionID:     sid,
		Ordinal:       ordinal,
		Role:          "user",
		Content:       content,
		ContentLength: len(content),
	}
}

// AsstMsg creates an assistant message for the given session.
func AsstMsg(
	sid string, ordinal int, content string,
) db.Message {
	return db.Message{
		SessionID:     sid,
		Ordinal:       ordinal,
		Role:          "assistant",
		Content:       content,
		ContentLength: len(content),
	}
}

// SeedSession creates and upserts a session with sensible
// defaults. Override any field via the opts functions.
func SeedSession(
	t *testing.T, d *db.DB, id, project string,
	opts ...func(*db.Session),
) {
	t.Helper()
	s := db.Session{
		ID:           id,
		Project:      project,
		Machine:      "local",
		Agent:        "claude",
		MessageCount: 1,
	}
	for _, opt := range opts {
		opt(&s)
	}
	if err := d.UpsertSession(s); err != nil {
		t.Fatalf("SeedSession %s: %v", id, err)
	}
}
