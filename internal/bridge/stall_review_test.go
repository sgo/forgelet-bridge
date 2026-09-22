package bridge

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/unclebob/forgelet-bridge/internal/relay"
	"github.com/unclebob/forgelet-bridge/internal/state"
)

// TestTickReportsWorkWhenItPostsOnlyAnApproval checks the approvals half of a
// tick on its own: work carried out there is work, so the tick is not idle.
func TestTickReportsWorkWhenItPostsOnlyAnApproval(t *testing.T) {
	store := &fakeApprovals{pending: []relay.Approval{phoneApproval()}}
	rooms := &fakeRooms{}
	built, cfg := newTestBridgeWithStores(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": store},
		map[string]BoardStore{"/forges/forge-a": &fakeBoard{}}, "/forges/forge-a")

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	if status := readStatus(t, filepath.Join(cfg.StateDir, StatusName)); status.Idle {
		t.Errorf("status = %+v, want the tick that posted an approval to report work", status)
	}
}

// TestTickReportsWorkWhenItPostsOnlyACardUpdate is the same for the activity
// room: a card update on its own is work.
func TestTickReportsWorkWhenItPostsOnlyACardUpdate(t *testing.T) {
	board := &fakeBoard{}
	board.set(boardCard("specifier"))
	rooms := &fakeRooms{}
	built, cfg := newTestBridgeWithStores(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": &fakeApprovals{}},
		map[string]BoardStore{"/forges/forge-a": board}, "/forges/forge-a")

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	if status := readStatus(t, filepath.Join(cfg.StateDir, StatusName)); status.Idle {
		t.Errorf("status = %+v, want the tick that posted a card update to report work", status)
	}
}

// TestTickReportsAndSavesAPairingOnItsOwn checks the third kind of work a tick
// does: pairing a message the operator sent with the request the dashboard took
// for it. That is work, and it has to reach the state file, or a restart would
// pair it again.
func TestTickReportsAndSavesAPairingOnItsOwn(t *testing.T) {
	store := &fakeStore{requests: []relay.Request{{ID: "req-1", Body: "is the build green?"}}}
	rooms := &fakeRooms{}
	built, cfg := newTestBridge(t, rooms, map[string]ForgeStore{"/forges/forge-a": store}, "/forges/forge-a")
	bookkeeping := built.State()
	bookkeeping.Relay.EnsureMaps()
	// The request is already in the room, so the plan has nothing to do; the
	// operator's message is still waiting to be paired with it.
	bookkeeping.Relay.Threads["req-1"] = "$posted"
	bookkeeping.Relay.Pending["$event-1"] = "is the build green?"

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	if paired := bookkeeping.Relay.Relayed["$event-1"]; paired != "req-1" {
		t.Errorf("relayed = %q, want the operator's message paired with the request", paired)
	}
	if status := readStatus(t, filepath.Join(cfg.StateDir, StatusName)); status.Idle {
		t.Errorf("status = %+v, want the tick that paired a request to report work", status)
	}
	saved, err := state.Load(filepath.Join(cfg.StateDir, "bridge-state.json"))
	if err != nil {
		t.Fatalf("read the saved state: %v", err)
	}
	if saved.Relay.Relayed["$event-1"] != "req-1" {
		t.Errorf("saved relay state = %+v, want the pairing saved", saved.Relay.Relayed)
	}
}

// TestTickStaysQuietWhenAMessagePairsWithNothing checks the other side of the
// same rule: a message waiting for a request the queue never took is no work
// done, so the tick stays idle.
func TestTickStaysQuietWhenAMessagePairsWithNothing(t *testing.T) {
	store := &fakeStore{requests: []relay.Request{{ID: "req-1", Body: "please retry the invoice card"}}}
	rooms := &fakeRooms{}
	built, cfg := newTestBridge(t, rooms, map[string]ForgeStore{"/forges/forge-a": store}, "/forges/forge-a")
	bookkeeping := built.State()
	bookkeeping.Relay.EnsureMaps()
	bookkeeping.Relay.Threads["req-1"] = "$posted"
	bookkeeping.Relay.Pending["$event-1"] = "is the build green?"

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	if requestID := bookkeeping.Relay.Relayed["$event-1"]; requestID != "" {
		t.Errorf("relayed = %q, want a message the queue never took left unpaired", requestID)
	}
	if status := readStatus(t, filepath.Join(cfg.StateDir, StatusName)); !status.Idle {
		t.Errorf("status = %+v, want the tick that paired nothing to stay idle", status)
	}
}

// TestTickPairsEachMessageWithItsOwnRequest checks that taking a request for one
// message takes it for good: two messages reading the same thing are two
// messages, and a request answers one of them.
func TestTickPairsEachMessageWithItsOwnRequest(t *testing.T) {
	store := &fakeStore{requests: []relay.Request{{ID: "req-1", Body: "is the build green?"}}}
	rooms := &fakeRooms{}
	built, _ := newTestBridge(t, rooms, map[string]ForgeStore{"/forges/forge-a": store}, "/forges/forge-a")
	bookkeeping := built.State()
	bookkeeping.Relay.EnsureMaps()
	bookkeeping.Relay.Threads["req-1"] = "$posted"
	bookkeeping.Relay.Pending["$event-1"] = "is the build green?"
	bookkeeping.Relay.Pending["$event-2"] = "is the build green?"

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	if len(bookkeeping.Relay.Relayed) != 1 {
		t.Errorf("relayed = %+v, want one message paired with the queue's one request", bookkeeping.Relay.Relayed)
	}
}

// TestAnswerGesturesSaysNothingWhenTheRoomTakesIt checks the answer the bridge
// gives the approvals room: a room that takes it is not a failure.
func TestAnswerGesturesSaysNothingWhenTheRoomTakesIt(t *testing.T) {
	rooms := &fakeRooms{}
	built, _ := newTestBridge(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}}, "/forges/forge-a")

	if err := built.answerGestures(context.Background(), Room{ApprovalsRoomID: "!approvals-forge-a"}); err != nil {
		t.Errorf("answerGestures = %v, want the room to have taken the answer", err)
	}
}

// TestTickReportsTheReplyWhenTheOperatorSendsAnApprovalBack checks the whole
// send-back: the forge records the feedback and the room is told, so the
// operator's phone shows what happened.
func TestTickReportsTheReplyWhenTheOperatorSendsAnApprovalBack(t *testing.T) {
	store := &fakeApprovals{pending: []relay.Approval{phoneApproval()}}
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithApprovals(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": store}, "/forges/forge-a")
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick: %v", err)
	}
	messageID := built.State().Relay.Approvals["forgelet-bridge/approval-1"].MessageID
	rooms.push(relay.RoomEvent{
		RoomID:     "!approvals-forge-a",
		EventID:    "$reply",
		Sender:     operator,
		Body:       "the timesheet total is still wrong",
		ThreadRoot: messageID,
	})

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick: %v", err)
	}
	// The room hears how the approval was resolved on the tick after the forge
	// has been told, so the reply is the third tick's work.
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("third Tick: %v", err)
	}

	sent := rooms.sentMessages()
	if len(sent) != 2 || sent[1].body != "Sent back with feedback" {
		t.Errorf("sent = %+v, want the approval message and the room's reply", sent)
	}
}
