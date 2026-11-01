package auth

import (
	"testing"
	"time"
)

// Deleting every User's stale known devices reads the whole table, so sign-in
// does it once a day however many sign-ins there are.
func TestAPeriodicJobIsDueOncePerInterval(t *testing.T) {
	w := &periodic{interval: 24 * time.Hour}

	got := []bool{
		w.due(someTime),
		w.due(someTime.Add(time.Minute)),
		w.due(someTime.Add(24*time.Hour - time.Second)),
		w.due(someTime.Add(24 * time.Hour)),
		w.due(someTime.Add(25 * time.Hour)),
		w.due(someTime.Add(48 * time.Hour)),
	}

	want := []bool{true, false, false, true, false, true}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("due = %v, want %v", got, want)
			break
		}
	}
}
