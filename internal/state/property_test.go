//go:build property

package state

import (
	"math/rand"
	"path/filepath"
	"reflect"
	"testing"
	"testing/quick"
)

// TestPropertySaveThenLoadKeepsTheState is the restart property of the state
// file: everything the bridge remembers survives being written and read back.
func TestPropertySaveThenLoadKeepsTheState(t *testing.T) {
	dir := t.TempDir()
	property := func(st State) bool {
		st.EnsureMaps()
		path := filepath.Join(dir, "bridge-state.json")
		if err := st.Save(path); err != nil {
			return false
		}
		loaded, err := Load(path)
		if err != nil {
			return false
		}
		return reflect.DeepEqual(*loaded, st)
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 200,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randomState(rnd))
		},
	}); err != nil {
		t.Error(err)
	}
}

// TestPropertyLoadOfAMissingFileIsAnEmptyState checks the first start: no
// state file is an empty state, not a failure.
func TestPropertyLoadOfAMissingFileIsAnEmptyState(t *testing.T) {
	dir := t.TempDir()
	property := func(name string) bool {
		loaded, err := Load(filepath.Join(dir, name, "never-written.json"))
		return err == nil && len(loaded.Forges) == 0 && len(loaded.Relay.Threads) == 0
	}
	if err := quick.Check(property, nil); err != nil {
		t.Error(err)
	}
}

func randomState(rnd *rand.Rand) State {
	state := State{}
	state.EnsureMaps()
	for count := rnd.Intn(4); count > 0; count-- {
		state.Forges[randomRoot(rnd)] = Forge{SpaceID: randomID(rnd), RoomID: randomID(rnd)}
	}
	for count := rnd.Intn(4); count > 0; count-- {
		state.Relay.Threads[randomID(rnd)] = randomID(rnd)
		state.Relay.Replied[randomID(rnd)] = randomID(rnd)
		state.Relay.Relayed[randomID(rnd)] = randomID(rnd)
	}
	return state
}

func randomRoot(rnd *rand.Rand) string {
	roots := []string{"", "/forges/forge-a", "/forges/forge-b", "relative/forge"}
	return roots[rnd.Intn(len(roots))]
}

func randomID(rnd *rand.Rand) string {
	ids := []string{"", "!space:example.org", "!room:example.org", "req-1", "$event-1", "two words"}
	return ids[rnd.Intn(len(ids))]
}
