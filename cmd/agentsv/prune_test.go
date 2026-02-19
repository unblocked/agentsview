package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wesm/agentsview/internal/db"
)

func TestParsePruneFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
		check   func(t *testing.T, cfg PruneConfig)
	}{
		{
			name:    "no filters",
			args:    []string{},
			wantErr: "at least one filter",
		},
		{
			name: "project filter",
			args: []string{"--project", "myapp"},
			check: func(t *testing.T, cfg PruneConfig) {
				t.Helper()
				if cfg.Filter.Project != "myapp" {
					t.Errorf(
						"Project = %q, want %q",
						cfg.Filter.Project, "myapp",
					)
				}
				if cfg.DryRun || cfg.Yes {
					t.Error("unexpected flag defaults")
				}
			},
		},
		{
			name: "all flags",
			args: []string{
				"--project", "p",
				"--max-messages", "5",
				"--before", "2024-01-01",
				"--first-message", "hello",
				"--dry-run",
				"--yes",
			},
			check: func(t *testing.T, cfg PruneConfig) {
				t.Helper()
				if cfg.Filter.Project != "p" {
					t.Errorf("Project = %q", cfg.Filter.Project)
				}
				if cfg.Filter.MaxMessages == nil || *cfg.Filter.MaxMessages != 5 {
					t.Errorf(
						"MaxMessages = %v", cfg.Filter.MaxMessages,
					)
				}
				if cfg.Filter.Before != "2024-01-01" {
					t.Errorf("Before = %q", cfg.Filter.Before)
				}
				if cfg.Filter.FirstMessage != "hello" {
					t.Errorf(
						"FirstMessage = %q",
						cfg.Filter.FirstMessage,
					)
				}
				if !cfg.DryRun {
					t.Error("DryRun should be true")
				}
				if !cfg.Yes {
					t.Error("Yes should be true")
				}
			},
		},
		{
			name:    "unknown flag",
			args:    []string{"--bogus"},
			wantErr: "flag provided but not defined",
		},
		{
			name:    "negative max-messages",
			args:    []string{"--max-messages", "-2"},
			wantErr: "max-messages must be >= 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := parsePruneFlags(tt.args)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q",
						tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q missing %q",
						err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

func TestParsePruneFlagsHelp(t *testing.T) {
	_, err := parsePruneFlags([]string{"--help"})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf(
			"expected flag.ErrHelp, got %v", err,
		)
	}
}

func TestPrunerEmptyFilterReturnsError(t *testing.T) {
	d := testDB(t)

	pruner, _ := newTestPruner(t, d, "")
	cfg := PruneConfig{
		Filter: db.PruneFilter{},
	}

	err := pruner.Prune(cfg)
	if err == nil {
		t.Fatal("expected error for empty filter")
	}
	if !strings.Contains(err.Error(), "at least one filter") {
		t.Errorf(
			"error %q should mention filter requirement",
			err,
		)
	}
}

func TestConfirm(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"yes lowercase", "y\n", true},
		{"yes full", "yes\n", true},
		{"YES uppercase", "YES\n", true},
		{"no", "n\n", false},
		{"empty", "\n", false},
		{"other text", "maybe\n", false},
		{"y with spaces", "  y  \n", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := strings.NewReader(tt.input)
			out := &bytes.Buffer{}
			got := confirm(in, out, "Delete?")
			if got != tt.want {
				t.Errorf("confirm() = %v, want %v", got, tt.want)
			}
			if !strings.Contains(out.String(), "[y/N]") {
				t.Error("prompt missing [y/N]")
			}
		})
	}
}

func TestWriteSummary(t *testing.T) {
	size1 := int64(1024)
	size2 := int64(2048)
	sessions := []db.Session{
		{ID: "s1", Project: "projA", FileSize: &size1},
		{ID: "s2", Project: "projA", FileSize: &size2},
		{ID: "s3", Project: "projB"},
	}

	var buf bytes.Buffer
	writeSummary(&buf, sessions)
	out := buf.String()

	if !strings.Contains(out, "Found 3 sessions") {
		t.Errorf("missing session count: %s", out)
	}
	if !strings.Contains(out, "3.0 KB") {
		t.Errorf("missing total size: %s", out)
	}
	if !strings.Contains(out, "projA") {
		t.Errorf("missing projA: %s", out)
	}
	if !strings.Contains(out, "projB") {
		t.Errorf("missing projB: %s", out)
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatBytes(tt.input)
			if got != tt.want {
				t.Errorf(
					"formatBytes(%d) = %q, want %q",
					tt.input, got, tt.want,
				)
			}
		})
	}
}

func testDB(t *testing.T) *db.DB {
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

func TestPrunerDryRun(t *testing.T) {
	d := testDB(t)

	seedSession(t, d, "s1", "test")

	pruner, buf := newTestPruner(t, d, "")
	cfg := PruneConfig{
		Filter: db.PruneFilter{Project: "test"},
		DryRun: true,
	}

	if err := pruner.Prune(cfg); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Dry run") {
		t.Errorf("expected dry run message: %s", out)
	}
	if !strings.Contains(out, "Found 1 sessions") {
		t.Errorf("expected summary: %s", out)
	}
}

func TestPrunerNoMatches(t *testing.T) {
	d := testDB(t)

	pruner, buf := newTestPruner(t, d, "")
	cfg := PruneConfig{
		Filter: db.PruneFilter{Project: "nonexistent"},
	}

	if err := pruner.Prune(cfg); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	if !strings.Contains(buf.String(), "No sessions match") {
		t.Errorf("expected no-match message: %s", buf.String())
	}
}

