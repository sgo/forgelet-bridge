package bridge

import (
	"context"
	"errors"
	"testing"

	"github.com/unclebob/forgelet-bridge/internal/relay"
	"github.com/unclebob/forgelet-bridge/internal/state"
)

// carryOutFacts is the bridge's state writer, and a tick leans on every edge of
// it: what it reports writing and clearing, whether what it wrote survives a
// restart, and which room's facts it may touch. These tests drive the writer
// directly and through a tick so those edges are pinned rather than assumed.

func approvalStateFact(key, marker string) relay.Fact {
	return relay.Fact{
		Type:    relay.ApprovalFactType,
		Key:     key,
		Content: map[string]any{"id": key, "task": marker},
	}
}

func TestCarryOutFactsReportsEveryStateItWritesAndClears(t *testing.T) {
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithStores(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": &fakeApprovals{}},
		map[string]BoardStore{"/forges/forge-a": &fakeBoard{}}, "/forges/forge-a")
	const room = "!approvals-forge-a"

	written, err := built.carryOutFacts(context.Background(), room, relay.ApprovalFactType,
		[]relay.Fact{approvalStateFact("a", "one"), approvalStateFact("b", "two")})
	if err != nil {
		t.Fatalf("first carry: %v", err)
	}
	if written != 2 {
		t.Fatalf("first carry wrote %d, want the two new facts", written)
	}

	// One fact changes and the other stops waiting in the same pass: the writer
	// reports both, and the state it saved holds the change and not the stale
	// fact.
	changed, err := built.carryOutFacts(context.Background(), room, relay.ApprovalFactType,
		[]relay.Fact{approvalStateFact("a", "three")})
	if err != nil {
		t.Fatalf("second carry: %v", err)
	}
	if changed != 2 {
		t.Fatalf("second carry reported %d, want the changed fact and the cleared one", changed)
	}
	saved, err := state.Load(built.statePath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, stale := saved.Relay.Waiting[waitingKey(room, relay.ApprovalFactType, "b")]; stale {
		t.Errorf("the cleared fact is still remembered: %v", saved.Relay.Waiting)
	}
	if want, _ := factContent(approvalStateFact("a", "three")); saved.Relay.Waiting[waitingKey(room, relay.ApprovalFactType, "a")] != want {
		t.Errorf("the saved state = %v, want the changed fact remembered", saved.Relay.Waiting)
	}

	// One changed fact alone is one event, and it is reported as one.
	alone, err := built.carryOutFacts(context.Background(), room, relay.ApprovalFactType,
		[]relay.Fact{approvalStateFact("a", "four")})
	if err != nil {
		t.Fatalf("third carry: %v", err)
	}
	if alone != 1 {
		t.Fatalf("third carry reported %d, want the one changed fact", alone)
	}
}

func TestCarryOutFactsSurfacesAStateItCannotSave(t *testing.T) {
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithStores(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": &fakeApprovals{}},
		map[string]BoardStore{"/forges/forge-a": &fakeBoard{}}, "/forges/forge-a")
	// A directory is not a state file: the save the writer makes must fail
	// loudly rather than be dropped.
	built.statePath = t.TempDir()

	if _, err := built.carryOutFacts(context.Background(), "!approvals-forge-a", relay.ApprovalFactType,
		[]relay.Fact{approvalStateFact("a", "one")}); err == nil {
		t.Fatal("carryOutFacts: want the save failure, got nil")
	}
}

func TestCarryOutFactsLeavesAnotherRoomsFactsAlone(t *testing.T) {
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithStores(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": &fakeApprovals{}},
		map[string]BoardStore{"/forges/forge-a": &fakeBoard{}}, "/forges/forge-a")
	built.state.Relay.EnsureMaps()

	const mine = "!activity-forge-a"
	otherRoom := "!approvals-forge-a"
	// Another room's fact of the same type, and this room's fact of another
	// type: neither is this writer's to clear.
	built.state.Relay.Waiting[waitingKey(otherRoom, relay.BoardFactType, "a")] = `{"projects":[]}`
	built.state.Relay.Waiting[waitingKey(mine, relay.ApprovalFactType, "b")] = `{"id":"b"}`
	built.state.Relay.Waiting[waitingKey(mine, relay.BoardFactType, "forgelet-bridge")] = `{"projects":[]}`

	// Nothing waits in the activity room, so its own stale fact is cleared -
	// and only it: another room's facts, and this room's facts of another type,
	// are neither this writer's to clear.
	if _, err := built.carryOutFacts(context.Background(), mine, relay.BoardFactType, nil); err != nil {
		t.Fatalf("carryOutFacts: %v", err)
	}
	states := rooms.sentStates()
	if len(states) != 1 || states[0].roomID != mine || states[0].stateKey != "forgelet-bridge" {
		t.Fatalf("states = %+v, want only the activity room's own fact cleared", states)
	}
	if _, kept := built.state.Relay.Waiting[waitingKey(otherRoom, relay.BoardFactType, "a")]; !kept {
		t.Error("the writer forgot a fact another room carries")
	}
	if _, kept := built.state.Relay.Waiting[waitingKey(mine, relay.ApprovalFactType, "b")]; !kept {
		t.Error("the writer forgot this room's fact of another type")
	}
}

func TestCarryOutFactsSurfacesAStateItCannotClear(t *testing.T) {
	rooms := &fakeRooms{stateErr: map[string]error{"!activity-forge-a": errors.New("the room is gone")}}
	built, _ := newTestBridgeWithStores(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": &fakeApprovals{}},
		map[string]BoardStore{"/forges/forge-a": &fakeBoard{}}, "/forges/forge-a")
	built.state.Relay.EnsureMaps()
	built.state.Relay.Waiting[waitingKey("!activity-forge-a", relay.BoardFactType, "forgelet-bridge")] = `{"projects":[]}`

	if _, err := built.carryOutFacts(context.Background(), "!activity-forge-a", relay.BoardFactType, nil); err == nil {
		t.Fatal("carryOutFacts: want the clear failure, got nil")
	}
}

func TestTickReportsAForgeWhoseFactsCannotBeWritten(t *testing.T) {
	cases := map[string]struct {
		roomID string
		build  func(t *testing.T, rooms *fakeRooms) *Bridge
	}{
		"activity": {roomID: "!activity-forge-a", build: func(t *testing.T, rooms *fakeRooms) *Bridge {
			board := &fakeBoard{lanes: []string{"specifier", "coder"}}
			board.set(boardCard("coder"))
			built, _ := newTestBridgeWithStores(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
				map[string]ApprovalStore{"/forges/forge-a": &fakeApprovals{}},
				map[string]BoardStore{"/forges/forge-a": board}, "/forges/forge-a")
			return built
		}},
		"approvals": {roomID: "!approvals-forge-a", build: func(t *testing.T, rooms *fakeRooms) *Bridge {
			built, _ := newTestBridgeWithApprovals(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
				map[string]ApprovalStore{"/forges/forge-a": &fakeApprovals{pending: []relay.Approval{phoneApproval()}}}, "/forges/forge-a")
			return built
		}},
		"clarifications": {roomID: "!clarifications-forge-a", build: func(t *testing.T, rooms *fakeRooms) *Bridge {
			store := &fakeClarifications{pending: []relay.Clarification{clarificationOf("forgelet-bridge", "clar-1", "coder", "which lane?")}}
			built, _ := newTestBridgeWithClarifications(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
				map[string]ClarificationStore{"/forges/forge-a": store}, "/forges/forge-a")
			return built
		}},
	}

	for name, one := range cases {
		rooms := &fakeRooms{stateErr: map[string]error{one.roomID: errors.New("the room is gone")}}
		built := one.build(t, rooms)
		if err := built.Tick(context.Background()); err != nil {
			t.Fatalf("%s: Tick: %v", name, err)
		}
		status := statusOf(t, built)
		if len(status.UnhappyForges) != 1 || status.UnhappyForges[0] != "forge-a" {
			t.Errorf("%s: unhappy forges = %v, want the forge whose %s state cannot be written", name, status.UnhappyForges, name)
		}
	}
}
