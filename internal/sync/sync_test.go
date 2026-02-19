package sync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wesm/agentsview/internal/parser"
)

// createTestFile creates a file at path with minimal content,
// creating parent directories as needed.
func createTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// setupTestDir creates a temporary directory and populates it with the given relative file paths.
func setupTestDir(t *testing.T, relativePaths []string) string {
	t.Helper()
	dir := t.TempDir()
	for _, p := range relativePaths {
		createTestFile(t, filepath.Join(dir, p))
	}
	return dir
}

// assertDiscoveredFiles verifies that the discovered files match the expected filenames and agent type.
func assertDiscoveredFiles(t *testing.T, got []DiscoveredFile, wantFilenames []string, wantAgent parser.AgentType) {
	t.Helper()

	want := make(map[string]bool)
	for _, f := range wantFilenames {
		want[f] = true
	}

	gotMap := make(map[string]bool)
	for _, f := range got {
		base := filepath.Base(f.Path)
		gotMap[base] = true
		if f.Agent != wantAgent {
			t.Errorf("file %q: agent = %q, want %q", base, f.Agent, wantAgent)
		}
	}

	if len(got) != len(want) {
		t.Errorf("got %d files total, want %d", len(got), len(want))
	}

	for file := range want {
		if !gotMap[file] {
			t.Errorf("missing expected file: %q", file)
		}
	}
	
	// Check for unexpected files
	for file := range gotMap {
		if !want[file] {
			t.Errorf("got unexpected file: %q", file)
		}
	}
}

func TestDiscoverClaudeProjects(t *testing.T) {
	dir := setupTestDir(t, []string{
		filepath.Join("project-a", "abc.jsonl"),
		filepath.Join("project-a", "def.jsonl"),
		filepath.Join("project-a", "agent-123.jsonl"), // Should be ignored
		filepath.Join("project-b", "xyz.jsonl"),
	})

	files := DiscoverClaudeProjects(dir)

	assertDiscoveredFiles(t, files, []string{
		"abc.jsonl",
		"def.jsonl",
		"xyz.jsonl",
	}, parser.AgentClaude)
}

func TestDiscoverClaudeProjectsEmpty(t *testing.T) {
	dir := t.TempDir()
	files := DiscoverClaudeProjects(dir)
	assertDiscoveredFiles(t, files, nil, parser.AgentClaude)
}

func TestDiscoverClaudeProjectsNonexistent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "does-not-exist")
	files := DiscoverClaudeProjects(dir)
	if files != nil {
		t.Errorf("expected nil, got %d files", len(files))
	}
}

func TestDiscoverCodexSessions(t *testing.T) {
	file1 := "rollout-123-abc-def-ghi-jkl-mno.jsonl"
	file2 := "rollout-456-abc-def-ghi-jkl-mno.jsonl"
	
	dir := setupTestDir(t, []string{
		filepath.Join("2024", "01", "15", file1),
		filepath.Join("2024", "02", "01", file2),
	})

	files := DiscoverCodexSessions(dir)

	assertDiscoveredFiles(t, files, []string{
		file1,
		file2,
	}, parser.AgentCodex)
}

func TestDiscoverCodexSessionsSkipsNonDigit(t *testing.T) {
	// Non-digit directory should be ignored
	dir := setupTestDir(t, []string{
		filepath.Join("notes", "01", "01", "x.jsonl"),
	})

	files := DiscoverCodexSessions(dir)
	assertDiscoveredFiles(t, files, nil, parser.AgentCodex)
}

func TestFindClaudeSourceFile(t *testing.T) {
	relPath := filepath.Join("project-a", "session-abc.jsonl")
	dir := setupTestDir(t, []string{relPath})

	expected := filepath.Join(dir, relPath)

	got := FindClaudeSourceFile(dir, "session-abc")
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}

	// Nonexistent
	got = FindClaudeSourceFile(dir, "nonexistent")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestFindClaudeSourceFileValidation(t *testing.T) {
	dir := t.TempDir()

	// Invalid session IDs should return empty
	tests := []string{"", "../etc/passwd", "a/b", "a b"}
	for _, id := range tests {
		got := FindClaudeSourceFile(dir, id)
		if got != "" {
			t.Errorf("FindClaudeSourceFile(%q) = %q, want empty",
				id, got)
		}
	}
}

func TestFindCodexSourceFile(t *testing.T) {
	uuid := "abc12345-1234-5678-9abc-def012345678"
	filename := "rollout-20240115-" + uuid + ".jsonl"
	relPath := filepath.Join("2024", "01", "15", filename)
	
	dir := setupTestDir(t, []string{relPath})
	expected := filepath.Join(dir, relPath)

	got := FindCodexSourceFile(dir, uuid)
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestExtractUUIDFromRollout(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{
			"rollout-20240115-abc12345-1234-5678-9abc-def012345678.jsonl",
			"abc12345-1234-5678-9abc-def012345678",
		},
		{
			"rollout-20240115T100000-abc12345-1234-5678-9abc-def012345678.jsonl",
			"abc12345-1234-5678-9abc-def012345678",
		},
		{
			"short.jsonl",
			"",
		},
		{
			"rollout-20240115-12345678-1234-1234-1234-1234567890ab-abc12345-1234-5678-9abc-def012345678.jsonl",
			"abc12345-1234-5678-9abc-def012345678",
		},
		{
			"rollout-20240115-abc12345-1234-5678-9abc-def012345678-suffix.jsonl",
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := extractUUIDFromRollout(tt.filename)
			if got != tt.want {
				t.Errorf("extractUUID(%q) = %q, want %q",
					tt.filename, got, tt.want)
			}
		})
	}
}

func TestIsValidSessionID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"abc-123", true},
		{"session_1", true},
		{"abc123", true},
		{"", false},
		{"../etc", false},
		{"a b", false},
		{"a/b", false},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := isValidSessionID(tt.id)
			if got != tt.want {
				t.Errorf("isValidSessionID(%q) = %v, want %v",
					tt.id, got, tt.want)
			}
		})
	}
}

func TestIsDigits(t *testing.T) {
	tests := []struct {
		s    string
		want bool
	}{
		{"123", true},
		{"0", true},
		{"", false},
		{"12a", false},
		{"abc", false},
		{"１２３", true}, // Fullwidth digits are supported
	}

	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			got := isDigits(tt.s)
			if got != tt.want {
				t.Errorf("isDigits(%q) = %v, want %v",
					tt.s, got, tt.want)
			}
		})
	}
}
