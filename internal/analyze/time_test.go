package analyze

import (
	"testing"
	"time"
)

func TestParseLast(t *testing.T) {
	_, err := ParseLast("90m")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = ParseLast("-1h")
	if err == nil {
		t.Fatalf("expected error for negative duration")
	}
}

func TestResolveWindowLast(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	start, end, err := ResolveWindow("", "", 90*time.Minute, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if end != now {
		t.Fatalf("expected end to be now")
	}
	if end.Sub(start) != 90*time.Minute {
		t.Fatalf("unexpected duration")
	}
}
