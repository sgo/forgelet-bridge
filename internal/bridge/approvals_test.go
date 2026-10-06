package bridge

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

// fakeApprovals is a forge's approvals for the bridge tests: what the projects
// are waiting for, and what the bridge decided.
type fakeApprovals struct {
	mu          sync.Mutex
	pending     []relay.Approval
	resolved    map[string]string
	resolvedErr error
	approved    []string
	sentBack    []sentBack
	errors      map[string]error
}

type sentBack struct {
	project  string
	id       string
	feedback string
}

func (f *fakeApprovals) Pending(_ context.Context) ([]relay.Approval, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]relay.Approval(nil), f.pending...), nil
}

func (f *fakeApprovals) Resolved(_ context.Context) (map[string]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.resolvedErr != nil {
		return nil, f.resolvedErr
	}
	resolved := map[string]string{}
	for key, resolution := range f.resolved {
		resolved[key] = resolution
	}
	return resolved, nil
}

func (f *fakeApprovals) setResolved(resolved map[string]string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resolved = resolved
}

func (f *fakeApprovals) Approve(_ context.Context, project, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.errors[project+"/"+id]; err != nil {
		return err
	}
	key := project + "/" + id
	f.approved = append(f.approved, key)
	f.pending = without(f.pending, key)
	return nil
}

func (f *fakeApprovals) SendBack(_ context.Context, project, id, feedback string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := project + "/" + id
	f.sentBack = append(f.sentBack, sentBack{project: project, id: id, feedback: feedback})
	f.pending = without(f.pending, key)
	return nil
}

func without(pending []relay.Approval, key string) []relay.Approval {
	kept := pending[:0]
	for _, approval := range pending {
		if approval.Key != key {
			kept = append(kept, approval)
		}
	}
	return kept
}

func (f *fakeApprovals) setPending(approvals ...relay.Approval) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pending = approvals
}

func (f *fakeApprovals) decisions() ([]string, []sentBack) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.approved...), append([]sentBack(nil), f.sentBack...)
}

func phoneApproval() relay.Approval {
	return relay.Approval{
		Key:       "forgelet-bridge/approval-1",
		Project:   "forgelet-bridge",
		ID:        "approval-1",
		Card:      "phone-approvals",
		Gate:      "coder → refactorer",
		Artifacts: []string{"internal/bridge/bridge.go", "internal/relay/relay.go"},
	}
}

func TestTickPostsPendingApprovalsIntoTheApprovalsRoom(t *testing.T) {
	store := &fakeApprovals{pending: []relay.Approval{phoneApproval()}}
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithApprovals(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": store}, "/forges/forge-a")

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	sent := rooms.sentMessages()
	if len(sent) != 1 {
		t.Fatalf("sent = %+v, want the approval in the approvals room", sent)
	}
	if sent[0].roomID != "!approvals-forge-a" {
		t.Errorf("room = %q, want the approvals room", sent[0].roomID)
	}
	for _, want := range []string{"phone-approvals", "forgelet-bridge", "coder → refactorer",
		"internal/bridge/bridge.go", "internal/relay/relay.go"} {
		if !strings.Contains(sent[0].body, want) {
			t.Errorf("approval message %q does not name %q", sent[0].body, want)
		}
	}
	state := built.State().Relay.Approvals["forgelet-bridge/approval-1"]
	if state.MessageID == "" {
		t.Error("the bridge did not remember the approval message")
	}
}

func TestTickApprovesWhenTheOperatorReacts(t *testing.T) {
	store := &fakeApprovals{pending: []relay.Approval{phoneApproval()}}
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithApprovals(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": store}, "/forges/forge-a")
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick: %v", err)
	}
	messageID := built.State().Relay.Approvals["forgelet-bridge/approval-1"].MessageID
	rooms.pushReaction(relay.Reaction{
		RoomID:        "!approvals-forge-a",
		Sender:        operator,
		Key:           relay.ApproveReaction,
		TargetEventID: messageID,
	})

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick: %v", err)
	}

	approved, _ := store.decisions()
	if len(approved) != 1 || approved[0] != "forgelet-bridge/approval-1" {
		t.Fatalf("approved = %v, want the operator's approval to reach the forge", approved)
	}
	state := built.State().Relay.Approvals["forgelet-bridge/approval-1"]
	if state.Resolution != relay.ResolutionApproved {
		t.Errorf("resolution = %q, want approved", state.Resolution)
	}
}

