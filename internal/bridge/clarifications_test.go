package bridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/unclebob/forgelet-bridge/internal/relay"
	"github.com/unclebob/forgelet-bridge/internal/state"
)

type answer struct {
	project string
	id      string
	text    string
}

type fakeClarificationSnapshot struct {
	store *fakeClarifications
	root  string
}

func (f fakeClarificationSnapshot) Identity() (string, error) {
	return f.root + "/clarification-snapshot", nil
}

func (f fakeClarificationSnapshot) Snapshot(ctx context.Context) (relay.DashboardSnapshot, error) {
	pending, err := f.store.Pending(ctx)
	if err != nil {
		return relay.DashboardSnapshot{}, err
	}
	return relay.DashboardSnapshot{TypedClarifications: pending}, nil
}

// fakeClarifications is the typed answer boundary. The pending and legacy
// answer fields remain so older focused tests can construct the same fixture
// without becoming a second bridge authority.
type fakeClarifications struct {
	mu         sync.Mutex
	pending    []relay.Clarification
	answers    []answer
	typed      []answer
	errors     map[string]error
	pendingErr error
}

func (f *fakeClarifications) Pending(_ context.Context) ([]relay.Clarification, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.pendingErr != nil {
		return nil, f.pendingErr
	}
	return append([]relay.Clarification(nil), f.pending...), nil
}

func (f *fakeClarifications) Answer(_ context.Context, project, id, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.answers = append(f.answers, answer{project: project, id: id, text: text})
	return nil
}

func (f *fakeClarifications) AnswerTyped(_ context.Context, project, id, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := project + "/" + id
	if err := f.errors[key]; err != nil {
		return err
	}
	f.typed = append(f.typed, answer{project: project, id: id, text: text})
	return nil
}

func (f *fakeClarifications) answered() []answer {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]answer(nil), f.answers...)
}

func (f *fakeClarifications) typedAnswers() []answer {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]answer(nil), f.typed...)
}

func clarificationOf(project, id, role, question string) relay.Clarification {
	return relay.Clarification{
		Key:       "/forges/forge-a/typed/" + project + "/" + id,
		ID:        id,
		Project:   project,
		Role:      role,
		Question:  question,
		Source:    relay.ClarificationSourceTyped,
		Lifecycle: relay.ClarificationOpen,
		Attention: "unseen",
	}
}

func typedClarificationOf(project, id, role, question, lifecycle, attention string) relay.Clarification {
	clarification := clarificationOf(project, id, role, question)
	clarification.Lifecycle = lifecycle
	clarification.Attention = attention
	return clarification
}

func clarificationRoom() Room {
	return Room{ClarificationsRoomID: "!clarifications-forge-a"}
}

func TestTypedOpenUnseenClarificationPostsOnceWithTheBoundedQuestion(t *testing.T) {
	root := "/forges/forge-a"
	rooms := &fakeRooms{}
	store := &fakeClarifications{}
	built, _ := newTestBridgeWithClarifications(t, rooms,
		map[string]ForgeStore{root: &fakeStore{}},
		map[string]ClarificationStore{root: store}, root)
	typed := typedClarificationOf(
		"project-alpha", "clar-1", "coder", "which lane?", relay.ClarificationOpen, "unseen")

	for tick := 0; tick < 2; tick++ {
		if _, err := built.carryOutClarifications(
			context.Background(), root, store, clarificationRoom(), nil,
			relay.DashboardSnapshot{TypedClarifications: []relay.Clarification{typed}}); err != nil {
			t.Fatalf("tick %d: %v", tick, err)
		}
	}

	messages := rooms.sentMessages()
	if len(messages) != 1 || messages[0].roomID != clarificationRoom().ClarificationsRoomID {
		t.Fatalf("messages = %+v, want one typed notice", messages)
	}
	for _, want := range []string{"project-alpha", "coder", "which lane?"} {
		if !strings.Contains(messages[0].body, want) {
			t.Errorf("message %q does not name %q", messages[0].body, want)
		}
	}
	remembered := built.State().Relay.Clarifications[typed.Key]
	if remembered.Source != relay.ClarificationSourceTyped || remembered.MessageID == "" {
		t.Fatalf("state = %+v, want the typed notification record", remembered)
	}
}

