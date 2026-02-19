package server_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wesm/agentsview/internal/config"
	"github.com/wesm/agentsview/internal/db"
	"github.com/wesm/agentsview/internal/server"
	"github.com/wesm/agentsview/internal/sync"
)

// TestServerTimeouts starts a real HTTP server and verifies that
// streaming connections (SSE) are not closed prematurely by WriteTimeout.
func TestServerTimeouts(t *testing.T) {
	// 1. Setup minimal server dependencies
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	defer database.Close()

	claudeDir := filepath.Join(dir, "claude")
	projectDir := filepath.Join(claudeDir, "test-project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("creating project dir: %v", err)
	}

	// Create a dummy session to watch
	sessionID := "watch-test"
	sessionPath := filepath.Join(projectDir, sessionID+".jsonl")
	if err := os.WriteFile(
		sessionPath,
		[]byte(`{"type":"user"}`),
		0o644,
	); err != nil {
		t.Fatalf("writing session file: %v", err)
	}

	// 2. Start server on a random port
	port := server.FindAvailablePort(40000)
	// Set a very short WriteTimeout to verify SSE is exempt.
	// If SSE were subject to this timeout, the connection would close
	// well before our 500ms wait below.
	cfg := config.Config{
		Host:         "127.0.0.1",
		Port:         port,
		DataDir:      dir,
		DBPath:       dbPath,
		WriteTimeout: 100 * time.Millisecond,
	}
	engine := sync.NewEngine(database, claudeDir, "", "test")
	srv := server.New(cfg, database, engine)

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- srv.ListenAndServe()
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// 3. Connect to the SSE endpoint
	url := fmt.Sprintf("http://127.0.0.1:%d/api/v1/sessions/%s/watch", port, sessionID)
	// Use a context with timeout to prevent test hanging
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}

	client := &http.Client{
		// Client timeout doesn't matter much with context, but good practice.
		Timeout: 0,
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	// 4. Trigger an update shortly after connecting to unblock Read
	// We wait 500ms, which is > WriteTimeout (100ms).
	// If the handler had a timeout, the response body would be closed/error'd by now.
	errCh := make(chan error, 1)
	go func() {
		time.Sleep(500 * time.Millisecond)
		if err := os.WriteFile(
			sessionPath,
			[]byte(`{"type":"user","content":"update"}`),
			0o644,
		); err != nil {
			errCh <- fmt.Errorf("writing update: %w", err)
			return
		}
		close(errCh)
	}()

	// 5. Read from the stream to verify it's open and receives data
	// This Read will block until data arrives or context times out.
	buf := make([]byte, 1024)
	n, err := resp.Body.Read(buf)
	
	// Check if update writer failed
	if writeErr := <-errCh; writeErr != nil {
		t.Fatalf("update writer failed: %v", writeErr)
	}

	if n == 0 && err != nil {
		t.Fatalf("failed to read bytes (timeout or closed?): %v", err)
	}
	t.Logf("Received %d bytes from SSE stream", n)

	// Check if server error occurred
	select {
	case err := <-serverErr:
		t.Fatalf("server exited unexpectedly: %v", err)
	default:
		// Server running
	}
}
