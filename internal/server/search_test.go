package server

import "testing"

func TestPrepareFTSQuery(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"single word unchanged", "login", "login"},
		{"multi-word gets quoted", "fix bug", `"fix bug"`},
		{"already quoted unchanged", `"fix bug"`, `"fix bug"`},
		{"empty string unchanged", "", ""},
		{"three words quoted", "a b c", `"a b c"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := prepareFTSQuery(tt.raw)
			if got != tt.want {
				t.Errorf("prepareFTSQuery(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}