func TestOpenSeenClarificationCreatesNoNoticeButCarriesItsFact(t *testing.T) {
	root := "/forges/forge-a"
	rooms := &fakeRooms{}
	store := &fakeClarifications{}
	built, _ := newTestBridgeWithClarifications(t, rooms,
		map[string]ForgeStore{root: &fakeStore{}},
		map[string]ClarificationStore{root: store}, root)
	typed := typedClarificationOf(
		"project-alpha", "clar-seen", "coder", "which stage?", relay.ClarificationOpen, "seen")

	if _, err := built.carryOutClarifications(
		context.Background(), root, store, clarificationRoom(), nil,
		relay.DashboardSnapshot{TypedClarifications: []relay.Clarification{typed}}); err != nil {
		t.Fatal(err)
	}
	if messages := rooms.sentMessages(); len(messages) != 0 {
		t.Fatalf("messages = %+v, want no notice for seen work", messages)
	}
	if states := rooms.sentStates(); len(states) != 1 || !strings.Contains(states[0].stateKey, "project-alpha") {
		t.Fatalf("states = %+v, want the open seen fact", states)
	}
}

func TestAnsweredAndWithdrawnRowsClearOnlyTheirProjectQualifiedRecord(t *testing.T) {
	for _, lifecycle := range []string{"answered", "withdrawn"} {
		t.Run(lifecycle, func(t *testing.T) {
			root := "/forges/forge-a"
			rooms := &fakeRooms{}
			store := &fakeClarifications{}
			built, _ := newTestBridgeWithClarifications(t, rooms,
				map[string]ForgeStore{root: &fakeStore{}},
				map[string]ClarificationStore{root: store}, root)
			open := typedClarificationOf(
				"project-alpha", "clar-ending", "coder", "which stage?", relay.ClarificationOpen, "unseen")
			other := typedClarificationOf(
				"project-beta", "clar-other", "coder", "other?", relay.ClarificationOpen, "seen")
			room := clarificationRoom()
			if _, err := built.carryOutClarifications(
				context.Background(), root, store, room, nil,
				relay.DashboardSnapshot{TypedClarifications: []relay.Clarification{open, other}}); err != nil {
				t.Fatal(err)
			}
			ended := open
			ended.Lifecycle = lifecycle
			ended.Attention = "seen"
			if _, err := built.carryOutClarifications(
				context.Background(), root, store, room, nil,
				relay.DashboardSnapshot{TypedClarifications: []relay.Clarification{ended, other}}); err != nil {
				t.Fatal(err)
			}
			if state := built.State().Relay.Clarifications[open.Key]; state.MessageID != "" {
				t.Fatalf("ended state = %+v, want its record cleared", state)
			}
			if state := built.State().Relay.Clarifications[other.Key]; state.MessageID != "" {
				t.Fatalf("other state = %+v, want the open sibling untouched", state)
			}
		})
	}
}

