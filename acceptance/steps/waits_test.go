package steps

import (
	"testing"

	"github.com/unclebob/forgelet-bridge/internal/bridge"
)

func TestSettledSinceWaitsForATickTheBridgeStartedAfterLooking(t *testing.T) {
	// A tick the bridge was already running when the step looked says nothing
	// about what the step wrote, so only the tick after it counts.
	cases := []struct {
		status bridge.Status
		seen   uint64
		want   bool
	}{
		{bridge.Status{Tick: 4, Idle: true}, 4, false},
		{bridge.Status{Tick: 5, Idle: true}, 4, false},
		{bridge.Status{Tick: 6, Idle: true}, 4, true},
		{bridge.Status{Tick: 7, Idle: true}, 4, true},
		{bridge.Status{Tick: 6, Idle: false}, 4, false},
	}
	for _, test := range cases {
		if got := settledSince(test.status, test.seen); got != test.want {
			t.Errorf("settledSince(%+v, %d) = %v, want %v", test.status, test.seen, got, test.want)
		}
	}
}