func TestTickRemembersTheApprovalsStateEvent(t *testing.T) {
	store := &fakeApprovals{pending: []relay.Approval{phoneApproval()}}
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithApprovals(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": store}, "/forges/forge-a")

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	state := built.State().Relay.Approvals["forgelet-bridge/approval-1"]
	if state.StateEventID == "" {
		t.Fatalf("the bridge did not remember the approval's state event: %+v", state)
	}
	if state.StateEventID == state.MessageID {
		t.Errorf("the state event id equals the message id (%q)", state.StateEventID)
	}
}

func TestTickApprovesWhenTheOperatorReactsToTheStateEvent(t *testing.T) {
	store := &fakeApprovals{pending: []relay.Approval{phoneApproval()}}
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithApprovals(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": store}, "/forges/forge-a")
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick: %v", err)
	}
	stateEventID := built.State().Relay.Approvals["forgelet-bridge/approval-1"].StateEventID
	if stateEventID == "" {
		t.Fatalf("the bridge did not remember the approval's state event")
	}
	rooms.pushReaction(relay.Reaction{
		RoomID:        "!approvals-forge-a",
		Sender:        operator,
		Key:           relay.ApproveReaction,
		TargetEventID: stateEventID,
	})

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick: %v", err)
	}

	approved, _ := store.decisions()
	if len(approved) != 1 || approved[0] != "forgelet-bridge/approval-1" {
		t.Fatalf("approved = %v, want the check mark on the state event to reach the forge", approved)
	}
}

func TestTickSendsBackAReplyThatNamesTheApprovalsStateEvent(t *testing.T) {
	store := &fakeApprovals{pending: []relay.Approval{phoneApproval()}}
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithApprovals(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": store}, "/forges/forge-a")
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick: %v", err)
	}
	stateEventID := built.State().Relay.Approvals["forgelet-bridge/approval-1"].StateEventID
	if stateEventID == "" {
		t.Fatalf("the bridge did not remember the approval's state event")
	}
	rooms.push(relay.RoomEvent{
		RoomID:  "!approvals-forge-a",
		EventID: "$reply",
		Sender:  operator,
		Body:    "> Approval for phone-approvals in forgelet-bridge\n\nthe timesheet total is still wrong",
		ReplyTo: stateEventID,
	})

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick: %v", err)
	}

	approved, back := store.decisions()
	if len(approved) != 0 || len(back) != 1 || back[0].id != "approval-1" {
		t.Fatalf("decisions = (%v, %+v), want the operator's reply on the state event sent back", approved, back)
	}
	if back[0].feedback != "the timesheet total is still wrong" {
		t.Errorf("feedback = %q, want the operator's own words", back[0].feedback)
	}
}

func TestTickMarksAnApprovalItResolvedOnTheMessage(t *testing.T) {
	store := &fakeApprovals{pending: []relay.Approval{phoneApproval()}}
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithApprovals(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": store}, "/forges/forge-a")
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick: %v", err)
	}
	messageID := built.State().Relay.Approvals["forgelet-bridge/approval-1"].MessageID
	rooms.pushReaction(relay.Reaction{RoomID: "!approvals-forge-a", Sender: operator, Key: relay.ApproveReaction, TargetEventID: messageID})
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick: %v", err)
	}

	rooms.sent = nil
	rooms.marks = nil
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("third Tick: %v", err)
	}

	marks := rooms.sentReactions()
	if len(marks) != 1 || marks[0].roomID != "!approvals-forge-a" || marks[0].target != messageID || marks[0].key != relay.CarriedOutReaction {
		t.Fatalf("reactions = %+v, want the bridge's mark on the approval's own message", marks)
	}
	if sent := rooms.sentMessages(); len(sent) != 0 {
		t.Errorf("sent = %+v, want no message when the approval is marked", sent)
	}
	if state := built.State().Relay.Approvals["forgelet-bridge/approval-1"]; state.ReactionID == "" {
		t.Error("the bridge did not remember the reaction")
	}
}

