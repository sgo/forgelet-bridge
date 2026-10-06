//go:build property

package bridge

import (
	"context"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

// TestPropertyWaitingFactsAreWrittenOnceAndClearedOnce is the promise the rooms'
// state and every restart rest on: carrying out a set of facts writes each one
// once, carrying the same set out again writes nothing, and carrying out an
// empty set clears exactly the facts that stopped waiting, one empty event each,
// and then forgets them.
func TestPropertyWaitingFactsAreWrittenOnceAndClearedOnce(t *testing.T) {
	property := func(facts []relay.Fact) bool {
		rooms := &fakeRooms{}
		built, _ := newTestBridgeWithStores(t, rooms,
			map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
			map[string]ApprovalStore{"/forges/forge-a": &fakeApprovals{}},
			map[string]BoardStore{"/forges/forge-a": &fakeBoard{}}, "/forges/forge-a")
		ctx := context.Background()

		written, err := built.carryOutFacts(ctx, "!approvals-forge-a", relay.ApprovalFactType, facts, nil)
		if err != nil || written != len(facts) || len(rooms.sentStates()) != len(facts) {
			return false
		}

		rooms.states = nil
		again, err := built.carryOutFacts(ctx, "!approvals-forge-a", relay.ApprovalFactType, facts, nil)
		if err != nil || again != 0 || len(rooms.sentStates()) != 0 {
			return false
		}

		rooms.states = nil
		cleared, err := built.carryOutFacts(ctx, "!approvals-forge-a", relay.ApprovalFactType, nil, nil)
		if err != nil || cleared != len(facts) {
			return false
		}
		states := rooms.sentStates()
		if len(states) != len(facts) {
			return false
		}
		for _, state := range states {
			if len(state.content) != 0 {
				return false
			}
		}
		return true
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 40,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randomApprovalFacts(rnd))
		},
	}); err != nil {
		t.Error(err)
	}
}

// randomApprovalFacts is a set of approval facts with distinct keys, the way a
// room holds them: one fact per approval, its own id as the state key.
func randomApprovalFacts(rnd *rand.Rand) []relay.Fact {
	keys := []string{"approval-1", "approval-2", "approval-3", "approval-4"}
	rnd.Shuffle(len(keys), func(i, j int) { keys[i], keys[j] = keys[j], keys[i] })
	gates := []string{"coder → refactorer", "specifier → coder"}
	facts := make([]relay.Fact, 0, len(keys))
	for _, key := range keys[:rnd.Intn(len(keys)+1)] {
		facts = append(facts, relay.Fact{
			Type:    relay.ApprovalFactType,
			Key:     key,
			Content: map[string]any{"id": key, "gate": gates[rnd.Intn(len(gates))]},
		})
	}
	return facts
}
