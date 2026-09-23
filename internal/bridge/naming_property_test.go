//go:build property

package bridge

import (
	"context"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"
)

// TestPropertyEveryStartAppliesTheNameAndProvisionsNothingAgain checks what the
// operator's naming depends on: a forge the bridge already knows is refreshed
// once on every start, so a name the configuration changed reaches the rooms it
// already has, and it is never provisioned a second time by doing so.
func TestPropertyEveryStartAppliesTheNameAndProvisionsNothingAgain(t *testing.T) {
	property := func(starts uint8) bool {
		if starts == 0 {
			return true
		}
		rooms := &fakeRooms{}
		store := &fakeStore{}
		stores := map[string]ForgeStore{"/forges/forge-a": store}
		built, cfg := newTestBridge(t, rooms, stores, "/forges/forge-a")
		if err := built.Tick(context.Background()); err != nil {
			return false
		}

		for range starts - 1 {
			restarted, err := New(cfg, rooms, stores,
				map[string]ApprovalStore{"/forges/forge-a": &fakeApprovals{}},
				map[string]ClarificationStore{"/forges/forge-a": &fakeClarifications{}},
				map[string]BoardStore{"/forges/forge-a": &fakeBoard{}}, nil)
			if err != nil {
				return false
			}
			if err := restarted.Tick(context.Background()); err != nil {
				return false
			}
		}

		return len(rooms.ensured) == 1 && len(rooms.refreshes()) == int(starts)-1
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 40,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(uint8(1 + rnd.Intn(4)))
		},
	}); err != nil {
		t.Error(err)
	}
}
