package bridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/unclebob/forgelet-bridge/internal/relay"
	"github.com/unclebob/forgelet-bridge/internal/state"
)

// fakeClarifications is a forge's clarifications for the bridge tests: the
// questions its agents are blocked on, and the answers the bridge carried back.
type fakeClarifications struct {
	mu         sync.Mutex
	pending    []relay.Clarification
	answers    []answer
	typed      []answer
	errors     map[string]error
	pendingErr error
}

type answer struct {
	project string
	id      string
	text    string
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

func (f *fakeClarifications) AnswerTyped(_ context.Context, project, id, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := project + "/" + id
	if err := f.errors[key]; err != nil {
		return err
	}
	f.typed = append(f.typed, answer{project: project, id: id, text: text})
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

func (f *fakeClarifications) typedAnswers() []answer {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]answer(nil), f.typed...)
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

func typedClarificationOf(project, id, role, question, lifecycle, attention string) relay.Clarification {
	clarification := clarificationOf(project, id, role, question)
	clarification.Key = "/forges/forge-a/typed/" + project + "/" + id
	clarification.Source = relay.ClarificationSourceTyped
	clarification.Lifecycle = lifecycle
	clarification.Attention = attention
	clarification.Notify = lifecycle == relay.ClarificationOpen && attention == "unseen"
	return clarification
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

func TestClarificationFactsKeepWorkAlreadyCarriedOutWhenALaterWriteFails(t *testing.T) {
	root := "/forges/forge-a"
	rooms := &fakeRooms{stateErrAfter: 2}
	first := clarificationOf("forgelet-bridge", "clar-1", "coder", "first?")
	second := clarificationOf("forgelet-bridge", "clar-2", "coder", "second?")
	store := &fakeClarifications{pending: []relay.Clarification{first, second}}
	built, _ := newTestBridgeWithClarifications(t, rooms,
		map[string]ForgeStore{root: &fakeStore{}},
		map[string]ClarificationStore{root: store}, root)

	carriedOut, err := built.carryOutClarifications(
		context.Background(), root, store, Room{ClarificationsRoomID: "!clarifications-forge-a"},
		nil, relay.DashboardSnapshot{}, false)
	if err == nil {
		t.Fatal("carryOutClarifications accepted a failed second fact write")
	}
	if carriedOut != 3 {
		t.Errorf("carried out = %d, want two messages and one written fact", carriedOut)
	}
}

func TestUnknownClarificationActionsAreRefused(t *testing.T) {
	root := "/forges/forge-a"
	rooms := &fakeRooms{}
	store := &fakeClarifications{}
	built, _ := newTestBridgeWithClarifications(t, rooms,
		map[string]ForgeStore{root: &fakeStore{}},
		map[string]ClarificationStore{root: store}, root)

	if err := built.carryOutClarification(
		context.Background(), root, store, Room{ClarificationsRoomID: "!clarifications-forge-a"},
		relay.ClarificationAction{Kind: "unknown"}); err == nil {
		t.Error("carryOutClarification accepted an unknown action")
	}
}

func TestClarificationInputFailureReportsNoWork(t *testing.T) {
	root := "/forges/forge-a"
	rooms := &fakeRooms{}
	store := &fakeClarifications{pendingErr: context.DeadlineExceeded}
	built, _ := newTestBridgeWithClarifications(t, rooms,
		map[string]ForgeStore{root: &fakeStore{}},
		map[string]ClarificationStore{root: store}, root)

	carriedOut, err := built.carryOutClarifications(
		context.Background(), root, store, Room{ClarificationsRoomID: "!clarifications-forge-a"},
		nil, relay.DashboardSnapshot{}, false)
	if err == nil {
		t.Fatal("carryOutClarifications accepted unreadable pending clarifications")
	}
	if carriedOut != 0 {
		t.Errorf("carried out = %d, want none when clarification inputs fail", carriedOut)
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
	// The forge has the answer now, so the next tick marks the message.
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("third Tick: %v", err)
	}

	answers := store.answered()
	if len(answers) != 1 || answers[0].text != "yes" || answers[0].id != "clar-1" {
		t.Fatalf("answers = %+v, want the operator's reply carried back", answers)
	}
	messageID := built.State().Relay.Clarifications["forgelet-bridge/clar-1"].MessageID
	if owner := built.State().Relay.Clarifications["forgelet-bridge/clar-1"].ForgeRoot; owner != "/forges/forge-a" {
		t.Errorf("clarification scope owner = %q, want the forge root", owner)
	}
	marks := rooms.sentReactions()
	if len(marks) != 1 || marks[0].roomID != "!clarifications-forge-a" || marks[0].target != messageID || marks[0].key != relay.CarriedOutReaction {
		t.Fatalf("reactions = %+v, want the bridge's mark on the clarification's own message", marks)
	}
	for _, sent := range rooms.sentMessages() {
		if sent.anchor == messageID {
			t.Errorf("sent = %+v, want no reply under the message when the answer is marked", rooms.sentMessages())
		}
	}
}

func TestTickAnswersAReplyThatNamesTheClarificationsStateEvent(t *testing.T) {
	store := &fakeClarifications{pending: []relay.Clarification{
		clarificationOf("forgelet-bridge", "clar-1", "coder", "which lane?"),
	}}
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithClarifications(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ClarificationStore{"/forges/forge-a": store}, "/forges/forge-a")
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick: %v", err)
	}
	stateEventID := built.State().Relay.Clarifications["forgelet-bridge/clar-1"].StateEventID
	if stateEventID == "" {
		t.Fatalf("the bridge did not remember the clarification's state event")
	}
	rooms.push(relay.RoomEvent{
		EventID: "$reply",
		RoomID:  "!clarifications-forge-a",
		Sender:  operator,
		Body:    "> Clarification for forgelet-bridge from coder\n\nyes",
		ReplyTo: stateEventID,
	})

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick: %v", err)
	}

	answers := store.answered()
	if len(answers) != 1 || answers[0].id != "clar-1" || answers[0].text != "yes" {
		t.Fatalf("answers = %+v, want the reply on the state event carried back", answers)
	}
}

