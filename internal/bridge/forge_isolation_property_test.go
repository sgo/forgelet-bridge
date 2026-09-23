//go:build property

package bridge

import (
	"context"
	"errors"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

// TestPropertyTheForgeReportNamesEachConfiguredForge is the promise a second
// forge rests on: after a tick, the status names exactly the forges the bridge
// served and exactly the ones it could not - in the order the configuration
// names them, no more and no fewer, and the two lists between them name every
// configured forge once - while every forge it could serve still carried its
// rooms, so one sick forge is named on its own instead of quieting the rest.
func TestPropertyTheForgeReportNamesEachConfiguredForge(t *testing.T) {
	roots := []string{"/forges/forge-a", "/forges/forge-b", "/forges/forge-c"}
	property := func(sick []bool) bool {
		rooms := &fakeRooms{}
		stores := map[string]ForgeStore{}
		for i, root := range roots {
			store := &fakeStore{requests: []relay.Request{{ID: "req-" + root, Body: "message for " + root}}}
			if sick[i] {
				store.readErr = errors.New("the forge is not there")
			}
			stores[root] = store
		}
		built, cfg := newTestBridge(t, rooms, stores, roots...)

		var wantReached, wantUnhappy []string
		for i, root := range roots {
			if sick[i] {
				wantUnhappy = append(wantUnhappy, cfg.ForgeName(root))
			} else {
				wantReached = append(wantReached, cfg.ForgeName(root))
			}
		}

		if err := built.Tick(context.Background()); err != nil {
			return false
		}
		status := statusOf(t, built)
		if !reflect.DeepEqual(status.ReachedForges, wantReached) {
			return false
		}
		if !reflect.DeepEqual(status.UnhappyForges, wantUnhappy) {
			return false
		}
		// A named forge is a reason to report; a served one leaves nothing to
		// report.
		if (len(wantUnhappy) > 0) != (status.LastError != "") {
			return false
		}
		for i, root := range roots {
			if sick[i] {
				continue
			}
			if !sentBodyTo(rooms, "!room-"+cfg.ForgeName(root), "message for "+root) {
				return false
			}
		}
		return true
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 300,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			sick := make([]bool, len(roots))
			for i := range sick {
				sick[i] = rnd.Intn(2) == 0
			}
			values[0] = reflect.ValueOf(sick)
		},
	}); err != nil {
		t.Error(err)
	}
}

// TestPropertyAServedAgainForgeStopsBeingNamed is the other half of the retry
// promise: a forge the bridge could not serve is tried again on the next tick,
// and once it answers the status stops naming it while the forges that still
// fail stay named.
func TestPropertyAServedAgainForgeStopsBeingNamed(t *testing.T) {
	roots := []string{"/forges/forge-a", "/forges/forge-b"}
	property := func(healed bool) bool {
		rooms := &fakeRooms{}
		stores := map[string]ForgeStore{}
		for _, root := range roots {
			stores[root] = &fakeStore{readErr: errors.New("the forge is not there")}
		}
		built, cfg := newTestBridge(t, rooms, stores, roots...)
		if err := built.Tick(context.Background()); err != nil {
			return false
		}
		if got := statusOf(t, built).UnhappyForges; len(got) != len(roots) {
			return false // both forges start sick and are named
		}

		if healed {
			healthy := stores[roots[0]].(*fakeStore)
			healthy.mu.Lock()
			healthy.readErr = nil
			healthy.mu.Unlock()
		}
		if err := built.Tick(context.Background()); err != nil {
			return false
		}

		// Only the first forge is ever healed: the second stays sick, so it is
		// still named, and the healed one is named only when it was left sick.
		want := []string{cfg.ForgeName(roots[1])}
		if !healed {
			want = []string{cfg.ForgeName(roots[0]), cfg.ForgeName(roots[1])}
		}
		return reflect.DeepEqual(statusOf(t, built).UnhappyForges, want)
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 20,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(rnd.Intn(2) == 0)
		},
	}); err != nil {
		t.Error(err)
	}
}
