package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/wesm/agentsview/internal/config"
	"github.com/wesm/agentsview/internal/db"
	"github.com/wesm/agentsview/internal/sync"
)

// TestContentTypeWrapper verifies that Content-Type is only set if missing
// when the status code matches the trigger status.
func TestContentTypeWrapper(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		handler        http.HandlerFunc
		triggerStatus  int
		wantStatus     int
		wantContentType string
		wantBody       string
	}{
		{
			name: "SetsContentTypeOnTriggerStatusMissingHeader",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte(`{"error":"timeout"}`))
			},
			triggerStatus:   http.StatusServiceUnavailable,
			wantStatus:      http.StatusServiceUnavailable,
			wantContentType: "application/json",
			wantBody:        `{"error":"timeout"}`,
		},
		{
			name: "RespectsExistingContentTypeOnTriggerStatus",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte("timeout error"))
			},
			triggerStatus:   http.StatusServiceUnavailable,
			wantStatus:      http.StatusServiceUnavailable,
			wantContentType: "text/plain",
			wantBody:        "timeout error",
		},
		{
			name: "IgnoresNonTriggerStatus",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ok"))
			},
			triggerStatus:   http.StatusServiceUnavailable,
			wantStatus:      http.StatusOK,
			wantContentType: "", // Not set by wrapper
			wantBody:        "ok",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			wrapper := &contentTypeWrapper{
				ResponseWriter: w,
				contentType:    "application/json",
				triggerStatus:  tt.triggerStatus,
			}

			req := httptest.NewRequest("GET", "/", nil)
			tt.handler(wrapper, req)

			resp := w.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("status code = %d, want %d", resp.StatusCode, tt.wantStatus)
			}

			gotCT := resp.Header.Get("Content-Type")
			if tt.wantContentType == "" {
				// Wrapper must NOT force application/json on non-trigger statuses.
				// Content-Type may be sniffed by httptest, but must not be
				// the wrapper's configured type.
				if gotCT == "application/json" {
					t.Errorf(
						"Content-Type = %q; wrapper should not set it for non-trigger status",
						gotCT,
					)
				}
			} else if gotCT != tt.wantContentType {
				t.Errorf("Content-Type = %q, want %q", gotCT, tt.wantContentType)
			}

			body, _ := io.ReadAll(resp.Body)
			if string(body) != tt.wantBody {
				t.Errorf("body = %q, want %q", string(body), tt.wantBody)
			}
		})
	}
}

// TestWithTimeoutTriggersOnSlowHandler verifies that withTimeout produces a
// 503 JSON timeout response when the handler exceeds the configured duration.
func TestWithTimeoutTriggersOnSlowHandler(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	defer database.Close()

	cfg := config.Config{
		Host:         "127.0.0.1",
		Port:         0,
		DataDir:      dir,
		DBPath:       dbPath,
		WriteTimeout: 10 * time.Millisecond,
	}
	engine := sync.NewEngine(database, dir, "", "test")
	srv := New(cfg, database, engine)

	// Handler that blocks well past the timeout.
	slow := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
		// If we reach here after context cancel, TimeoutHandler
		// already wrote the 503.
	})

	handler := srv.withTimeout(slow)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/test")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}

	body, _ := io.ReadAll(resp.Body)
	var jsonErr jsonError
	if err := json.Unmarshal(body, &jsonErr); err != nil {
		t.Fatalf("body is not valid JSON: %v (body=%q)", err, string(body))
	}
	if jsonErr.Error != "request timed out" {
		t.Errorf("error = %q, want %q", jsonErr.Error, "request timed out")
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}

// TestRoutesTimeoutWiring verifies that API routes are wrapped with timeout
// middleware (positive assertion) and that export/SPA routes are NOT wrapped
// (negative assertion).
func TestRoutesTimeoutWiring(t *testing.T) {
	t.Parallel()

	isTimeoutResponse := func(t *testing.T, resp *http.Response) bool {
		t.Helper()
		if resp.StatusCode != http.StatusServiceUnavailable {
			return false
		}
		body, _ := io.ReadAll(resp.Body)
		var je jsonError
		if json.Unmarshal(body, &je) != nil {
			return false
		}
		return je.Error == "request timed out"
	}

	// Positive: wrapped routes must produce a timeout with an
	// impossibly short deadline (1 ns). Any real handler (DB query,
	// serialization, etc.) will exceed this.
	t.Run("WrappedRoutesTimeout", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		dbPath := filepath.Join(dir, "test.db")
		database, err := db.Open(dbPath)
		if err != nil {
			t.Fatalf("opening db: %v", err)
		}
		defer database.Close()

		cfg := config.Config{
			Host:         "127.0.0.1",
			Port:         0,
			DataDir:      dir,
			DBPath:       dbPath,
			WriteTimeout: time.Nanosecond,
		}
		engine := sync.NewEngine(database, dir, "", "test")
		srv := New(cfg, database, engine)

		ts := httptest.NewServer(srv.Handler())
		defer ts.Close()

		wrapped := []struct {
			name string
			path string
		}{
			{"ListSessions", "/api/v1/sessions"},
			{"GetStats", "/api/v1/stats"},
		}

		for _, tt := range wrapped {
			t.Run(tt.name, func(t *testing.T) {
				resp, err := ts.Client().Get(ts.URL + tt.path)
				if err != nil {
					t.Fatalf("request failed: %v", err)
				}
				defer resp.Body.Close()

				if !isTimeoutResponse(t, resp) {
					t.Errorf(
						"%s: expected timeout 503, got %d",
						tt.path, resp.StatusCode,
					)
				}
			})
		}
	})

	// Negative: unwrapped routes must NOT produce a timeout response,
	// even with the same short deadline.
	t.Run("UnwrappedRoutesNoTimeout", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		dbPath := filepath.Join(dir, "test.db")
		database, err := db.Open(dbPath)
		if err != nil {
			t.Fatalf("opening db: %v", err)
		}
		defer database.Close()

		cfg := config.Config{
			Host:         "127.0.0.1",
			Port:         0,
			DataDir:      dir,
			DBPath:       dbPath,
			WriteTimeout: time.Nanosecond,
		}
		engine := sync.NewEngine(database, dir, "", "test")
		srv := New(cfg, database, engine)

		ts := httptest.NewServer(srv.Handler())
		defer ts.Close()

		unwrapped := []struct {
			name       string
			path       string
			wantStatus int
		}{
			{"ExportSession", "/api/v1/sessions/invalid-id/export",
				http.StatusNotFound},
			{"SPA", "/", http.StatusOK},
		}

		for _, tt := range unwrapped {
			t.Run(tt.name, func(t *testing.T) {
				resp, err := ts.Client().Get(ts.URL + tt.path)
				if err != nil {
					t.Fatalf("request failed: %v", err)
				}
				defer resp.Body.Close()

				if isTimeoutResponse(t, resp) {
					t.Errorf(
						"%s: unexpected timeout for unwrapped route",
						tt.path,
					)
				}
				if resp.StatusCode != tt.wantStatus {
					t.Errorf(
						"%s: status = %d, want %d",
						tt.path, resp.StatusCode, tt.wantStatus,
					)
				}
			})
		}
	})
}