func TestTickMarksAnAnswerTheOperatorGaveOnlyOnce(t *testing.T) {
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
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("third Tick: %v", err)
	}
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("fourth Tick: %v", err)
	}

	if marks := rooms.sentReactions(); len(marks) != 1 {
		t.Errorf("reactions = %+v, want the clarification marked exactly once", marks)
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

func TestTypedUnseenClarificationPostsOnceAndIsRememberedAsTyped(t *testing.T) {
	root := "/forges/forge-a"
	rooms := &fakeRooms{}
	store := &fakeClarifications{}
	built, _ := newTestBridgeWithClarifications(t, rooms,
		map[string]ForgeStore{root: &fakeStore{}},
		map[string]ClarificationStore{root: store}, root)
	typed := typedClarificationOf(
		"project-alpha", "clar-typed", "coder", "which stage?", relay.ClarificationOpen, "unseen")
	snapshot := relay.DashboardSnapshot{TypedClarifications: []relay.Clarification{typed}}

	for tick := 0; tick < 2; tick++ {
		if _, err := built.carryOutClarifications(
			context.Background(), root, store, Room{ClarificationsRoomID: "!clarifications-forge-a"},
			nil, snapshot, true); err != nil {
			t.Fatalf("tick %d: %v", tick, err)
		}
	}

	messages := rooms.sentMessages()
	if len(messages) != 1 || messages[0].roomID != "!clarifications-forge-a" {
		t.Fatalf("messages = %+v, want one typed notice in the clarifications room", messages)
	}
	remembered := built.State().Relay.Clarifications[typed.Key]
	if remembered.Source != relay.ClarificationSourceTyped || remembered.MessageID == "" {
		t.Fatalf("state = %+v, want the typed notification record", remembered)
	}
}

func TestTypedSeenClarificationDoesNotPostANewNotice(t *testing.T) {
	root := "/forges/forge-a"
	rooms := &fakeRooms{}
	store := &fakeClarifications{}
	built, _ := newTestBridgeWithClarifications(t, rooms,
		map[string]ForgeStore{root: &fakeStore{}},
		map[string]ClarificationStore{root: store}, root)
	typed := typedClarificationOf(
		"project-alpha", "clar-seen", "coder", "which stage?", relay.ClarificationOpen, "seen")

	if _, err := built.carryOutClarifications(
		context.Background(), root, store, Room{ClarificationsRoomID: "!clarifications-forge-a"},
		nil, relay.DashboardSnapshot{TypedClarifications: []relay.Clarification{typed}}, true); err != nil {
		t.Fatal(err)
	}
	if messages := rooms.sentMessages(); len(messages) != 0 {
		t.Fatalf("messages = %+v, want no notice for a seen clarification", messages)
	}
}

func TestLegacyInputPathDoesNotReconcileTypedSnapshotState(t *testing.T) {
	root := "/forges/forge-a"
	rooms := &fakeRooms{}
	store := &fakeClarifications{}
	built, _ := newTestBridgeWithClarifications(t, rooms,
		map[string]ForgeStore{root: &fakeStore{}},
		map[string]ClarificationStore{root: store}, root)
	typed := typedClarificationOf(
		"project-alpha", "clar-kept", "coder", "which stage?", relay.ClarificationOpen, "unseen")
	built.State().Relay.Clarifications[typed.Key] = relay.ClarificationState{
		ForgeRoot: root, RoomID: "!clarifications-forge-a", MessageID: "$message",
		Source: relay.ClarificationSourceTyped,
	}

	if _, err := built.clarificationInputsForTick(
		context.Background(), root, store, Room{ClarificationsRoomID: "!clarifications-forge-a"},
		relay.DashboardSnapshot{}, false); err != nil {
		t.Fatal(err)
	}
	if state := built.State().Relay.Clarifications[typed.Key]; state.MessageID == "" {
		t.Fatal("the independent-store path reconciled snapshot-only state")
	}
}

func TestTypedReconciliationDoesNotSaveWhenNothingIsStale(t *testing.T) {
	root := "/forges/forge-a"
	rooms := &fakeRooms{}
	store := &fakeClarifications{}
	built, _ := newTestBridgeWithClarifications(t, rooms,
		map[string]ForgeStore{root: &fakeStore{}},
		map[string]ClarificationStore{root: store}, root)
	notDirectory := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(notDirectory, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	built.statePath = filepath.Join(notDirectory, "bridge-state.json")
	open := typedClarificationOf(
		"project-alpha", "clar-open", "coder", "which stage?", relay.ClarificationOpen, "unseen")
	built.State().Relay.Clarifications[open.Key] = relay.ClarificationState{
		ForgeRoot: root, RoomID: "!clarifications-forge-a", MessageID: "$message",
		Source: relay.ClarificationSourceTyped,
	}
	if err := built.reconcileTypedClarifications(
		root, Room{ClarificationsRoomID: "!clarifications-forge-a"}, []relay.Clarification{open}); err != nil {
		t.Fatalf("an unchanged typed snapshot tried to save state: %v", err)
	}
}

func TestTypedAnsweredAndWithdrawnClarificationsClearTheRecord(t *testing.T) {
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
			room := Room{ClarificationsRoomID: "!clarifications-forge-a"}
			if _, err := built.carryOutClarifications(
				context.Background(), root, store, room, nil,
				relay.DashboardSnapshot{TypedClarifications: []relay.Clarification{open}}, true); err != nil {
				t.Fatal(err)
			}

			ended := open
			ended.Lifecycle = lifecycle
			ended.Attention = "seen"
			ended.Notify = false
			if _, err := built.carryOutClarifications(
				context.Background(), root, store, room, nil,
				relay.DashboardSnapshot{TypedClarifications: []relay.Clarification{ended}}, true); err != nil {
				t.Fatal(err)
			}

			if state := built.State().Relay.Clarifications[open.Key]; state.MessageID != "" || state.Source != "" {
				t.Fatalf("state = %+v, want the typed record cleared", state)
			}
			if messages := rooms.sentMessages(); len(messages) != 1 {
				t.Fatalf("messages = %+v, want only the original notice", messages)
			}
		})
	}
}

