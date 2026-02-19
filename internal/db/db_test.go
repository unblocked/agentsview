package db

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const (
	defaultMachine = "local"
	defaultAgent   = "claude"
)

func testDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	d, err := Open(path)
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// strPtr returns a pointer to s.
func strPtr(s string) *string { return &s }

// int64Ptr returns a pointer to n.
func int64Ptr(n int64) *int64 { return &n }

// intPtr returns a pointer to n.
func intPtr(n int) *int { return &n }

// insertSession creates and upserts a session with sensible
// defaults. Override any field via the opts functions.
func insertSession(
	t *testing.T, d *DB, id, project string,
	opts ...func(*Session),
) {
	t.Helper()
	s := Session{
		ID:           id,
		Project:      project,
		Machine:      defaultMachine,
		Agent:        defaultAgent,
		MessageCount: 1,
	}
	for _, opt := range opts {
		opt(&s)
	}
	if err := d.UpsertSession(s); err != nil {
		t.Fatalf("insertSession %s: %v", id, err)
	}
}

// insertMessages is a helper that inserts messages and fails
// the test on error.
func insertMessages(t *testing.T, d *DB, msgs ...Message) {
	t.Helper()
	if err := d.InsertMessages(msgs); err != nil {
		t.Fatalf("insertMessages: %v", err)
	}
}

// userMsg creates a user message with the given content.
func userMsg(sid string, ordinal int, content string) Message {
	return Message{
		SessionID:     sid,
		Ordinal:       ordinal,
		Role:          "user",
		Content:       content,
		ContentLength: len(content),
		Timestamp:     "2024-01-01T00:00:00Z",
	}
}

// asstMsg creates an assistant message with the given content.
func asstMsg(sid string, ordinal int, content string) Message {
	return Message{
		SessionID:     sid,
		Ordinal:       ordinal,
		Role:          "assistant",
		Content:       content,
		ContentLength: len(content),
		Timestamp:     "2024-01-01T00:00:00Z",
	}
}

// requireSessionExists asserts that a session exists and returns it.
func requireSessionExists(t *testing.T, d *DB, id string) *Session {
	t.Helper()
	s, err := d.GetSession(context.Background(), id)
	if err != nil {
		t.Fatalf("GetSession %q: %v", id, err)
	}
	if s == nil {
		t.Fatalf("session %q should exist", id)
	}
	return s
}

// requireSessionGone asserts that a session does not exist.
func requireSessionGone(t *testing.T, d *DB, id string) {
	t.Helper()
	s, err := d.GetSession(context.Background(), id)
	if err != nil {
		t.Fatalf("GetSession %q: %v", id, err)
	}
	if s != nil {
		t.Fatalf("session %q should be gone", id)
	}
}

func TestOpenCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "test.db")
	d, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("db file not created: %v", err)
	}
}

func TestSessionCRUD(t *testing.T) {
	d := testDB(t)

	s := Session{
		ID:           "test-session-1",
		Project:      "my_project",
		Machine:      defaultMachine,
		Agent:        defaultAgent,
		FirstMessage: strPtr("Hello world"),
		StartedAt:    strPtr("2024-01-01T00:00:00Z"),
		EndedAt:      strPtr("2024-01-01T01:00:00Z"),
		MessageCount: 5,
	}

	if err := d.UpsertSession(s); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}

	got := requireSessionExists(t, d, "test-session-1")
	if got.Project != "my_project" {
		t.Errorf("project = %q, want %q", got.Project, "my_project")
	}
	if got.MessageCount != 5 {
		t.Errorf("message_count = %d, want 5", got.MessageCount)
	}

	// Update
	s.MessageCount = 10
	if err := d.UpsertSession(s); err != nil {
		t.Fatalf("UpsertSession update: %v", err)
	}
	got = requireSessionExists(t, d, "test-session-1")
	if got.MessageCount != 10 {
		t.Errorf("after update: message_count = %d, want 10",
			got.MessageCount)
	}

	// Get nonexistent
	requireSessionGone(t, d, "nonexistent")
}

func TestListSessions(t *testing.T) {
	d := testDB(t)

	for i := range 5 {
		ea := fmt.Sprintf("2024-01-01T0%d:00:00Z", i)
		insertSession(t, d,
			fmt.Sprintf("session-%c", 'a'+i), "proj",
			func(s *Session) {
				s.EndedAt = strPtr(ea)
				s.MessageCount = i + 1
			},
		)
	}

	page, err := d.ListSessions(context.Background(), SessionFilter{Limit: 10})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(page.Sessions) != 5 {
		t.Errorf("got %d sessions, want 5", len(page.Sessions))
	}

	page, err = d.ListSessions(context.Background(), SessionFilter{Limit: 2})
	if err != nil {
		t.Fatalf("ListSessions limit: %v", err)
	}
	if len(page.Sessions) != 2 {
		t.Errorf("got %d sessions, want 2", len(page.Sessions))
	}
	if page.NextCursor == "" {
		t.Error("expected next cursor")
	}

	page2, err := d.ListSessions(context.Background(), SessionFilter{
		Limit:  10,
		Cursor: page.NextCursor,
	})
	if err != nil {
		t.Fatalf("ListSessions cursor: %v", err)
	}
	if len(page2.Sessions) != 3 {
		t.Errorf("got %d sessions, want 3", len(page2.Sessions))
	}
}

