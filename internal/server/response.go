package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
)

// writeJSON writes v as JSON with the given HTTP status code.
// Logs a warning if JSON encoding fails.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON: encoding response: %v", err)
	}
}

// writeError writes a JSON error response with the given status
// and message.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// handleContextError handles context.Canceled and context.DeadlineExceeded
// errors by writing the appropriate HTTP response. Returns true if the
// error was handled (i.e., it was a context error), false otherwise.
func handleContextError(w http.ResponseWriter, err error) bool {
	if errors.Is(err, context.Canceled) {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		writeError(w, http.StatusGatewayTimeout, "gateway timeout")
		return true
	}
	return false
}
