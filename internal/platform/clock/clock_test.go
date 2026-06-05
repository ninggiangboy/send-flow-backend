package clock

import (
	"testing"
	"time"
)

func TestFixedClock_Advance(t *testing.T) {
	start := time.Date(2026, 5, 31, 10, 0, 0, 0, time.UTC)
	clock := NewFixed(start)

	clock.Advance(5 * time.Minute)

	if clock.Now() != start.Add(5*time.Minute) {
		t.Fatalf("unexpected time: %s", clock.Now())
	}
}