func TestListSessionsProjectFilter(t *testing.T) {
	d := testDB(t)

	for i, proj := range []string{"proj_a", "proj_a", "proj_b"} {
		ea := fmt.Sprintf("2024-01-01T00:00:0%dZ", i)
		insertSession(t, d,
			fmt.Sprintf("%s-%d", proj, i), proj,
			func(s *Session) { s.EndedAt = strPtr(ea) },
		)
	}

	page, _ := d.ListSessions(context.Background(), SessionFilter{
		Project: "proj_a", Limit: 10,
	})
	if len(page.Sessions) != 2 {
		t.Errorf("got %d sessions, want 2", len(page.Sessions))
	}
}

func TestMessageCRUD(t *testing.T) {
	d := testDB(t)

	insertSession(t, d, "s1", "p", func(s *Session) {
		s.MessageCount = 4
	})

	m1 := userMsg("s1", 0, "Hello")
	m2 := asstMsg("s1", 1, "Hi there")
	m2.Timestamp = "2024-01-01T00:00:01Z"
	m3 := userMsg("s1", 2, "Thanks")
	m3.Timestamp = "2024-01-01T00:00:02Z"
	m4 := userMsg("s1", 3, "Empty TS")
	m4.Timestamp = ""

	insertMessages(t, d, m1, m2, m3, m4)

	got, err := d.GetAllMessages(context.Background(), "s1")
	if err != nil {
		t.Fatalf("GetAllMessages: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("got %d messages, want 4", len(got))
	}
	if got[0].Content != "Hello" {
		t.Errorf("first message = %q", got[0].Content)
	}
	if got[3].Timestamp != "" {
		t.Errorf("expected empty timestamp, got %q", got[3].Timestamp)
	}

	// Paginated
	got, err = d.GetMessages(context.Background(), "s1", 1, 2, true)
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d messages, want 2", len(got))
	}
	if got[0].Ordinal != 1 {
		t.Errorf("first ordinal = %d, want 1", got[0].Ordinal)
	}

	// Descending
	got, err = d.GetMessages(context.Background(), "s1", 2, 10, false)
	if err != nil {
		t.Fatalf("GetMessages desc: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d, want 3", len(got))
	}
	if got[0].Ordinal != 2 {
		t.Errorf("desc first ordinal = %d, want 2", got[0].Ordinal)
	}
}

