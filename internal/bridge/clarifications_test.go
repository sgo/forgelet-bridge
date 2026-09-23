package bridge

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/unclebob/forgelet-bridge/internal/relay"
	"github.com/unclebob/forgelet-bridge/internal/state"
)

// fakeClarifications is a forge's clarifications for the bridge tests: the
// questions its agents are blocked on, and the answers the bridge carried back.
type fakeClarifications struct {
	mu      sync.Mutex
	pending []relay.Clarification
	answers []answer
	errors  map[string]error
}

type answer struct {
	project string
	id      string
	text    string
}

func (f *fakeClarifications) Pending(_ context.Context) ([]relay.Clarification, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]relay.Clarification(nil), f.pending...), nil
}

func (f *fakeClarifications) Answer(_ context.Context, project, id, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := project + "/" + id
	if err := f.errors[key]; err != nil {
		return err
	}
	f.answers = append(f.answers, answer{project: project, id: id, text: text})
	kept := f.pending[:0]
	for _, clarification := range f.pending {
		if clarification.Key != key {
			kept = append(kept, clarification)
		}
	}
	f.pending = kept
	return nil
}

func (f *fakeClarifications) answered() []answer {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]answer(nil), f.answers...)
}

func clarificationOf(project, id, role, question string) relay.Clarification {
	return relay.Clarification{
		Key:      project + "/" + id,
		ID:       id,
		Project:  project,
		Role:     role,
		Question: question,
	}
}

func TestTickPostsAPendingClarificationWithWhatItTakesToAnswer(t *testing.T) {
	store := &fakeClarifications{pending: []relay.Clarification{
		clarificationOf("forgelet-bridge", "clar-1", "coder", "which lane should the refund card start in?"),
	}}
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithClarifications(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ClarificationStore{"/forges/forge-a": store}, "/forges/forge-a")

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	message := lastMessageIn(rooms, "!clarifications-forge-a")
	if message == nil {
		t.Fatalf("sent = %+v, want the clarification posted into its room", rooms.sentMessages())
	}
	for _, want := range []string{"forgelet-bridge", "coder", "which lane should the refund card start in?", "answer"} {
		if !strings.Contains(message.body, want) {
			t.Errorf("clarification message %q does not name %q", message.body, want)
		}
	}
}

func TestTickCarriesTheOperatorsReplyBackAsTheAnswer(t *testing.T) {
	store := &fakeClarifications{pending: []relay.Clarification{
		clarificationOf("forgelet-bridge", "clar-1", "coder", "which lane?"),
	}}
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithClarifications(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ClarificationStore{"/forges/forge-a": store}, "/forges/forge-a")
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick: %v", err)
	}

	rooms.push(relay.RoomEvent{EventID: "$reply", RoomID: "!clarifications-forge-a", Sender: operator, Body: "yes", ThreadRoot: "$event-" + clarificationMessage(clarificationOf("forgelet-bridge", "clar-1", "coder", "which lane?"))})
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick: %v", err)
	}
	// The forge has the answer now, so the next tick reports it in the thread.
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("third Tick: %v", err)
	}

	answers := store.answered()
	if len(answers) != 1 || answers[0].text != "yes" || answers[0].id != "clar-1" {
		t.Fatalf("answers = %+v, want the operator's reply carried back", answers)
	}
	if reply := lastMessageIn(rooms, "!clarifications-forge-a"); reply == nil || reply.body != "Answered" {
		t.Errorf("sent = %+v, want the answer reported in the thread", rooms.sentMessages())
	}
}

