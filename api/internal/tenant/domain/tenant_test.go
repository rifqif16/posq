package domain

import (
	"testing"
	"time"
)

func TestTrialEndsAt(t *testing.T) {
	now := time.Date(2026, 9, 30, 8, 0, 0, 0, time.FixedZone("WIB", 7*3600))
	got := TrialEndsAt(now, 14)
	want := time.Date(2026, 10, 14, 1, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
