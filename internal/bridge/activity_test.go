package bridge

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/unclebob/forgelet-bridge/internal/relay"
	"github.com/unclebob/forgelet-bridge/internal/state"
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

// finishedCards is a board of cards that finished before the bridge got to
// them, which is what a forge arriving with a history looks like.
func finishedCards(count int) []relay.Card {
	cards := make([]relay.Card, 0, count)
	for index := 0; index < count; index++ {
		name := fmt.Sprintf("finished-card-%d", index)
		cards = append(cards, relay.Card{
			Key:     "forgelet-bridge/" + name,
			Project: "forgelet-bridge",
			Name:    name,
			Lane:    "done",
			Done:    true,
		})
	}
	return cards
}

// A forge that arrives with a board full of finished cards says nothing about
// them, and says nothing about them on the next tick either: the cards that
// finished before the bridge got there are history, and a first appearance must
// not produce a burst the size of the board it arrives with.
func TestTickDoesNotReplayTheBoardAForgeArrivesWith(t *testing.T) {
	board := &fakeBoard{}
	board.set(finishedCards(40)...)
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithStores(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": &fakeApprovals{}},
		map[string]BoardStore{"/forges/forge-a": board}, "/forges/forge-a")

	for tick := 0; tick < 2; tick++ {
		if err := built.Tick(context.Background()); err != nil {
			t.Fatalf("Tick %d: %v", tick+1, err)
		}
	}

	if sent := rooms.sentMessages(); len(sent) != 0 {
		t.Errorf("sent = %d messages, want the history the forge arrived with left unsaid", len(sent))
	}
	if _, known := built.State().Relay.Activity["forgelet-bridge/finished-card-0"]; !known {
		t.Errorf("the cards the forge arrived with were not remembered, so a later tick would narrate them")
	}
}

// The news notifies and the routine step does not: a card appearing and a card
// finishing go as ordinary messages, and a lane-to-lane move goes as a notice.
func TestTickSendsTheNewsAsMessagesAndTheRoutineStepAsANotice(t *testing.T) {
	board := &fakeBoard{}
	board.set(boardCard("specifier"))
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithStores(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": &fakeApprovals{}},
		map[string]BoardStore{"/forges/forge-a": board}, "/forges/forge-a")

	for _, lane := range []string{"specifier", "coder", "done"} {
		board.set(boardCard(lane))
		if err := built.Tick(context.Background()); err != nil {
			t.Fatalf("Tick for lane %s: %v", lane, err)
		}
	}

	wantNotice := map[string]bool{
		"card card-activity-feed appeared in the project forgelet-bridge in the lane specifier": false,
		"card card-activity-feed moved on in the project forgelet-bridge to the lane coder":     true,
		"card card-activity-feed finished in the project forgelet-bridge":                       false,
	}
	seen := map[string]bool{}
	for _, sent := range rooms.sentMessages() {
		want, tracked := wantNotice[sent.body]
		if !tracked {
			continue
		}
		seen[sent.body] = true
		if sent.notice != want {
			t.Errorf("%q was sent with notice=%v, want %v", sent.body, sent.notice, want)
		}
	}
	for body := range wantNotice {
		if !seen[body] {
			t.Errorf("the room never heard %q", body)
		}
	}
}

// The status reports what the bridge still owes each forge, so a forge that was
// reached behind a queue reads as reached with a backlog rather than as a
// failure to reach it.
func TestStatusReportsTheWorkEachForgeStillOwes(t *testing.T) {
	rooms := &fakeRooms{}
	chat := &fakeStore{requests: []relay.Request{{ID: "req-1", Body: "is the build green?"}}}
	built, cfg := newTestBridgeWithStores(t, rooms, map[string]ForgeStore{"/forges/forge-a": chat},
		map[string]ApprovalStore{"/forges/forge-a": &fakeApprovals{}},
		map[string]BoardStore{"/forges/forge-a": &fakeBoard{}}, "/forges/forge-a")

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	status := readStatus(t, filepath.Join(cfg.StateDir, StatusName))
	if len(status.Owed) != 1 || status.Owed[0].Name != "forge-a" {
		t.Fatalf("owed = %+v, want the configured forge named", status.Owed)
	}
	if status.Owed[0].Items != 0 {
		t.Errorf("owed items = %d, want nothing owed once the tick has carried the queue out", status.Owed[0].Items)
	}
}

func TestTickRemembersASingleCardTheForgeArrivedWith(t *testing.T) {
	board := &fakeBoard{}
	board.set(finishedCards(1)...)
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithStores(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": &fakeApprovals{}},
		map[string]BoardStore{"/forges/forge-a": board}, "/forges/forge-a")

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	// The state on disk, not the one in memory: a card remembered only until
	// the process ends would be narrated the moment the bridge came back.
	reloaded, err := state.Load(built.statePath)
	if err != nil {
		t.Fatalf("Load state: %v", err)
	}
	if _, known := reloaded.Relay.Activity["forgelet-bridge/finished-card-0"]; !known {
		t.Errorf("the one card the forge arrived with was not saved, so a restart would narrate it")
	}
}

func TestTickAnnouncesAFinishAfterAQuietTick(t *testing.T) {
	board := &fakeBoard{}
	board.set(boardCard("specifier"))
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithStores(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": &fakeApprovals{}},
		map[string]BoardStore{"/forges/forge-a": board}, "/forges/forge-a")
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick: %v", err)
	}
	before := len(rooms.sentMessages())

	// A tick where the card has not moved says nothing, and must change nothing:
	// the card is in flight, so its finish is still news.
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick: %v", err)
	}
	board.set(boardCard("done"))
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("third Tick: %v", err)
	}

	sent := rooms.sentMessages()[before:]
	if len(sent) != 1 || !strings.Contains(sent[0].body, "finished") {
		t.Errorf("sent = %+v, want the card's finish announced after the quiet tick", sent)
	}
}

func TestOwedForCountsEveryKindOfWork(t *testing.T) {
	built, _ := newTestBridge(t, &fakeRooms{}, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}}, "/forges/forge-a")

	built.pendingFor("/forges/forge-a").keep(relay.Action{Kind: relay.CreateForgeRequest, RequestID: "req-1"})
	for _, key := range []string{"a-1", "a-2"} {
		built.pendingApprovalsFor("/forges/forge-a").work.keep(relay.ApprovalAction{Kind: relay.ResolveApproval, Key: key})
	}
	for _, key := range []string{"c-1", "c-2", "c-3"} {
		built.pendingClarificationsFor("/forges/forge-a").keep(relay.ClarificationAction{Kind: relay.AnswerClarification, Key: key})
	}

	if got := built.owedFor("/forges/forge-a"); got != 6 {
		t.Errorf("owedFor = %d, want every kind counted: 1 chat, 2 approvals, 3 clarifications", got)
	}
}