func TestRestartRepeatsNoClarificationWork(t *testing.T) {
	store := &fakeClarifications{pending: []relay.Clarification{
		clarificationOf("forgelet-bridge", "clar-1", "coder", "which lane?"),
	}}
	rooms := &fakeRooms{}
	built, cfg := newTestBridgeWithClarifications(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ClarificationStore{"/forges/forge-a": store}, "/forges/forge-a")
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick: %v", err)
	}
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick: %v", err)
	}
	posted := len(rooms.sentMessages())

	restartedRooms := &fakeRooms{}
	restarted, err := New(cfg, restartedRooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": &fakeApprovals{}},
		map[string]ClarificationStore{"/forges/forge-a": store},
		map[string]BoardStore{"/forges/forge-a": &fakeBoard{}}, nil)
	if err != nil {
		t.Fatalf("New after restart: %v", err)
	}
	if err := restarted.Tick(context.Background()); err != nil {
		t.Fatalf("Tick after restart: %v", err)
	}

	if sent := restartedRooms.sentMessages(); len(sent) != 0 {
		t.Errorf("sent after restart = %+v, want nothing repeated", sent)
	}
	if posted == 0 {
		t.Fatal("the bridge never posted the clarification in the first place")
	}
}

func TestTickReportsAClarificationAnsweredOnTheDesktop(t *testing.T) {
	clarification := clarificationOf("forgelet-bridge", "clar-1", "coder", "which lane?")
	store := &fakeClarifications{pending: []relay.Clarification{clarification}}
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithClarifications(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ClarificationStore{"/forges/forge-a": store}, "/forges/forge-a")
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick: %v", err)
	}

	// The desktop answers it: the dashboard no longer lists it as pending.
	store.mu.Lock()
	store.pending = nil
	store.mu.Unlock()
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick: %v", err)
	}

	if reply := lastMessageIn(rooms, "!clarifications-forge-a"); reply == nil || reply.body != "Resolved on the desktop" {
		t.Errorf("sent = %+v, want the desktop's answer reported", rooms.sentMessages())
	}
	reloaded, err := state.Load(built.statePath)
	if err != nil {
		t.Fatalf("Load state: %v", err)
	}
	answer := reloaded.Relay.Clarifications[clarification.Key]
	if answer.ReplyID == "" {
		t.Errorf("state = %+v, want the report remembered so it is not repeated", answer)
	}
}

func TestTickKeepsAnAnswerTheForgeRefusedAndTriesItAgain(t *testing.T) {
	clarification := clarificationOf("forgelet-bridge", "clar-1", "coder", "which lane?")
	store := &fakeClarifications{
		pending: []relay.Clarification{clarification},
		errors:  map[string]error{"forgelet-bridge/clar-1": fmt.Errorf("the forge is not there")},
	}
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithClarifications(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ClarificationStore{"/forges/forge-a": store}, "/forges/forge-a")
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick: %v", err)
	}
	rooms.push(relay.RoomEvent{EventID: "$reply", RoomID: "!clarifications-forge-a", Sender: operator, Body: "yes", ThreadRoot: "$event-" + clarificationMessage(clarification)})
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick: %v", err)
	}
	if len(store.answered()) != 0 {
		t.Fatalf("answers = %+v, want none while the forge refuses", store.answered())
	}

	store.mu.Lock()
	store.errors = nil
	store.mu.Unlock()
	// The reply is not in the room any more, but the answer the bridge kept is.
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("third Tick: %v", err)
	}
	answers := store.answered()
	if len(answers) != 1 || answers[0].text != "yes" {
		t.Errorf("answers = %+v, want the refused answer tried again", answers)
	}
}

// lastMessageIn is the last message the bridge sent into a room, nil when it
// sent none.
func lastMessageIn(rooms *fakeRooms, roomID string) *sentMessage {
	sent := rooms.sentMessages()
	for index := len(sent) - 1; index >= 0; index-- {
		if sent[index].roomID == roomID {
			return &sent[index]
		}
	}
	return nil
}

func TestTickReportsAForgeWithNoClarificationsStore(t *testing.T) {
	// A forge the bridge has no clarifications for is reported, not a tick that
	// takes the rest of the bridge down with it.
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithClarifications(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ClarificationStore{}, "/forges/forge-a")

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	status := statusOf(t, built)
	if len(status.UnhappyForges) != 1 || status.UnhappyForges[0] != "forge-a" {
		t.Errorf("unhappy forges = %v, want the forge without clarifications named", status.UnhappyForges)
	}
}