func TestMinimap(t *testing.T) {
	d := testDB(t)

	insertSession(t, d, "s1", "p", func(s *Session) {
		s.MessageCount = 2
	})

	m2 := asstMsg("s1", 1, "Hi")
	m2.HasThinking = true
	m2.HasToolUse = true

	insertMessages(t, d,
		userMsg("s1", 0, "Hello"),
		m2,
	)

	entries, err := d.GetMinimap(context.Background(), "s1")
	if err != nil {
		t.Fatalf("GetMinimap: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if !entries[1].HasThinking {
		t.Error("expected HasThinking on second entry")
	}
}

func TestReplaceSessionMessages(t *testing.T) {
	d := testDB(t)

	insertSession(t, d, "s1", "p")

	insertMessages(t, d, userMsg("s1", 0, "old"))

	if err := d.ReplaceSessionMessages("s1", []Message{
		userMsg("s1", 0, "new1"),
		asstMsg("s1", 1, "new2"),
	}); err != nil {
		t.Fatalf("ReplaceSessionMessages: %v", err)
	}

	got, _ := d.GetAllMessages(context.Background(), "s1")
	if len(got) != 2 {
		t.Fatalf("got %d messages, want 2", len(got))
	}
	if got[0].Content != "new1" {
		t.Errorf("content = %q, want %q", got[0].Content, "new1")
	}
}

func TestSearch(t *testing.T) {
	d := testDB(t)
	if !d.HasFTS() {
		t.Skip("skipping search test: no FTS support")
	}

	insertSession(t, d, "s1", "p", func(s *Session) {
		s.MessageCount = 2
	})

	m1 := userMsg("s1", 0, "Fix the authentication bug")
	m2 := asstMsg("s1", 1, "Looking at the auth module")
	m2.Timestamp = "2024-01-01T00:00:01Z"

	insertMessages(t, d, m1, m2)

	page, err := d.Search(context.Background(), SearchFilter{
		Query: "authentication",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(page.Results) != 1 {
		t.Fatalf("got %d results, want 1", len(page.Results))
	}
	if page.Results[0].SessionID != "s1" {
		t.Errorf("session_id = %q", page.Results[0].SessionID)
	}
}

func TestSearchCanceledContext(t *testing.T) {
	d := testDB(t)
	if !d.HasFTS() {
		t.Skip("skipping search test: no FTS support")
	}

	insertSession(t, d, "s1", "p", func(s *Session) {
		s.MessageCount = 1
	})
	insertMessages(t, d, userMsg("s1", 0, "searchable content"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := d.Search(ctx, SearchFilter{
		Query: "searchable",
		Limit: 10,
	})
	if err == nil {
		t.Fatal("expected error from canceled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestListSessionsCanceledContext(t *testing.T) {
	d := testDB(t)

	insertSession(t, d, "s1", "p", func(s *Session) {
		s.MessageCount = 1
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := d.ListSessions(ctx, SessionFilter{Limit: 10})
	if err == nil {
		t.Fatal("expected error from canceled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestGetMessagesCanceledContext(t *testing.T) {
	d := testDB(t)

	insertSession(t, d, "s1", "p", func(s *Session) {
		s.MessageCount = 1
	})
	insertMessages(t, d, userMsg("s1", 0, "msg"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := d.GetMessages(ctx, "s1", 0, 10, true)
	if err == nil {
		t.Fatal("expected error from canceled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestGetStatsCanceledContext(t *testing.T) {
	d := testDB(t)

	// Ensure there's some data so the query actually has work to do
	// (though with immediate cancel it shouldn't matter).
	insertSession(t, d, "s1", "p")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := d.GetStats(ctx)
	if err == nil {
		t.Fatal("expected error from canceled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestStats(t *testing.T) {
	d := testDB(t)

	insertSession(t, d, "s1", "p1")
	insertSession(t, d, "s2", "p2", func(s *Session) {
		s.Machine = "remote"
		s.Agent = "codex"
	})
	insertMessages(t, d,
		userMsg("s1", 0, "hi"),
		userMsg("s2", 0, "bye"),
	)

	stats, err := d.GetStats(context.Background())
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.SessionCount != 2 {
		t.Errorf("session_count = %d, want 2", stats.SessionCount)
	}
	if stats.MessageCount != 2 {
		t.Errorf("message_count = %d, want 2", stats.MessageCount)
	}
	if stats.ProjectCount != 2 {
		t.Errorf("project_count = %d, want 2", stats.ProjectCount)
	}
	if stats.MachineCount != 2 {
		t.Errorf("machine_count = %d, want 2", stats.MachineCount)
	}
}

func TestGetProjects(t *testing.T) {
	d := testDB(t)

	insertSession(t, d, "s1", "alpha")
	insertSession(t, d, "s2", "beta", func(s *Session) {
		s.MessageCount = 2
	})
	insertSession(t, d, "s3", "alpha")

	projects, err := d.GetProjects(context.Background())
	if err != nil {
		t.Fatalf("GetProjects: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("got %d projects, want 2", len(projects))
	}
	if projects[0].Name != "alpha" || projects[0].SessionCount != 2 {
		t.Errorf("alpha: %+v", projects[0])
	}
}

// setupPruneData inserts the standard sessions used by the prune
// candidate filter tests.
func setupPruneData(t *testing.T, d *DB) {
	t.Helper()
	insertSession(t, d, "s1", "spicytakes", func(s *Session) {
		s.FirstMessage = strPtr("You are a code reviewer")
		s.EndedAt = strPtr("2024-01-15T00:00:00Z")
		s.MessageCount = 2
	})
	insertSession(t, d, "s2", "spicytakes", func(s *Session) {
		s.FirstMessage = strPtr("Analyze this blog post")
		s.EndedAt = strPtr("2024-03-01T00:00:00Z")
		s.MessageCount = 2
	})
	insertSession(t, d, "s3", "roborev", func(s *Session) {
		s.FirstMessage = strPtr("You are a code reviewer")
		s.EndedAt = strPtr("2024-03-01T00:00:00Z")
		s.MessageCount = 2
	})
	insertSession(t, d, "s4", "spicytakes", func(s *Session) {
		s.FirstMessage = strPtr("Help me refactor")
		s.EndedAt = strPtr("2024-06-01T00:00:00Z")
		s.MessageCount = 10
	})
}

func TestFindPruneCandidates(t *testing.T) {
	d := testDB(t)
	setupPruneData(t, d)

	tests := []struct {
		name   string
		filter PruneFilter
		want   int
	}{
		{
			name:   "ProjectSubstring",
			filter: PruneFilter{Project: "spicy"},
			want:   3,
		},
		{
			name:   "MaxMessages",
			filter: PruneFilter{MaxMessages: intPtr(2)},
			want:   3,
		},
		{
			name: "BeforeDate",
			filter: PruneFilter{
				Before: "2024-02-01",
			},
			want: 1,
		},
		{
			name: "FirstMessagePrefix",
			filter: PruneFilter{
				FirstMessage: "You are a code reviewer",
			},
			want: 2,
		},
		{
			name: "CombinedProjectAndMaxMessages",
			filter: PruneFilter{
				Project: "spicytakes", MaxMessages: intPtr(2),
			},
			want: 2,
		},
		{
			name: "AllFiltersNoMatch",
			filter: PruneFilter{
				Project:      "spicytakes",
				MaxMessages:  intPtr(2),
				Before:       "2024-02-01",
				FirstMessage: "Analyze",
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := d.FindPruneCandidates(tt.filter)
			if err != nil {
				t.Fatalf("FindPruneCandidates: %v", err)
			}
			if len(got) != tt.want {
				t.Errorf("got %d candidates, want %d",
					len(got), tt.want)
			}
		})
	}

	// The "before" case also checks the specific ID returned.
	t.Run("BeforeDateReturnsCorrectID", func(t *testing.T) {
		got, err := d.FindPruneCandidates(PruneFilter{
			Before: "2024-02-01",
		})
		if err != nil {
			t.Fatalf("FindPruneCandidates: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("got %d, want 1", len(got))
		}
		if got[0].ID != "s1" {
			t.Errorf("got ID %q, want s1", got[0].ID)
		}
	})

	// File metadata returned correctly.
	t.Run("ReturnsFileMetadata", func(t *testing.T) {
		fp := "/path/to/file.jsonl"
		insertSession(t, d, "s5", "test", func(s *Session) {
			s.FilePath = strPtr(fp)
			s.FileSize = int64Ptr(4096)
		})
		got, err := d.FindPruneCandidates(PruneFilter{
			Project: "test",
		})
		if err != nil {
			t.Fatalf("FindPruneCandidates: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("got %d, want 1", len(got))
		}
		if got[0].FilePath == nil || *got[0].FilePath != fp {
			t.Errorf("file_path = %v, want %q", got[0].FilePath, fp)
		}
		if got[0].FileSize == nil || *got[0].FileSize != 4096 {
			t.Errorf("file_size = %v, want 4096", got[0].FileSize)
		}
	})
}

// collectIDs extracts session IDs for error messages.
func collectIDs(sessions []Session) []string {
	ids := make([]string, len(sessions))
	for i, s := range sessions {
		ids[i] = s.ID
	}
	return ids
}

func TestFindPruneCandidatesLikeEscaping(t *testing.T) {
	d := testDB(t)

	insertSession(t, d, "e1", "my%project", func(s *Session) {
		s.FirstMessage = strPtr("100% complete")
	})
	insertSession(t, d, "e2", "my_project", func(s *Session) {
		s.FirstMessage = strPtr("100% complete")
	})
	insertSession(t, d, "e3", "myXproject")
	insertSession(t, d, "e4", `my\project`, func(s *Session) {
		s.FirstMessage = strPtr(`path\to\file`)
	})

	tests := []struct {
		name     string
		filter   PruneFilter
		wantN    int
		wantOnly string
	}{
		{
			name: "LiteralPercent",
			filter: PruneFilter{
				Project: "%",
			},
			wantN: 1, wantOnly: "e1",
		},
		{
			name: "LiteralUnderscore",
			filter: PruneFilter{
				Project: "_",
			},
			wantN: 1, wantOnly: "e2",
		},
		{
			name: "PercentInFirstMessage",
			filter: PruneFilter{
				FirstMessage: "100%",
			},
			wantN: 2,
		},
		{
			name: "BackslashInProject",
			filter: PruneFilter{
				Project: `\`,
			},
			wantN: 1, wantOnly: "e4",
		},
		{
			name: "BackslashInFirstMessage",
			filter: PruneFilter{
				FirstMessage: `path\to`,
			},
			wantN: 1, wantOnly: "e4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := d.FindPruneCandidates(tt.filter)
			if err != nil {
				t.Fatalf("FindPruneCandidates: %v", err)
			}
			if len(got) != tt.wantN {
				t.Fatalf("got %v, want %d results",
					collectIDs(got), tt.wantN)
			}
			if tt.wantOnly != "" && got[0].ID != tt.wantOnly {
				t.Errorf("got %v, want [%s]",
					collectIDs(got), tt.wantOnly)
			}
		})
	}
}

func TestFindPruneCandidatesMaxMessagesSentinel(t *testing.T) {
	d := testDB(t)

	insertSession(t, d, "m1", "p", func(s *Session) {
		s.MessageCount = 0
	})
	insertSession(t, d, "m2", "p")
	insertSession(t, d, "m3", "p", func(s *Session) {
		s.MessageCount = 5
	})

	tests := []struct {
		name   string
		filter PruneFilter
		want   int
	}{
		{
			name:   "ZeroMatchesOnlyZero",
			filter: PruneFilter{MaxMessages: intPtr(0)},
			want:   1,
		},
		{
			name: "NilDisablesFilter",
			filter: PruneFilter{
				Project: "p",
			},
			want: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := d.FindPruneCandidates(tt.filter)
			if err != nil {
				t.Fatalf("FindPruneCandidates: %v", err)
			}
			if len(got) != tt.want {
				t.Errorf("got %d, want %d", len(got), tt.want)
			}
		})
	}

	// Additional check: MaxMessages=0 returns m1 specifically.
	got, err := d.FindPruneCandidates(PruneFilter{MaxMessages: intPtr(0)})
	if err != nil {
		t.Fatalf("FindPruneCandidates MaxMessages=0: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("MaxMessages 0: got %d results, want 1", len(got))
	}
	if got[0].ID != "m1" {
		t.Errorf("MaxMessages 0: got %q, want m1", got[0].ID)
	}
}

func TestDeleteSessions(t *testing.T) {
	d := testDB(t)

	for _, id := range []string{"s1", "s2", "s3"} {
		insertSession(t, d, id, "p")
		insertMessages(t, d, userMsg(id, 0, "msg for "+id))
	}

	stats, _ := d.GetStats(context.Background())
	if stats.SessionCount != 3 {
		t.Fatalf("initial sessions = %d, want 3", stats.SessionCount)
	}
	if stats.MessageCount != 3 {
		t.Fatalf("initial messages = %d, want 3", stats.MessageCount)
	}

	deleted, err := d.DeleteSessions([]string{"s1", "s3"})
	if err != nil {
		t.Fatalf("DeleteSessions: %v", err)
	}
	if deleted != 2 {
		t.Errorf("deleted = %d, want 2", deleted)
	}

	requireSessionGone(t, d, "s1")
	requireSessionExists(t, d, "s2")
	requireSessionGone(t, d, "s3")

	msgs, _ := d.GetAllMessages(context.Background(), "s1")
	if len(msgs) != 0 {
		t.Errorf("s1 messages = %d, want 0", len(msgs))
	}
	msgs, _ = d.GetAllMessages(context.Background(), "s2")
	if len(msgs) != 1 {
		t.Errorf("s2 messages = %d, want 1", len(msgs))
	}

	stats, _ = d.GetStats(context.Background())
	if stats.SessionCount != 1 {
		t.Errorf("session_count = %d, want 1", stats.SessionCount)
	}
	if stats.MessageCount != 1 {
		t.Errorf("message_count = %d, want 1", stats.MessageCount)
	}

	deleted, err = d.DeleteSessions(nil)
	if err != nil {
		t.Fatalf("DeleteSessions empty: %v", err)
	}
	if deleted != 0 {
		t.Errorf("deleted empty = %d, want 0", deleted)
	}
}

func TestSessionFileInfo(t *testing.T) {
	d := testDB(t)

	insertSession(t, d, "s1", "p", func(s *Session) {
		s.FileSize = int64Ptr(1024)
		s.FileMtime = int64Ptr(1700000000)
		s.FileHash = strPtr("abc123def456")
	})

	gotSize, gotHash, ok := d.GetSessionFileInfo("s1")
	if !ok {
		t.Fatal("expected ok")
	}
	if gotSize != 1024 {
		t.Errorf("got size=%d, want 1024", gotSize)
	}
	if gotHash != "abc123def456" {
		t.Errorf("got hash=%q, want %q", gotHash, "abc123def456")
	}

	_, _, ok = d.GetSessionFileInfo("nonexistent")
	if ok {
		t.Error("expected !ok for nonexistent")
	}
}

func TestGetSessionFull(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	t.Run("AllMetadata", func(t *testing.T) {
		insertSession(t, d, "full-1", "proj", func(s *Session) {
			s.FirstMessage = strPtr("hello")
			s.StartedAt = strPtr("2024-01-01T00:00:00Z")
			s.EndedAt = strPtr("2024-01-01T01:00:00Z")
			s.MessageCount = 5
			s.FilePath = strPtr("/tmp/session.jsonl")
			s.FileSize = int64Ptr(2048)
			s.FileMtime = int64Ptr(1700000000)
			s.FileHash = strPtr("abc123")
		})

		got, err := d.GetSessionFull(ctx, "full-1")
		if err != nil {
			t.Fatalf("GetSessionFull: %v", err)
		}
		if got == nil {
			t.Fatal("expected non-nil session")
		}
		if got.ID != "full-1" {
			t.Errorf("ID = %q, want %q", got.ID, "full-1")
		}
		if got.Project != "proj" {
			t.Errorf("Project = %q, want %q", got.Project, "proj")
		}
		if got.MessageCount != 5 {
			t.Errorf("MessageCount = %d, want 5", got.MessageCount)
		}
		if got.FilePath == nil || *got.FilePath != "/tmp/session.jsonl" {
			t.Errorf("FilePath = %v, want %q", got.FilePath, "/tmp/session.jsonl")
		}
		if got.FileSize == nil || *got.FileSize != 2048 {
			t.Errorf("FileSize = %v, want 2048", got.FileSize)
		}
		if got.FileMtime == nil || *got.FileMtime != 1700000000 {
			t.Errorf("FileMtime = %v, want 1700000000", got.FileMtime)
		}
		if got.FileHash == nil || *got.FileHash != "abc123" {
			t.Errorf("FileHash = %v, want %q", got.FileHash, "abc123")
		}
		if got.FirstMessage == nil || *got.FirstMessage != "hello" {
			t.Errorf("FirstMessage = %v, want %q", got.FirstMessage, "hello")
		}
		if got.StartedAt == nil || *got.StartedAt != "2024-01-01T00:00:00Z" {
			t.Errorf("StartedAt = %v, want %q", got.StartedAt, "2024-01-01T00:00:00Z")
		}
		if got.EndedAt == nil || *got.EndedAt != "2024-01-01T01:00:00Z" {
			t.Errorf("EndedAt = %v, want %q", got.EndedAt, "2024-01-01T01:00:00Z")
		}
	})

	t.Run("NullMetadata", func(t *testing.T) {
		insertSession(t, d, "full-2", "proj", func(s *Session) {
			s.MessageCount = 1
		})

		got, err := d.GetSessionFull(ctx, "full-2")
		if err != nil {
			t.Fatalf("GetSessionFull: %v", err)
		}
		if got == nil {
			t.Fatal("expected non-nil session")
		}
		if got.FilePath != nil {
			t.Errorf("FilePath = %v, want nil", got.FilePath)
		}
		if got.FileSize != nil {
			t.Errorf("FileSize = %v, want nil", got.FileSize)
		}
		if got.FileMtime != nil {
			t.Errorf("FileMtime = %v, want nil", got.FileMtime)
		}
		if got.FileHash != nil {
			t.Errorf("FileHash = %v, want nil", got.FileHash)
		}
		if got.FirstMessage != nil {
			t.Errorf("FirstMessage = %v, want nil", got.FirstMessage)
		}
		if got.StartedAt != nil {
			t.Errorf("StartedAt = %v, want nil", got.StartedAt)
		}
		if got.EndedAt != nil {
			t.Errorf("EndedAt = %v, want nil", got.EndedAt)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		got, err := d.GetSessionFull(ctx, "nonexistent")
		if err != nil {
			t.Fatalf("GetSessionFull: %v", err)
		}
		if got != nil {
			t.Errorf("expected nil session, got %+v", got)
		}
	})
}

func TestCursorEncodeDecode(t *testing.T) {
	d := testDB(t)
	encoded := d.EncodeCursor("2024-01-01T00:00:00Z", "session-1")
	cur, err := d.DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("DecodeCursor: %v", err)
	}
	if cur.EndedAt != "2024-01-01T00:00:00Z" {
		t.Errorf("EndedAt = %q", cur.EndedAt)
	}
	if cur.ID != "session-1" {
		t.Errorf("ID = %q", cur.ID)
	}

	encodedWithTotal := d.EncodeCursor(
		"2024-01-01T00:00:00Z",
		"session-1",
		123,
	)
	cur, err = d.DecodeCursor(encodedWithTotal)
	if err != nil {
		t.Fatalf("DecodeCursor with total: %v", err)
	}
	if cur.Total != 123 {
		t.Errorf("Total = %d, want 123", cur.Total)
	}
}

func TestCursorTampering(t *testing.T) {
	d := testDB(t)
	// 1. Create a valid signed cursor
	original := d.EncodeCursor("2024-01-01T00:00:00Z", "s1", 100)

	parts := strings.Split(original, ".")
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts (payload.sig), got %d", len(parts))
	}

	payload := parts[0]
	sig := parts[1]

	// 2. Decode payload, modify Total, re-encode
	data, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("DecodeString payload: %v", err)
	}
	var c SessionCursor
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatalf("Unmarshal payload: %v", err)
	}
	c.Total = 999
	tamperedData, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal tampered: %v", err)
	}
	tamperedPayload := base64.RawURLEncoding.EncodeToString(tamperedData)

	// 3. Construct tampered cursor with original signature
	tamperedCursor := tamperedPayload + "." + sig

	// 4. Decode should fail signature check
	_, err = d.DecodeCursor(tamperedCursor)
	if err == nil {
		t.Fatal("expected error for tampered cursor, got nil")
	}
	if !strings.Contains(err.Error(), "signature mismatch") {
		t.Errorf("expected signature mismatch error, got: %v", err)
	}
}

func TestLegacyCursor(t *testing.T) {
	d := testDB(t)
	// Create a legacy cursor (base64 json only, no signature)
	c := SessionCursor{
		EndedAt: "2024-01-01T00:00:00Z",
		ID:      "s1",
		Total:   100, // Should be ignored
	}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal legacy: %v", err)
	}
	legacy := base64.RawURLEncoding.EncodeToString(data)

	// Decode
	got, err := d.DecodeCursor(legacy)
	if err != nil {
		t.Fatalf("DecodeCursor legacy: %v", err)
	}

	// Verify ID/EndedAt are preserved
	if got.ID != "s1" {
		t.Errorf("ID = %q, want s1", got.ID)
	}
	// Verify Total is ZEROED out
	if got.Total != 0 {
		t.Errorf("Total = %d, want 0 (untrusted legacy)", got.Total)
	}
}

func TestCursorSecretConcurrency(t *testing.T) {
	d := testDB(t)

	const goroutines = 8
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			for j := range iterations {
				switch id % 3 {
				case 0:
					secret := []byte(
						fmt.Sprintf("secret-%d-%d", id, j),
					)
					d.SetCursorSecret(secret)
				case 1:
					d.EncodeCursor(
						"2024-01-01T00:00:00Z",
						fmt.Sprintf("s-%d-%d", id, j),
						42,
					)
				case 2:
					encoded := d.EncodeCursor(
						"2024-01-01T00:00:00Z", "s1",
					)
					// Decode may fail if secret rotated
					// between encode and decode; that's OK.
					_, err := d.DecodeCursor(encoded)
					if err != nil &&
						!errors.Is(err, ErrInvalidCursor) {
						t.Errorf(
							"unexpected DecodeCursor error: %v",
							err,
						)
					}
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestSetCursorSecretDefensiveCopy(t *testing.T) {
	d := testDB(t)

	secret := []byte("my-secret-key-for-testing-copy!!")
	d.SetCursorSecret(secret)

	encoded := d.EncodeCursor("2024-01-01T00:00:00Z", "s1")

	// Mutate the original slice — should not affect the DB.
	for i := range secret {
		secret[i] = 0
	}

	_, err := d.DecodeCursor(encoded)
	if err != nil {
		t.Fatalf(
			"DecodeCursor failed after caller mutated secret: %v",
			err,
		)
	}
}

func TestSampleMinimap(t *testing.T) {
	// Create a helper to generate n entries
	makeEntries := func(n int) []MinimapEntry {
		entries := make([]MinimapEntry, 0, n)
		for i := range n {
			entries = append(entries, MinimapEntry{
				Ordinal: i,
				Role:    "user",
			})
		}
		return entries
	}

	tests := []struct {
		name    string
		entries []MinimapEntry
		max     int
		wantLen int
		// simple check function for ordinals
		check func([]MinimapEntry) error
	}{
		{
			name:    "SampleDown",
			entries: makeEntries(10),
			max:     3,
			wantLen: 3,
			check: func(got []MinimapEntry) error {
				if got[0].Ordinal != 0 || got[1].Ordinal != 4 || got[2].Ordinal != 9 {
					return fmt.Errorf("ordinals = [%d %d %d], want [0 4 9]",
						got[0].Ordinal, got[1].Ordinal, got[2].Ordinal)
				}
				return nil
			},
		},
		{
			name:    "ExactSize",
			entries: makeEntries(5),
			max:     5,
			wantLen: 5,
		},
		{
			name:    "SmallerThanMax",
			entries: makeEntries(3),
			max:     5,
			wantLen: 3,
		},
		{
			name:    "Empty",
			entries: makeEntries(0),
			max:     5,
			wantLen: 0,
		},
		{
			name:    "MaxOne",
			entries: makeEntries(10),
			max:     1,
			wantLen: 1,
			check: func(got []MinimapEntry) error {
				if got[0].Ordinal != 0 {
					return fmt.Errorf("ordinal = %d, want 0", got[0].Ordinal)
				}
				return nil
			},
		},
		{
			name:    "MaxZero",
			entries: makeEntries(10),
			max:     0,
			wantLen: 10, // Returns original if max <= 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SampleMinimap(tt.entries, tt.max)
			if len(got) != tt.wantLen {
				t.Errorf("len = %d, want %d", len(got), tt.wantLen)
				return
			}
			if tt.check != nil {
				if err := tt.check(got); err != nil {
					t.Error(err)
				}
			}
		})
	}
}

func TestDeleteSession(t *testing.T) {
	d := testDB(t)

	insertSession(t, d, "s1", "p")
	insertMessages(t, d, userMsg("s1", 0, "test"))

	if err := d.DeleteSession("s1"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	requireSessionGone(t, d, "s1")

	msgs, _ := d.GetAllMessages(context.Background(), "s1")
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages after cascade, got %d",
			len(msgs))
	}
}

func TestMigrationRace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "race.db")

	// 1. Create legacy schema (no file_hash)
	// We use a raw connection to avoid running our migrations.
	rawDB, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	defer rawDB.Close()

	// Create sessions table without file_hash
	_, err = rawDB.Exec(`
		CREATE TABLE sessions (
			id          TEXT PRIMARY KEY,
			project     TEXT NOT NULL,
			machine     TEXT NOT NULL DEFAULT 'local',
			agent       TEXT NOT NULL DEFAULT 'claude',
			first_message TEXT,
			started_at  TEXT,
			ended_at    TEXT,
			message_count INTEGER NOT NULL DEFAULT 0,
			file_path   TEXT,
			file_size   INTEGER,
			file_mtime  INTEGER,
			created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
		);
	`)
	if err != nil {
		t.Fatalf("setup schema: %v", err)
	}

	// 2. Run concurrent Open
	// Both should succeed. One will likely hit "duplicate column".
	errCh := make(chan error, 2)
	var (
		mu         sync.Mutex
		cond       = sync.NewCond(&mu)
		readyCount = 0
		start      = false
	)

	for range 2 {
		go func() {
			mu.Lock()
			readyCount++
			if readyCount == 2 {
				cond.Broadcast()
			}
			for !start {
				cond.Wait()
			}
			mu.Unlock()

			db, err := Open(path)
			if err != nil {
				errCh <- err
				return
			}
			db.Close()
			errCh <- nil
		}()
	}

	mu.Lock()
	for readyCount < 2 {
		cond.Wait()
	}
	start = true
	cond.Broadcast()
	mu.Unlock()

	for range 2 {
		if err := <-errCh; err != nil {
			t.Errorf("concurrent Open failed: %v", err)
		}
	}

	// 3. Verify schema has file_hash
	db, err := Open(path)
	if err != nil {
		t.Fatalf("re-open: %v", err)
	}
	defer db.Close()

	// Check if column exists by selecting it
	// We use the internal writer connection since we're in the same package
	_, err = db.writer.Exec("SELECT file_hash FROM sessions LIMIT 1")
	if err != nil {
		t.Errorf("file_hash column missing or inaccessible: %v", err)
	}
}

func TestFTSBackfill(t *testing.T) {
	// Check for FTS support first
	dCheck := testDB(t)
	if !dCheck.HasFTS() {
		dCheck.Close()
		t.Skip("skipping FTS backfill test: no FTS support")
	}
	dCheck.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "backfill.db")

	// 1. Create DB and drop FTS to simulate "old" DB or broken state
	d1, err := Open(path)
	if err != nil {
		t.Fatalf("Open 1: %v", err)
	}
	// Use writer directly to ensure it happens
	if _, err := d1.writer.Exec("DROP TABLE IF EXISTS messages_fts"); err != nil {
		t.Fatalf("dropping fts: %v", err)
	}
	// Also drop triggers, otherwise inserts will fail
	for _, tr := range []string{"messages_ai", "messages_ad", "messages_au"} {
		if _, err := d1.writer.Exec("DROP TRIGGER IF EXISTS " + tr); err != nil {
			t.Fatalf("dropping trigger %s: %v", tr, err)
		}
	}

	// 2. Insert messages while FTS is missing
	insertSession(t, d1, "s1", "proj")
	insertMessages(t, d1, userMsg("s1", 0, "unique_keyword"))

	if err := d1.Close(); err != nil {
		t.Fatalf("Close 1: %v", err)
	}

	// 3. Re-open. This should detect missing FTS, create it, and backfill.
	d2, err := Open(path)
	if err != nil {
		t.Fatalf("Open 2: %v", err)
	}
	defer d2.Close()

	if !d2.HasFTS() {
		t.Fatal("FTS should be available after re-open")
	}

	// 4. Verify search finds the message
	page, err := d2.Search(context.Background(), SearchFilter{
		Query: "unique_keyword",
		Limit: 1,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(page.Results) != 1 {
		t.Fatalf("got %d results, want 1", len(page.Results))
	}
	if page.Results[0].SessionID != "s1" {
		t.Errorf("result session_id = %q, want s1", page.Results[0].SessionID)
	}
}