func TestTickMarksASendBackWithTheLeftArrowOnTheMessage(t *testing.T) {
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

	rooms.sent = nil
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("third Tick: %v", err)
	}

	marks := rooms.sentReactions()
	if len(marks) != 1 || marks[0].roomID != "!approvals-forge-a" || marks[0].target != messageID || marks[0].key != relay.SentBackReaction {
		t.Fatalf("reactions = %+v, want the send-back's left arrow on the approval's own message", marks)
	}
	for _, sent := range rooms.sentMessages() {
		if sent.anchor == messageID {
			t.Errorf("sent = %+v, want no reply under the message when the send-back is marked", rooms.sentMessages())
		}
	}
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("fourth Tick: %v", err)
	}
	if marks := rooms.sentReactions(); len(marks) != 1 {
		t.Errorf("reactions = %+v, want the send-back marked exactly once", marks)
	}
}

func TestTickMarksAnApprovalOnlyOnce(t *testing.T) {
	store := &fakeApprovals{pending: []relay.Approval{phoneApproval()}}
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithApprovals(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": store}, "/forges/forge-a")
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick: %v", err)
	}
	messageID := built.State().Relay.Approvals["forgelet-bridge/approval-1"].MessageID
	rooms.pushReaction(relay.Reaction{RoomID: "!approvals-forge-a", Sender: operator, Key: relay.ApproveReaction, TargetEventID: messageID})
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick: %v", err)
	}
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("third Tick: %v", err)
	}

	if marks := rooms.sentReactions(); len(marks) != 1 {
		t.Errorf("reactions = %+v, want the approval marked exactly once", marks)
	}
}

func TestTickSendsAnApprovalBackWithTheOperatorsReply(t *testing.T) {
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

	_, sentBack := store.decisions()
	if len(sentBack) != 1 || sentBack[0].feedback != "the timesheet total is still wrong" {
		t.Fatalf("sent back = %+v, want the operator's feedback", sentBack)
	}
}

func TestTickLeavesAnApprovalAloneWhenSomeoneElseReacts(t *testing.T) {
	store := &fakeApprovals{pending: []relay.Approval{phoneApproval()}}
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithApprovals(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": store}, "/forges/forge-a")
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick: %v", err)
	}
	messageID := built.State().Relay.Approvals["forgelet-bridge/approval-1"].MessageID
	rooms.pushReaction(relay.Reaction{RoomID: "!approvals-forge-a", Sender: "@stranger:example.org", Key: relay.ApproveReaction, TargetEventID: messageID})
	rooms.push(relay.RoomEvent{RoomID: "!approvals-forge-a", EventID: "$plain", Sender: operator, Body: "when is this due?"})

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick: %v", err)
	}

	approved, sentBack := store.decisions()
	if len(approved) != 0 || len(sentBack) != 0 {
		t.Errorf("decisions = %v / %+v, want none", approved, sentBack)
	}
	if state := built.State().Relay.Approvals["forgelet-bridge/approval-1"]; state.Resolution != "" {
		t.Errorf("resolution = %q, want the approval still undecided", state.Resolution)
	}
}

func TestTickKeepsTheApprovalsRoomOutOfTheChatChannel(t *testing.T) {
	store := &fakeApprovals{pending: []relay.Approval{phoneApproval()}}
	chat := &fakeStore{}
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithApprovals(t, rooms, map[string]ForgeStore{"/forges/forge-a": chat},
		map[string]ApprovalStore{"/forges/forge-a": store}, "/forges/forge-a")
	rooms.push(relay.RoomEvent{RoomID: "!approvals-forge-a", EventID: "$plain", Sender: operator, Body: "when is this due?"})

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	if got := chat.createdBodies(); len(got) != 0 {
		t.Errorf("chat requests = %v, want none from an approvals room message", got)
	}
}