func TestTypedClarificationMissingFromAHealthySnapshotClearsTheRecord(t *testing.T) {
	root := "/forges/forge-a"
	rooms := &fakeRooms{}
	store := &fakeClarifications{}
	built, _ := newTestBridgeWithClarifications(t, rooms,
		map[string]ForgeStore{root: &fakeStore{}},
		map[string]ClarificationStore{root: store}, root)
	typed := typedClarificationOf(
		"project-alpha", "clar-stale", "coder", "which stage?", relay.ClarificationOpen, "unseen")
	room := Room{ClarificationsRoomID: "!clarifications-forge-a"}
	if _, err := built.carryOutClarifications(
		context.Background(), root, store, room, nil,
		relay.DashboardSnapshot{TypedClarifications: []relay.Clarification{typed}}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := built.carryOutClarifications(
		context.Background(), root, store, room, nil, relay.DashboardSnapshot{}, true); err != nil {
		t.Fatal(err)
	}

	if state := built.State().Relay.Clarifications[typed.Key]; state.MessageID != "" || state.Source != "" {
		t.Fatalf("state = %+v, want the stale typed record cleared", state)
	}
	reloaded, err := state.Load(built.statePath)
	if err != nil {
		t.Fatal(err)
	}
	if record := reloaded.Relay.Clarifications[typed.Key]; record.MessageID != "" {
		t.Fatalf("persisted state = %+v, want the stale record cleared", record)
	}
}

func TestTypedReplyUsesTheTypedProjectChatPath(t *testing.T) {
	root := "/forges/forge-a"
	rooms := &fakeRooms{}
	store := &fakeClarifications{}
	built, _ := newTestBridgeWithClarifications(t, rooms,
		map[string]ForgeStore{root: &fakeStore{}},
		map[string]ClarificationStore{root: store}, root)
	typed := typedClarificationOf(
		"project-alpha", "clar-answer", "coder", "which stage?", relay.ClarificationOpen, "unseen")
	snapshot := relay.DashboardSnapshot{TypedClarifications: []relay.Clarification{typed}}
	room := Room{ClarificationsRoomID: "!clarifications-forge-a"}
	if _, err := built.carryOutClarifications(
		context.Background(), root, store, room, nil, snapshot, true); err != nil {
		t.Fatal(err)
	}
	messageID := built.State().Relay.Clarifications[typed.Key].MessageID
	reply := relay.RoomEvent{
		EventID: "$reply", RoomID: room.ClarificationsRoomID, Sender: operator,
		Body: "run qa", ThreadRoot: messageID,
	}

	if _, err := built.carryOutClarifications(
		context.Background(), root, store, room,
		[]relay.RoomEvent{reply}, snapshot, true); err != nil {
		t.Fatal(err)
	}

	answers := store.typedAnswers()
	if len(answers) != 1 || answers[0].project != "project-alpha" ||
		answers[0].id != "clar-answer" || answers[0].text != "run qa" {
		t.Fatalf("typed answers = %+v, want the project's normal answer", answers)
	}
	if legacy := store.answered(); len(legacy) != 0 {
		t.Fatalf("legacy answers = %+v, want no legacy endpoint use", legacy)
	}
}

func TestTypedAnswerFailureRetainsNoAnsweredRecord(t *testing.T) {
	root := "/forges/forge-a"
	rooms := &fakeRooms{}
	typed := typedClarificationOf(
		"project-alpha", "clar-failed", "coder", "which stage?", relay.ClarificationOpen, "seen")
	store := &fakeClarifications{errors: map[string]error{
		"project-alpha/clar-failed": fmt.Errorf("project chat is unavailable"),
	}}
	built, _ := newTestBridgeWithClarifications(t, rooms,
		map[string]ForgeStore{root: &fakeStore{}},
		map[string]ClarificationStore{root: store}, root)
	built.State().Relay.Clarifications[typed.Key] = relay.ClarificationState{
		ForgeRoot: root, RoomID: "!clarifications-forge-a", MessageID: "$message",
		Source: relay.ClarificationSourceTyped,
	}
	err := built.answerClarification(context.Background(), root, store, relay.ClarificationAction{
		Kind: relay.AnswerClarification, Key: typed.Key, Clarification: typed, Answer: "run qa",
	})
	if err == nil {
		t.Fatal("answerClarification accepted a failed typed answer")
	}
	if state := built.State().Relay.Clarifications[typed.Key]; state.Answer != "" {
		t.Fatalf("state = %+v, want no answered record after failure", state)
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
	if marks := rooms.sentReactions(); len(marks) != 0 {
		t.Errorf("reactions = %+v, want the desktop's answer to take no reaction", marks)
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
		t.Errorf("unhappy forges = %v, want the forge the bridge has no clarifications for", status.UnhappyForges)
	}
}