func TestPrunerAbort(t *testing.T) {
	d := testDB(t)

	seedSession(t, d, "s1", "test")

	pruner, buf := newTestPruner(t, d, "n\n")
	cfg := PruneConfig{
		Filter: db.PruneFilter{Project: "test"},
	}

	if err := pruner.Prune(cfg); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	if !strings.Contains(buf.String(), "Aborted") {
		t.Errorf("expected abort message: %s", buf.String())
	}

	// Session should still exist.
	s, err := d.GetSession(context.Background(),"s1")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if s == nil {
		t.Error("session was deleted despite abort")
	}
}

func TestPrunerConfirmDelete(t *testing.T) {
	d := testDB(t)

	seedSession(t, d, "s1", "test")

	pruner, buf := newTestPruner(t, d, "y\n")
	cfg := PruneConfig{
		Filter: db.PruneFilter{Project: "test"},
	}

	if err := pruner.Prune(cfg); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	if !strings.Contains(buf.String(), "Deleted 1 sessions") {
		t.Errorf("expected deletion message: %s", buf.String())
	}

	s, err := d.GetSession(context.Background(),"s1")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if s != nil {
		t.Error("session still exists after confirmed delete")
	}
}

func TestPrunerYesFlag(t *testing.T) {
	d := testDB(t)

	seedSession(t, d, "s1", "test")

	pruner, buf := newTestPruner(t, d, "")
	cfg := PruneConfig{
		Filter: db.PruneFilter{Project: "test"},
		Yes:    true,
	}

	if err := pruner.Prune(cfg); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "[y/N]") {
		t.Error("should not prompt when --yes is set")
	}
	if !strings.Contains(out, "Deleted 1 sessions") {
		t.Errorf("expected deletion message: %s", out)
	}
}

func TestDeleteFilesRemovesFiles(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "session1")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}

	f := filepath.Join(subdir, "data.jsonl")
	if err := os.WriteFile(f, []byte("test data"), 0o644); err != nil {
		t.Fatal(err)
	}

	sessions := []db.Session{
		{ID: "s1", FilePath: &f},
	}

	removed, reclaimed := deleteFiles(sessions)
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}
	if reclaimed != 9 {
		t.Errorf("reclaimed = %d, want 9", reclaimed)
	}

	// File should be gone.
	if _, err := os.Stat(f); !os.IsNotExist(err) {
		t.Error("file still exists")
	}

	// Empty parent dir should be removed.
	if _, err := os.Stat(subdir); !os.IsNotExist(err) {
		t.Error("empty parent dir still exists")
	}
}

func TestDeleteFilesMissingFile(t *testing.T) {
	path := "/nonexistent/path/file.jsonl"
	sessions := []db.Session{
		{ID: "s1", FilePath: &path},
	}

	removed, reclaimed := deleteFiles(sessions)
	if removed != 0 {
		t.Errorf("removed = %d, want 0", removed)
	}
	if reclaimed != 0 {
		t.Errorf("reclaimed = %d, want 0", reclaimed)
	}
}

func TestDeleteFilesNilPath(t *testing.T) {
	sessions := []db.Session{
		{ID: "s1", FilePath: nil},
	}

	removed, reclaimed := deleteFiles(sessions)
	if removed != 0 {
		t.Errorf("removed = %d, want 0", removed)
	}
	if reclaimed != 0 {
		t.Errorf("reclaimed = %d, want 0", reclaimed)
	}
}

func TestPruneHelpExitCode(t *testing.T) {
	if os.Getenv("GO_TEST_PRUNE_HELPER_PROCESS") == "1" {
		// Attempt to run prune --help
		// This should call os.Exit(0)
		runPrune([]string{"--help"})
		// If we are here, os.Exit wasn't called or failed
		t.Fatal("runPrune did not exit")
		return
	}

	// Run the test in a subprocess
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}

	// We pass -test.run to only run THIS function in subprocess
	cmd := exec.Command(exe, "-test.run=^TestPruneHelpExitCode$")
	// Set the helper env var
	cmd.Env = append(os.Environ(), "GO_TEST_PRUNE_HELPER_PROCESS=1")
	
	// We might need to capture output to ensure it prints help
	var out bytes.Buffer
	cmd.Stderr = &out
	cmd.Stdout = &out

	err = cmd.Run()
	
	// Check exit code
	if err != nil {
		// If exit code is not 0, err will be of type *ExitError
		// If TestPruneHelpExitCode subprocess exits 0, err is nil.
		t.Fatalf("subprocess failed with %v\nOutput: %s", err, out.String())
	}
}

func newTestPruner(t *testing.T, d *db.DB, input string) (*Pruner, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	p := &Pruner{
		DB:  d,
		Out: &buf,
		In:  strings.NewReader(input),
	}
	return p, &buf
}

func seedSession(t *testing.T, d *db.DB, id, project string) {
	t.Helper()
	ended := "2024-01-01T00:00:00Z"
	err := d.UpsertSession(db.Session{
		ID:           id,
		Project:      project,
		Machine:      "local",
		Agent:        "claude",
		MessageCount: 0,
		EndedAt:      &ended,
	})
	if err != nil {
		t.Fatalf("seeding session: %v", err)
	}
}
