package timeutil

import (
	"testing"
	"time"
)

func TestPtr(t *testing.T) {
	t.Run("zero time returns nil", func(t *testing.T) {
		got := Ptr(time.Time{})
		if got != nil {
			t.Errorf("Ptr(zero) = %v, want nil", *got)
		}
	})

	t.Run("non-zero returns RFC3339Nano UTC", func(t *testing.T) {
		ts := time.Date(
			2024, 6, 15, 12, 30, 45, 123000000,
			time.UTC,
		)
		got := Ptr(ts)
		if got == nil {
			t.Fatal("Ptr returned nil for non-zero time")
		}
		want := "2024-06-15T12:30:45.123Z"
		if *got != want {
			t.Errorf("Ptr = %q, want %q", *got, want)
		}
	})

	t.Run("converts to UTC", func(t *testing.T) {
		loc := time.FixedZone("EST", -5*60*60)
		ts := time.Date(
			2024, 6, 15, 7, 30, 0, 0, loc,
		)
		got := Ptr(ts)
		if got == nil {
			t.Fatal("Ptr returned nil")
		}
		want := "2024-06-15T12:30:00Z"
		if *got != want {
			t.Errorf("Ptr = %q, want %q", *got, want)
		}
	})
}

func TestFormat(t *testing.T) {
	t.Run("zero time returns empty", func(t *testing.T) {
		got := Format(time.Time{})
		if got != "" {
			t.Errorf("Format(zero) = %q, want empty", got)
		}
	})

	t.Run("non-zero returns RFC3339Nano UTC", func(t *testing.T) {
		ts := time.Date(
			2024, 6, 15, 12, 30, 45, 0, time.UTC,
		)
		got := Format(ts)
		want := "2024-06-15T12:30:45Z"
		if got != want {
			t.Errorf("Format = %q, want %q", got, want)
		}
	})
}
