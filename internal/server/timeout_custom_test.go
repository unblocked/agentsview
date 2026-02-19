package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wesm/agentsview/internal/config"
)

func TestWithTimeout_Timeout(t *testing.T) {
	t.Parallel()
	
	cfg := config.Config{
		WriteTimeout: 10 * time.Millisecond,
	}
	s := &Server{
		cfg: cfg,
	}

	slowHandler := func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("too slow"))
	}

	wrapped := s.withTimeout(slowHandler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503 Service Unavailable, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type: application/json, got %q", contentType)
	}

	body, _ := io.ReadAll(resp.Body)
	var errResp jsonError
	if err := json.Unmarshal(body, &errResp); err != nil {
		t.Errorf("expected JSON body, got error %v: %s", err, string(body))
	}
	if errResp.Error != "request timed out" {
		t.Errorf("expected error message 'request timed out', got %q", errResp.Error)
	}
}

func TestWithTimeout_Success(t *testing.T) {
	t.Parallel()
	
	cfg := config.Config{
		WriteTimeout: 100 * time.Millisecond,
	}
	s := &Server{
		cfg: cfg,
	}

	fastHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "value")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"status":"ok"}`))
	}

	wrapped := s.withTimeout(fastHandler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	wrapped.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201 Created, got %d", resp.StatusCode)
	}

	if val := resp.Header.Get("X-Custom"); val != "value" {
		t.Errorf("expected X-Custom header 'value', got %q", val)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"status":"ok"}` {
		t.Errorf("expected body '{\"status\":\"ok\"}', got %q", string(body))
	}
}
