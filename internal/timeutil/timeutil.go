package timeutil

import "time"

// Ptr formats a time.Time to an RFC3339Nano string pointer
// for DB storage. Returns nil for zero time.
func Ptr(t time.Time) *string {
	if t.IsZero() {
		return nil
	}
	s := t.UTC().Format(time.RFC3339Nano)
	return &s
}

// Format formats a time.Time to an RFC3339Nano string for DB
// storage. Returns empty string for zero time.
func Format(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}