func TestTickReportsAnApprovalResolvedOnTheDesktop(t *testing.T) {
	store := &fakeApprovals{pending: []relay.Approval{phoneApproval()}}
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithApprovals(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": store}, "/forges/forge-a")
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick: %v", err)
	}
	messageID := built.State().Relay.Approvals["forgelet-bridge/approval-1"].MessageID
	store.setPending()
	rooms.sent = nil

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick: %v", err)
	}

	sent := rooms.sentMessages()
	if len(sent) != 1 || sent[0].body != "Resolved on the desktop" || sent[0].anchor != messageID {
		t.Fatalf("sent = %+v, want the desktop resolution in the thread", sent)
	}
	if state := built.State().Relay.Approvals["forgelet-bridge/approval-1"]; state.Resolution != relay.ResolutionDesktop {
		t.Errorf("resolution = %q, want desktop", state.Resolution)
	}
}

func TestTickMarksAnApprovalTheDeskResolved(t *testing.T) {
	cases := map[string]string{
		relay.ResolutionApproved: relay.CarriedOutReaction,
		relay.ResolutionSentBack: relay.SentBackReaction,
	}
	for resolution, mark := range cases {
		store := &fakeApprovals{pending: []relay.Approval{phoneApproval()}}
		rooms := &fakeRooms{}
		built, _ := newTestBridgeWithApprovals(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
			map[string]ApprovalStore{"/forges/forge-a": store}, "/forges/forge-a")
		if err := built.Tick(context.Background()); err != nil {
			t.Fatalf("%s: first Tick: %v", resolution, err)
		}
		messageID := built.State().Relay.Approvals["forgelet-bridge/approval-1"].MessageID
		// The desk resolves it: it is no longer pending, and the desk wrote
		// down how it ended.
		store.setPending()
		store.setResolved(map[string]string{"forgelet-bridge/approval-1": resolution})
		rooms.sent = nil
		rooms.marks = nil

		if err := built.Tick(context.Background()); err != nil {
			t.Fatalf("%s: second Tick: %v", resolution, err)
		}

		marks := rooms.sentReactions()
		if len(marks) != 1 || marks[0].target != messageID || marks[0].key != mark {
			t.Fatalf("%s: reactions = %+v, want the desk's ending marked on the approval's own message", resolution, marks)
		}
		for _, sent := range rooms.sentMessages() {
			if sent.anchor == messageID {
				t.Errorf("%s: sent = %+v, want no reply when the desk's ending is marked", resolution, rooms.sentMessages())
			}
		}
		approved, sentBack := store.decisions()
		if len(approved) != 0 || len(sentBack) != 0 {
			t.Errorf("%s: decisions = %v / %+v, want the desk's resolution left alone", resolution, approved, sentBack)
		}
	}
}

func TestTickReportsAForgeWhoseDeskRecordCannotBeRead(t *testing.T) {
	// A desk the bridge cannot read is that forge's problem, reported like any
	// other, not a tick that takes the rest of the bridge down with it.
	store := &fakeApprovals{
		pending:     []relay.Approval{phoneApproval()},
		resolvedErr: fmt.Errorf("the desk is not there"),
	}
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithApprovals(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": store}, "/forges/forge-a")

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	status := statusOf(t, built)
	if len(status.UnhappyForges) != 1 || status.UnhappyForges[0] != "forge-a" {
		t.Errorf("unhappy forges = %v, want the forge whose desk record cannot be read", status.UnhappyForges)
	}
}

func TestTickReportsAResolutionOnlyOnce(t *testing.T) {
	store := &fakeApprovals{pending: []relay.Approval{phoneApproval()}}
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithApprovals(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": store}, "/forges/forge-a")
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick: %v", err)
	}
	messageID := built.State().Relay.Approvals["forgelet-bridge/approval-1"].MessageID
	store.setPending()
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick: %v", err)
	}
	rooms.sent = nil
	rooms.pushReaction(relay.Reaction{RoomID: "!approvals-forge-a", Sender: operator, Key: relay.ApproveReaction, TargetEventID: messageID})

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("third Tick: %v", err)
	}

	if sent := rooms.sentMessages(); len(sent) != 0 {
		t.Errorf("sent = %+v, want nothing after the approval was already resolved", sent)
	}
	approved, _ := store.decisions()
	if len(approved) != 0 {
		t.Errorf("approved = %v, want the resolved approval left alone", approved)
	}
}

