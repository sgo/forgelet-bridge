package bridge

import (
	"context"
	"testing"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

func TestTickCarriesTheBoardsStateAndLeavesItAloneWhenNothingChanged(t *testing.T) {
	board := &fakeBoard{lanes: []string{"specifier", "coder"}}
	board.set(boardCard("coder"))
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithStores(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": &fakeApprovals{}},
		map[string]BoardStore{"/forges/forge-a": board}, "/forges/forge-a")

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick: %v", err)
	}
	states := rooms.sentStates()
	if len(states) != 1 || states[0].eventType != relay.BoardFactType || states[0].stateKey != "forgelet-bridge" {
		t.Fatalf("states = %+v, want one board state keyed by the project", states)
	}
	if states[0].roomID != "!activity-forge-a" {
		t.Errorf("room = %q, want the activity room", states[0].roomID)
	}

	rooms.states = nil
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick: %v", err)
	}
	if states := rooms.sentStates(); len(states) != 0 {
		t.Errorf("states = %+v, want the same facts left alone", states)
	}
}

func TestTickRewritesTheBoardsStateWhenACardMovesAndClearsItWhenItStops(t *testing.T) {
	board := &fakeBoard{lanes: []string{"specifier", "coder"}}
	board.set(boardCard("specifier"))
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithStores(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": &fakeApprovals{}},
		map[string]BoardStore{"/forges/forge-a": board}, "/forges/forge-a")
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick: %v", err)
	}
	rooms.states = nil

	board.set(boardCard("coder"))
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick: %v", err)
	}
	if states := rooms.sentStates(); len(states) != 1 || states[0].content["projects"] == nil {
		t.Fatalf("states = %+v, want the moved card written again", rooms.sentStates())
	}
	rooms.states = nil

	// The project closes: its board stops waiting and its state is cleared.
	board.set()
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("third Tick: %v", err)
	}
	states := rooms.sentStates()
	if len(states) != 1 || len(states[0].content) != 0 {
		t.Fatalf("states = %+v, want the board cleared with empty content", states)
	}
}

func TestTickCarriesAnApprovalAsStateAndClearsItWhenItStopsWaiting(t *testing.T) {
	store := &fakeApprovals{pending: []relay.Approval{phoneApproval()}}
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithApprovals(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": store}, "/forges/forge-a")
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick: %v", err)
	}
	states := rooms.sentStates()
	if len(states) != 1 || states[0].eventType != relay.ApprovalFactType || states[0].stateKey != "approval-1" {
		t.Fatalf("states = %+v, want one approval state keyed by the approval", states)
	}
	if states[0].roomID != "!approvals-forge-a" {
		t.Errorf("room = %q, want the approvals room", states[0].roomID)
	}
	rooms.states = nil

	// The approval is resolved: it stops waiting and its state is cleared.
	store.setPending()
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick: %v", err)
	}
	states = rooms.sentStates()
	if len(states) != 1 || states[0].stateKey != "approval-1" || len(states[0].content) != 0 {
		t.Fatalf("states = %+v, want the resolved approval cleared", states)
	}
}

func TestTickCarriesAClarificationAsStateAndClearsItWhenAnswered(t *testing.T) {
	clarification := clarificationOf("forgelet-bridge", "clar-1", "coder", "which lane?")
	store := &fakeClarifications{pending: []relay.Clarification{clarification}}
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithClarifications(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ClarificationStore{"/forges/forge-a": store}, "/forges/forge-a")
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick: %v", err)
	}
	states := rooms.sentStates()
	if len(states) != 1 || states[0].eventType != relay.ClarificationFactType || states[0].stateKey != "clar-1" {
		t.Fatalf("states = %+v, want one clarification state keyed by the question", states)
	}
	if states[0].roomID != "!clarifications-forge-a" {
		t.Errorf("room = %q, want the clarifications room", states[0].roomID)
	}
	rooms.states = nil

	store.mu.Lock()
	store.pending = nil
	store.mu.Unlock()
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick: %v", err)
	}
	states = rooms.sentStates()
	if len(states) != 1 || states[0].stateKey != "clar-1" || len(states[0].content) != 0 {
		t.Fatalf("states = %+v, want the answered question cleared", states)
	}
}

func TestACarriedFactSurvivesARestartWithoutRewriting(t *testing.T) {
	store := &fakeApprovals{pending: []relay.Approval{phoneApproval()}}
	rooms := &fakeRooms{}
	built, cfg := newTestBridgeWithApprovals(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": store}, "/forges/forge-a")
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick: %v", err)
	}

	restartedRooms := &fakeRooms{}
	restarted, err := New(cfg, restartedRooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": store},
		map[string]ClarificationStore{"/forges/forge-a": &fakeClarifications{}},
		map[string]BoardStore{"/forges/forge-a": &fakeBoard{}}, nil)
	if err != nil {
		t.Fatalf("New after restart: %v", err)
	}
	if err := restarted.Tick(context.Background()); err != nil {
		t.Fatalf("Tick after restart: %v", err)
	}

	if states := restartedRooms.sentStates(); len(states) != 0 {
		t.Errorf("states after restart = %+v, want nothing the room already holds", states)
	}
}
