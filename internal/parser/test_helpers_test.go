package parser

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// --- JSON Builders ---

func claudeUserJSON(content, timestamp string, cwd ...string) string {
	m := map[string]any{
		"type":      "user",
		"timestamp": timestamp,
		"message": map[string]any{
			"content": content,
		},
	}
	if len(cwd) > 0 {
		m["cwd"] = cwd[0]
	}
	b, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func claudeAssistantJSON(content any, timestamp string) string {
	m := map[string]any{
		"type":      "assistant",
		"timestamp": timestamp,
		"message": map[string]any{
			"content": content,
		},
	}
	b, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func claudeSnapshotJSON(timestamp string) string {
	m := map[string]any{
		"type": "user",
		"snapshot": map[string]any{
			"timestamp": timestamp,
		},
		"message": map[string]any{
			"content": "hello",
		},
	}
	b, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func codexSessionMetaJSON(id, cwd, originator, timestamp string) string {
	m := map[string]any{
		"type":      "session_meta",
		"timestamp": timestamp,
		"payload": map[string]any{
			"id":         id,
			"cwd":        cwd,
			"originator": originator,
		},
	}
	b, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func codexMsgJSON(role, text, timestamp string) string {
	m := map[string]any{
		"type":      "response_item",
		"timestamp": timestamp,
		"payload": map[string]any{
			"role": role,
			"content": []map[string]string{
				{
					"type": getCodexContentType(role),
					"text": text,
				},
			},
		},
	}
	b, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func getCodexContentType(role string) string {
	if role == "user" {
		return "input_text"
	}
	return "output_text"
}

// --- Data Generators ---

func generateLargeString(size int) string {
	return strings.Repeat("x", size)
}

// --- Assertions ---

func assertSessionMeta(t *testing.T, s *ParsedSession, wantID, wantProject string, wantAgent AgentType) {
	t.Helper()
	if s == nil {
		t.Fatal("session is nil")
	}
	if s.ID != wantID {
		t.Errorf("session ID = %q, want %q", s.ID, wantID)
	}
	if s.Project != wantProject {
		t.Errorf("project = %q, want %q", s.Project, wantProject)
	}
	if s.Agent != wantAgent {
		t.Errorf("agent = %q, want %q", s.Agent, wantAgent)
	}
}

func assertMessage(t *testing.T, m ParsedMessage, wantRole RoleType, wantContentSnippet string) {
	t.Helper()
	if m.Role != wantRole {
		t.Errorf("role = %q, want %q", m.Role, wantRole)
	}
	if wantContentSnippet != "" && !strings.Contains(m.Content, wantContentSnippet) {
		t.Errorf("content missing snippet %q, got %q", wantContentSnippet, m.Content)
	}
}

func assertMessageCount(t *testing.T, count, want int) {
	t.Helper()
	if count != want {
		t.Fatalf("message count = %d, want %d", count, want)
	}
}

func assertTimestamp(t *testing.T, got time.Time, want time.Time) {
	t.Helper()
	if !got.Equal(want) {
		t.Errorf("timestamp = %v, want %v", got, want)
	}
}

func joinJSONL(lines ...string) string {
	return strings.Join(lines, "\n") + "\n"
}