func TestTickKeepsGoingWhenAnApprovalCannotBeDecided(t *testing.T) {
	store := &fakeApprovals{
		pending: []relay.Approval{phoneApproval()},
		errors:  map[string]error{"forgelet-bridge/approval-1": fmt.Errorf("the forge is not there")},
	}
	board := &fakeBoard{}
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithStores(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": store},
		map[string]BoardStore{"/forges/forge-a": board}, "/forges/forge-a")
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick: %v", err)
	}
	messageID := built.State().Relay.Approvals["forgelet-bridge/approval-1"].MessageID
	rooms.pushReaction(relay.Reaction{RoomID: "!approvals-forge-a", Sender: operator, Key: relay.ApproveReaction, TargetEventID: messageID})
	board.set(boardCard("coder"))
	rooms.sent = nil

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("the refused approval failed the whole tick: %v", err)
	}

	// The operator is told the room could not carry it out...
	told := false
	for _, sent := range rooms.sentMessages() {
		if strings.Contains(sent.body, "Could not approve it") && sent.anchor == messageID {
			told = true
		}
	}
	if !told {
		t.Errorf("sent = %+v, want the refusal reported in the approval's thread", rooms.sentMessages())
	}
	// ...and the card update behind it still went out.
	update := false
	for _, sent := range rooms.sentMessages() {
		if strings.Contains(sent.body, "card card-activity-feed appeared") {
			update = true
		}
	}
	if !update {
		t.Errorf("sent = %+v, want the card update to go ahead", rooms.sentMessages())
	}
	// The refused approval is kept for another try.
	if state := built.State().Relay.Approvals["forgelet-bridge/approval-1"]; state.Resolution != "" {
		t.Errorf("resolution = %q, want the approval still undecided", state.Resolution)
	}

	// Once the forge can take it, the retry lands.
	store.mu.Lock()
	store.errors = nil
	store.mu.Unlock()
	rooms.sent = nil
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("retry Tick: %v", err)
	}
	approved, _ := store.decisions()
	if len(approved) != 1 {
		t.Fatalf("approved = %v, want the retry to reach the forge", approved)
	}
}

func TestApprovalMessageNamesTheOneChangedFileItWasGiven(t *testing.T) {
	approval := phoneApproval()
	approval.Artifacts = []string{"internal/bridge/bridge.go"}

	message := approvalMessage(approval)

	if !strings.Contains(message, "Changed files: internal/bridge/bridge.go") {
		t.Errorf("message = %q, want the one changed file named", message)
	}
}

func TestApprovalMessageLeavesTheFilesOutWhenTheApprovalNamesNone(t *testing.T) {
	approval := phoneApproval()
	approval.Artifacts = nil

	if message := approvalMessage(approval); strings.Contains(message, "Changed files") {
		t.Errorf("message = %q, want no changed-files line for an approval that names none", message)
	}
}

func TestTickReportsWorkWhenItPostsAnApproval(t *testing.T) {
	store := &fakeApprovals{pending: []relay.Approval{phoneApproval()}}
	rooms := &fakeRooms{}
	chat := &fakeStore{requests: []relay.Request{{ID: "req-1", Body: "is the build green?"}}}
	built, cfg := newTestBridgeWithApprovals(t, rooms, map[string]ForgeStore{"/forges/forge-a": chat},
		map[string]ApprovalStore{"/forges/forge-a": store}, "/forges/forge-a")

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	if status := readStatus(t, filepath.Join(cfg.StateDir, StatusName)); status.Idle {
		t.Errorf("status = %+v, want the tick that posted a chat message and an approval to report work", status)
	}
}