func TestAbsentAndObsoleteLegacyRecordsAreReconciledWithoutTouchingOpenTyped(t *testing.T) {
	root := "/forges/forge-a"
	rooms := &fakeRooms{}
	store := &fakeClarifications{}
	built, _ := newTestBridgeWithClarifications(t, rooms,
		map[string]ForgeStore{root: &fakeStore{}},
		map[string]ClarificationStore{root: store}, root)
	open := typedClarificationOf(
		"project-alpha", "clar-open", "coder", "which stage?", relay.ClarificationOpen, "unseen")
	legacy := typedClarificationOf(
		"project-legacy", "clar-old", "coder", "old?", relay.ClarificationOpen, "unseen")
	legacy.Source = ""
	room := clarificationRoom()
	built.State().Relay.Clarifications[legacy.Key] = relay.ClarificationState{
		ForgeRoot: root, RoomID: room.ClarificationsRoomID, MessageID: "$legacy",
	}
	if _, err := built.carryOutClarifications(
		context.Background(), root, store, room, nil,
		relay.DashboardSnapshot{TypedClarifications: []relay.Clarification{open}}); err != nil {
		t.Fatal(err)
	}
	if _, exists := built.State().Relay.Clarifications[legacy.Key]; exists {
		t.Fatal("obsolete legacy record remains")
	}
	if state := built.State().Relay.Clarifications[open.Key]; state.MessageID == "" {
		t.Fatal("the open typed record was removed")
	}
	if _, err := built.carryOutClarifications(
		context.Background(), root, store, room, nil, relay.DashboardSnapshot{}); err != nil {
		t.Fatal(err)
	}
	if _, exists := built.State().Relay.Clarifications[open.Key]; exists {
		t.Fatal("absent typed record was not cleared")
	}
	reloaded, err := state.Load(built.statePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := reloaded.Relay.Clarifications[open.Key]; exists {
		t.Fatal("cleared typed record was not persisted")
	}
}

func TestTypedReplyUsesOnlyTheProjectChatAnswererAndMarksTheNoticeOnce(t *testing.T) {
	root := "/forges/forge-a"
	rooms := &fakeRooms{}
	store := &fakeClarifications{}
	built, _ := newTestBridgeWithClarifications(t, rooms,
		map[string]ForgeStore{root: &fakeStore{}},
		map[string]ClarificationStore{root: store}, root)
	typed := typedClarificationOf(
		"project-alpha", "clar-answer", "coder", "which stage?", relay.ClarificationOpen, "unseen")
	room := clarificationRoom()
	snapshot := relay.DashboardSnapshot{TypedClarifications: []relay.Clarification{typed}}
	if _, err := built.carryOutClarifications(context.Background(), root, store, room, nil, snapshot); err != nil {
		t.Fatal(err)
	}
	messageID := built.State().Relay.Clarifications[typed.Key].MessageID
	reply := relay.RoomEvent{
		EventID: "$reply", RoomID: room.ClarificationsRoomID, Sender: operator,
		Body: "run qa", ThreadRoot: messageID,
	}
	for tick := 0; tick < 2; tick++ {
		if _, err := built.carryOutClarifications(
			context.Background(), root, store, room, []relay.RoomEvent{reply}, snapshot); err != nil {
			t.Fatal(err)
		}
	}

	answers := store.typedAnswers()
	if len(answers) != 1 || answers[0] != (answer{project: "project-alpha", id: "clar-answer", text: "run qa"}) {
		t.Fatalf("typed answers = %+v, want exactly one project-chat answer", answers)
	}
	if legacy := store.answered(); len(legacy) != 0 {
		t.Fatalf("legacy answers = %+v, want none", legacy)
	}
	marks := rooms.sentReactions()
	if len(marks) != 1 || marks[0].target != messageID || marks[0].key != relay.CarriedOutReaction {
		t.Fatalf("reactions = %+v, want one carried-out mark", marks)
	}
}

func TestReplyToTheClarificationStateEventUsesProjectChat(t *testing.T) {
	root := "/forges/forge-a"
	rooms := &fakeRooms{}
	store := &fakeClarifications{}
	built, _ := newTestBridgeWithClarifications(t, rooms,
		map[string]ForgeStore{root: &fakeStore{}},
		map[string]ClarificationStore{root: store}, root)
	typed := typedClarificationOf(
		"project-alpha", "clar-event", "coder", "which stage?", relay.ClarificationOpen, "unseen")
	room := clarificationRoom()
	snapshot := relay.DashboardSnapshot{TypedClarifications: []relay.Clarification{typed}}
	if _, err := built.carryOutClarifications(context.Background(), root, store, room, nil, snapshot); err != nil {
		t.Fatal(err)
	}
	eventID := built.State().Relay.Clarifications[typed.Key].StateEventID
	reply := relay.RoomEvent{
		EventID: "$reply", RoomID: room.ClarificationsRoomID, Sender: operator,
		Body: "run qa", ReplyTo: eventID,
	}
	if _, err := built.carryOutClarifications(
		context.Background(), root, store, room, []relay.RoomEvent{reply}, snapshot); err != nil {
		t.Fatal(err)
	}
	if answers := store.typedAnswers(); len(answers) != 1 || answers[0].text != "run qa" {
		t.Fatalf("typed answers = %+v, want the event reply carried back", answers)
	}
}

func TestTypedAnswerFailureLeavesNoAcceptedRecord(t *testing.T) {
	root := "/forges/forge-a"
	store := &fakeClarifications{errors: map[string]error{
		"project-alpha/clar-failed": fmt.Errorf("project chat unavailable"),
	}}
	built, _ := newTestBridgeWithClarifications(t, &fakeRooms{},
		map[string]ForgeStore{root: &fakeStore{}},
		map[string]ClarificationStore{root: store}, root)
	typed := typedClarificationOf(
		"project-alpha", "clar-failed", "coder", "which stage?", relay.ClarificationOpen, "seen")
	built.State().Relay.Clarifications[typed.Key] = relay.ClarificationState{
		ForgeRoot: root, RoomID: clarificationRoom().ClarificationsRoomID, MessageID: "$message",
	}
	err := built.answerClarification(context.Background(), root, store, clarificationRoom(), relay.ClarificationAction{
		Kind: relay.AnswerClarification, Key: typed.Key, Clarification: typed, Answer: "run qa",
	})
	if err == nil || !strings.Contains(err.Error(), "project chat") {
		t.Fatalf("answer error = %v, want a failed project-chat answer", err)
	}
	if state := built.State().Relay.Clarifications[typed.Key]; state.Answer != "" {
		t.Fatalf("state = %+v, want no accepted answer", state)
	}
}

func TestClarificationWorkFailsVisiblyWithoutAnAnswerer(t *testing.T) {
	root := "/forges/forge-a"
	built, _ := newTestBridgeWithClarifications(t, &fakeRooms{},
		map[string]ForgeStore{root: &fakeStore{}},
		map[string]ClarificationStore{}, root)
	carriedOut, err := built.carryOutClarifications(
		context.Background(), root, nil, clarificationRoom(), nil, relay.DashboardSnapshot{})
	if err == nil || !strings.Contains(err.Error(), "no typed clarification answerer") {
		t.Fatalf("error = %v, want the missing answerer", err)
	}
	if carriedOut != 0 {
		t.Fatalf("carried out = %d, want none without an answerer", carriedOut)
	}
}

func TestTickReportsAMissingAutomationSnapshotToTheClarificationConsumer(t *testing.T) {
	root := "/forges/forge-a"
	built, _ := newTestBridge(t, &fakeRooms{},
		map[string]ForgeStore{root: &fakeStore{}}, root)
	if err := built.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	status := statusOf(t, built)
	if !reflect.DeepEqual(status.ClarificationFailures, []string{"forge-a"}) {
		t.Fatalf("clarification failures = %v, want forge-a", status.ClarificationFailures)
	}
	if !reflect.DeepEqual(status.ReachedForges, []string{"forge-a"}) {
		t.Fatalf("reached forges = %v, want unrelated consumers to continue", status.ReachedForges)
	}
}

func TestUnknownClarificationActionsAreRefused(t *testing.T) {
	root := "/forges/forge-a"
	store := &fakeClarifications{}
	built, _ := newTestBridgeWithClarifications(t, &fakeRooms{},
		map[string]ForgeStore{root: &fakeStore{}},
		map[string]ClarificationStore{root: store}, root)
	if err := built.carryOutClarification(
		context.Background(), root, store, clarificationRoom(),
		relay.ClarificationAction{Kind: "unknown"}); err == nil {
		t.Fatal("unknown action was accepted")
	}
}

func TestClarificationInputsKeepOnlyOpenRows(t *testing.T) {
	open := typedClarificationOf(
		"project", "open", "coder", "open?", relay.ClarificationOpen, "seen")
	answered := typedClarificationOf(
		"project", "answered", "coder", "answered?", "answered", "seen")
	withdrawn := typedClarificationOf(
		"project", "withdrawn", "coder", "withdrawn?", "withdrawn", "seen")
	got := clarificationInputs(relay.DashboardSnapshot{
		TypedClarifications: []relay.Clarification{open, answered, withdrawn},
	})
	if !reflect.DeepEqual(got, []relay.Clarification{open}) {
		t.Fatalf("inputs = %+v, want only the open row", got)
	}
}

func TestReconciliationDoesNotSaveWhenNothingIsStale(t *testing.T) {
	root := "/forges/forge-a"
	built, _ := newTestBridgeWithClarifications(t, &fakeRooms{},
		map[string]ForgeStore{root: &fakeStore{}},
		map[string]ClarificationStore{root: &fakeClarifications{}}, root)
	notDirectory := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(notDirectory, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	built.statePath = filepath.Join(notDirectory, "bridge-state.json")
	open := typedClarificationOf(
		"project-alpha", "clar-open", "coder", "which stage?", relay.ClarificationOpen, "seen")
	built.State().Relay.Clarifications[open.Key] = relay.ClarificationState{
		ForgeRoot: root, RoomID: clarificationRoom().ClarificationsRoomID, MessageID: "$message",
	}
	if err := built.reconcileClarifications(
		root, clarificationRoom(), []relay.Clarification{open}); err != nil {
		t.Fatalf("unchanged snapshot tried to save state: %v", err)
	}
}

func TestStaleClarificationRequiresTheSameOwnerRoomAndNoOpenRow(t *testing.T) {
	room := clarificationRoom()
	tests := []struct {
		name  string
		state relay.ClarificationState
		open  bool
		want  bool
	}{
		{name: "same room closed", state: relay.ClarificationState{
			ForgeRoot: "/forge", RoomID: room.ClarificationsRoomID,
		}, want: true},
		{name: "unscoped room closed", state: relay.ClarificationState{
			ForgeRoot: "/forge",
		}, want: true},
		{name: "same room open", state: relay.ClarificationState{
			ForgeRoot: "/forge", RoomID: room.ClarificationsRoomID,
		}, open: true},
		{name: "other room", state: relay.ClarificationState{
			ForgeRoot: "/forge", RoomID: "!other",
		}},
		{name: "other owner", state: relay.ClarificationState{
			ForgeRoot: "/other", RoomID: room.ClarificationsRoomID,
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := staleClarification(test.state, "/forge", room, test.open); got != test.want {
				t.Fatalf("stale = %t, want %t", got, test.want)
			}
		})
	}
}

func lastMessageIn(rooms *fakeRooms, roomID string) *sentMessage {
	sent := rooms.sentMessages()
	for index := len(sent) - 1; index >= 0; index-- {
		if sent[index].roomID == roomID {
			return &sent[index]
		}
	}
	return nil
}
