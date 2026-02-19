package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMiddleware_Timeout(t *testing.T) {
	te := setup(t)
	// Seed some data so handlers don't fail with 404 before checking context
	te.seedSession(t, "s1", "my-app", 10)
	te.seedMessages(t, "s1", 10)

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"ListSessions", "GET", "/api/v1/sessions"},
		{"GetSession", "GET", "/api/v1/sessions/s1"},
		{"GetMessages", "GET", "/api/v1/sessions/s1/messages"},
		{"GetMinimap", "GET", "/api/v1/sessions/s1/minimap"},
		{"GetStats", "GET", "/api/v1/stats"},
		{"ListProjects", "GET", "/api/v1/projects"},
		{"ListMachines", "GET", "/api/v1/machines"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a context that is already past its deadline
			// This forces the middleware (http.TimeoutHandler) to timeout immediately
			ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Hour))
			defer cancel()

			req := httptest.NewRequest(tt.method, tt.path, nil).WithContext(ctx)
			w := httptest.NewRecorder()
			te.handler.ServeHTTP(w, req)

			// Expect 503 Service Unavailable from middleware
			if w.Code != http.StatusServiceUnavailable {
				t.Errorf("expected status 503, got %d. Body: %s", w.Code, w.Body.String())
			}

			// Check that response is JSON
			if contentType := w.Header().Get("Content-Type"); contentType != "application/json" {
				t.Errorf("expected Content-Type application/json, got %s", contentType)
			}

			// Check body contains "request timed out"
			if !strings.Contains(w.Body.String(), "request timed out") {
				t.Errorf("expected body to contain 'request timed out', got %s", w.Body.String())
			}
		})
	}
}
