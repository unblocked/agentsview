package server_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestUploadSession_SaveFailure(t *testing.T) {
	te := setup(t)

	// Create a file where the project directory should be
	// to force os.MkdirAll to fail
	projectPath := filepath.Join(te.dataDir, "uploads", "failproj")
	if err := os.MkdirAll(filepath.Dir(projectPath), 0o755); err != nil {
		t.Fatalf("creating uploads dir: %v", err)
	}
	f, err := os.Create(projectPath)
	if err != nil {
		t.Fatalf("creating conflict file: %v", err)
	}
	f.Close()

	w := te.upload(t, "test.jsonl", "{}", "project=failproj")
	assertStatus(t, w, http.StatusInternalServerError)

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decoding error response: %v", err)
	}
	if got, want := resp["error"], "failed to save upload"; got != want {
		t.Errorf("expected error %q, got %q", want, got)
	}
}

func TestUploadSession_DBFailure(t *testing.T) {
	te := setup(t)

	// Close DB to force saveSessionToDB to fail
	te.db.Close()

	content := `{"type":"user","timestamp":"2024-01-01T10:00:00Z","message":{"content":"Hello"}}`
	w := te.upload(t, "test.jsonl", content, "project=myproj")
	assertStatus(t, w, http.StatusInternalServerError)

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decoding error response: %v", err)
	}
	if got, want := resp["error"], "failed to save session to database"; got != want {
		t.Errorf("expected error %q, got %q", want, got)
	}
}
