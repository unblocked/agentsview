package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// SSEStream manages a Server-Sent Events connection.
type SSEStream struct {
	w http.ResponseWriter
	f http.Flusher
}

// NewSSEStream initializes an SSE connection by setting the
// required headers and flushing them to the client. Returns an
// error if the ResponseWriter does not support streaming.
func NewSSEStream(w http.ResponseWriter) (*SSEStream, error) {
	f, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	f.Flush()
	return &SSEStream{w: w, f: f}, nil
}

// Send writes an SSE event with the given name and string data.
func (s *SSEStream) Send(event, data string) {
	fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event, data)
	s.f.Flush()
}

// SendJSON writes an SSE event with JSON-serialized data.
// Logs and skips the event if marshaling fails.
func (s *SSEStream) SendJSON(event string, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("SSE marshal error for %q: %v", event, err)
		return
	}
	s.Send(event, string(data))
}
