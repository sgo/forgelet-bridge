//go:build property

package bridge

import (
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"
	"time"
)

// TestPropertyFailedTicksWaitLongerButNotForever checks the retry spacing the
// bridge uses when a tick fails: the wait never drops below the interval the
// bridge was told to use, never shrinks while failures continue, and never
// passes its ceiling however many ticks fail in a row.
func TestPropertyFailedTicksWaitLongerButNotForever(t *testing.T) {
	property := func(interval time.Duration, failures uint8) bool {
		if interval <= 0 || interval > maxBackOff {
			// The bridge backs off the interval it was configured with.
			return true
		}
		wait := interval
		for range failures {
			next := backOff(wait, interval)
			if next < interval || next < wait || next > maxBackOff {
				return false
			}
			wait = next
		}
		return true
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 300,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			intervals := []time.Duration{
				time.Millisecond, 200 * time.Millisecond, time.Second, 5 * time.Second, maxBackOff,
			}
			values[0] = reflect.ValueOf(intervals[rnd.Intn(len(intervals))])
			values[1] = reflect.ValueOf(uint8(rnd.Intn(256)))
		},
	}); err != nil {
		t.Error(err)
	}
}
