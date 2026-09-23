package bridge

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

// fakeBoard is a forge's project boards for the bridge tests.
type fakeBoard struct {
	mu    sync.Mutex
	cards []relay.Card
}

func (f *fakeBoard) Cards() ([]relay.Card, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]relay.Card(nil), f.cards...), nil
}

func (f *fakeBoard) set(cards ...relay.Card) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cards = cards
}

func boardCard(lane string) relay.Card {
	return relay.Card{
		Key:     "forgelet-bridge/card-activity-feed",
		Project: "forgelet-bridge",
		Name:    "card-activity-feed",
		Lane:    lane,
		Done:    lane == "done",
	}
}

func TestTickReportsWorkWhenItPostsACardUpdateAndAChatMessage(t *testing.T) {
	board := &fakeBoard{}
	board.set(boardCard("specifier"))
	rooms := &fakeRooms{}
	chat := &fakeStore{requests: []relay.Request{{ID: "req-1", Body: "is the build green?"}}}
	built, cfg := newTestBridgeWithStores(t, rooms, map[string]ForgeStore{"/forges/forge-a": chat},
		map[string]ApprovalStore{"/forges/forge-a": &fakeApprovals{}},
		map[string]BoardStore{"/forges/forge-a": board}, "/forges/forge-a")

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	if status := readStatus(t, filepath.Join(cfg.StateDir, StatusName)); status.Idle {
		t.Errorf("status = %+v, want the tick that posted a chat message and a card update to report work", status)
	}
}

func TestTickPostsEveryCardUpdateItOwes(t *testing.T) {
	board := &fakeBoard{}
	second := boardCard("specifier")
	second.Key = "forgelet-bridge/other-card"
	second.Name = "other-card"
	board.set(boardCard("specifier"), second)
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithStores(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": &fakeApprovals{}},
		map[string]BoardStore{"/forges/forge-a": board}, "/forges/forge-a")

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	if sent := rooms.sentMessages(); len(sent) != 2 {
		t.Errorf("sent = %+v, want one update per card", sent)
	}
}

func TestTickPostsACardThatAppeared(t *testing.T) {
	board := &fakeBoard{}
	board.set(boardCard("specifier"))
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithStores(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": &fakeApprovals{}},
		map[string]BoardStore{"/forges/forge-a": board}, "/forges/forge-a")

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	sent := rooms.sentMessages()
	if len(sent) != 1 {
		t.Fatalf("sent = %+v, want one card update", sent)
	}
	if sent[0].roomID != "!activity-forge-a" {
		t.Errorf("room = %q, want the activity room", sent[0].roomID)
	}
	want := "card card-activity-feed appeared in the project forgelet-bridge in the lane specifier"
	if sent[0].body != want {
		t.Errorf("update = %q, want %q", sent[0].body, want)
	}
}

func TestTickPostsThatACardMovedOn(t *testing.T) {
	board := &fakeBoard{}
	board.set(boardCard("specifier"))
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithStores(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": &fakeApprovals{}},
		map[string]BoardStore{"/forges/forge-a": board}, "/forges/forge-a")
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick: %v", err)
	}
	rooms.sent = nil
	board.set(boardCard("coder"))

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick: %v", err)
	}

	sent := rooms.sentMessages()
	want := "card card-activity-feed moved on in the project forgelet-bridge to the lane coder"
	if len(sent) != 1 || sent[0].body != want {
		t.Fatalf("sent = %+v, want %q", sent, want)
	}
}

func TestTickPostsThatACardFinished(t *testing.T) {
	board := &fakeBoard{}
	board.set(boardCard("coder"))
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithStores(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": &fakeApprovals{}},
		map[string]BoardStore{"/forges/forge-a": board}, "/forges/forge-a")
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick: %v", err)
	}
	rooms.sent = nil
	board.set(boardCard("done"))

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick: %v", err)
	}

	sent := rooms.sentMessages()
	want := "card card-activity-feed finished in the project forgelet-bridge"
	if len(sent) != 1 || sent[0].body != want {
		t.Fatalf("sent = %+v, want %q", sent, want)
	}
}

func TestTickStaysQuietWhileTheBoardDoesNotChange(t *testing.T) {
	board := &fakeBoard{}
	board.set(boardCard("specifier"))
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithStores(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": &fakeApprovals{}},
		map[string]BoardStore{"/forges/forge-a": board}, "/forges/forge-a")
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick: %v", err)
	}
	rooms.sent = nil

	for tick := 0; tick < 5; tick++ {
		if err := built.Tick(context.Background()); err != nil {
			t.Fatalf("tick %d: %v", tick, err)
		}
	}

	if sent := rooms.sentMessages(); len(sent) != 0 {
		t.Errorf("sent = %+v, want no updates at all", sent)
	}
}

func TestTickDoesNotRepeatUpdatesAfterARestart(t *testing.T) {
	board := &fakeBoard{}
	board.set(boardCard("specifier"))
	rooms := &fakeRooms{}
	built, cfg := newTestBridgeWithStores(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": &fakeApprovals{}},
		map[string]BoardStore{"/forges/forge-a": board}, "/forges/forge-a")
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick: %v", err)
	}
	first := rooms.sentMessages()
	if len(first) != 1 || !strings.Contains(first[0].body, "appeared") {
		t.Fatalf("first update = %+v, want the card appearing", first)
	}

	restartedRooms := &fakeRooms{}
	restarted, err := New(cfg, restartedRooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": &fakeApprovals{}},
		map[string]ClarificationStore{"/forges/forge-a": &fakeClarifications{}},
		map[string]BoardStore{"/forges/forge-a": board}, nil)
	if err != nil {
		t.Fatalf("New after restart: %v", err)
	}
	if err := restarted.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick: %v", err)
	}
	if err := restarted.Tick(context.Background()); err != nil {
		t.Fatalf("third Tick: %v", err)
	}

	if sent := restartedRooms.sentMessages(); len(sent) != 0 {
		t.Errorf("sent = %+v, want nothing replayed", sent)
	}
}
