package bridge

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

type fakeSnapshotSource struct {
	mu        sync.Mutex
	identity  string
	snapshot  relay.DashboardSnapshot
	err       error
	calls     int
	callLimit int
}

type fakeSnapshotClarifications struct {
	*fakeClarifications
	source *fakeSnapshotSource
}

func (f fakeSnapshotClarifications) Identity() (string, error) {
	return f.source.Identity()
}

func (f fakeSnapshotClarifications) Snapshot(ctx context.Context) (relay.DashboardSnapshot, error) {
	return f.source.Snapshot(ctx)
}

func (f *fakeSnapshotSource) Identity() (string, error) {
	return f.identity, nil
}

func (f *fakeSnapshotSource) Snapshot(_ context.Context) (relay.DashboardSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.callLimit > 0 && f.calls <= f.callLimit {
		return relay.DashboardSnapshot{}, f.err
	}
	return f.snapshot, nil
}

func (f *fakeSnapshotSource) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func snapshotApproval() relay.Approval {
	return relay.Approval{
		Key:     "/forges/forge-a/forgelet-bridge/snapshot-approval",
		Project: "forgelet-bridge",
		ID:      "snapshot-approval",
		Card:    "snapshot-card",
		Gate:    "coder → refactorer",
	}
}

func snapshotClarification() relay.Clarification {
	return relay.Clarification{
		Key:       "/forges/forge-a/typed/forgelet-bridge/snapshot-clarification",
		ID:        "snapshot-clarification",
		Project:   "forgelet-bridge",
		Role:      "coder",
		Question:  "which lane?",
		Source:    relay.ClarificationSourceTyped,
		Lifecycle: relay.ClarificationOpen,
		Attention: "unseen",
	}
}

func TestTickCarriesApprovalsAndClarificationsFromOneSnapshot(t *testing.T) {
	root := "/forges/forge-a"
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithStores(t, rooms,
		map[string]ForgeStore{root: &fakeStore{}},
		map[string]ApprovalStore{root: &fakeApprovals{}},
		map[string]BoardStore{root: &fakeBoard{}}, root)
	source := &fakeSnapshotSource{
		identity: root + "/dashboard",
		snapshot: relay.DashboardSnapshot{
			Approvals:           []relay.Approval{snapshotApproval()},
			TypedClarifications: []relay.Clarification{snapshotClarification()},
		},
	}
	built.SetSnapshotSources(map[string]SnapshotStore{root: source})

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if got := source.callCount(); got != 1 {
		t.Fatalf("snapshot calls = %d, want one for both consumers", got)
	}
	if lastMessageIn(rooms, "!approvals-forge-a") == nil {
		t.Error("the pending approval did not reach the approvals room")
	}
	if lastMessageIn(rooms, "!clarifications-forge-a") == nil {
		t.Error("the pending clarification did not reach the clarifications room")
	}
}

func TestTickCarriesDeskResolutionFromTheSameSnapshot(t *testing.T) {
	root := "/forges/forge-a"
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithStores(t, rooms,
		map[string]ForgeStore{root: &fakeStore{}},
		map[string]ApprovalStore{root: &fakeApprovals{}},
		map[string]BoardStore{root: &fakeBoard{}}, root)
	approval := snapshotApproval()
	source := &fakeSnapshotSource{
		identity: root + "/dashboard",
		snapshot: relay.DashboardSnapshot{Approvals: []relay.Approval{approval}},
	}
	built.SetSnapshotSources(map[string]SnapshotStore{root: source})
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick: %v", err)
	}

	source.mu.Lock()
	source.snapshot = relay.DashboardSnapshot{ResolvedApprovals: map[string]string{
		approval.Key: relay.ResolutionApproved,
	}}
	source.mu.Unlock()
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick: %v", err)
	}
	if got := source.callCount(); got != 2 {
		t.Fatalf("snapshot calls = %d, want one in each tick", got)
	}
	marks := rooms.sentReactions()
	if len(marks) != 1 || marks[0].key != relay.CarriedOutReaction {
		t.Errorf("marks = %+v, want the desk resolution carried to the message", marks)
	}
}

func TestFailedSnapshotIsUsedByNoConsumerAndIsNotRetriedInTheTick(t *testing.T) {
	root := "/forges/forge-a"
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithStores(t, rooms,
		map[string]ForgeStore{root: &fakeStore{}},
		map[string]ApprovalStore{root: &fakeApprovals{}},
		map[string]BoardStore{root: &fakeBoard{}}, root)
	source := &fakeSnapshotSource{
		identity: root + "/dashboard",
		err:      errors.New("dashboard unavailable"),
		snapshot: relay.DashboardSnapshot{
			Approvals:           []relay.Approval{snapshotApproval()},
			TypedClarifications: []relay.Clarification{snapshotClarification()},
		},
		callLimit: 1,
	}
	built.SetSnapshotSources(map[string]SnapshotStore{root: source})

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("failed Tick: %v", err)
	}
	if got := source.callCount(); got != 1 {
		t.Fatalf("snapshot calls = %d, want exactly one failed read", got)
	}
	if got := len(rooms.sentMessages()); got != 0 {
		t.Fatalf("sent messages = %d, want none from a failed snapshot", got)
	}
	if built.unhappy[root] == "" {
		t.Error("the failed snapshot was not reported against its forge")
	}

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("recovery Tick: %v", err)
	}
	if got := source.callCount(); got != 2 {
		t.Fatalf("snapshot calls after recovery = %d, want a fresh read", got)
	}
	if lastMessageIn(rooms, "!approvals-forge-a") == nil ||
		lastMessageIn(rooms, "!clarifications-forge-a") == nil {
		t.Error("the healthy tick did not recover both consumers")
	}
}

func TestSnapshotCacheIsScopedToTheTickAndDashboardIdentity(t *testing.T) {
	root := "/forges/forge-a"
	rooms := &fakeRooms{}
	built, cfg := newTestBridgeWithStores(t, rooms,
		map[string]ForgeStore{root: &fakeStore{}},
		map[string]ApprovalStore{root: &fakeApprovals{}},
		map[string]BoardStore{root: &fakeBoard{}}, root)
	source := &fakeSnapshotSource{
		identity: "shared-dashboard",
		snapshot: relay.DashboardSnapshot{TypedClarifications: []relay.Clarification{snapshotClarification()}},
	}
	project := Project{
		Name:           "project-a",
		Root:           "/forges/forge-a/projects/project-a",
		Store:          &fakeStore{},
		Clarifications: fakeSnapshotClarifications{fakeClarifications: &fakeClarifications{}, source: source},
	}
	built.SetSnapshotSources(map[string]SnapshotStore{root: source})
	built.SetProjectSource(fakeProjectSource{projects: []Project{project}})

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick: %v", err)
	}
	if got := source.callCount(); got != 1 {
		t.Fatalf("snapshot calls = %d, want the forge and project to share one", got)
	}
	if cfg.Forges[0].Root != root {
		t.Fatalf("configuration root = %q, want %q", cfg.Forges[0].Root, root)
	}

	other := &fakeSnapshotSource{identity: "other-dashboard", snapshot: source.snapshot}
	built.SetSnapshotSources(map[string]SnapshotStore{root: other})
	project.Clarifications = fakeSnapshotClarifications{fakeClarifications: &fakeClarifications{}, source: other}
	built.SetProjectSource(fakeProjectSource{projects: []Project{project}})
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick: %v", err)
	}
	if got := source.callCount(); got != 1 {
		t.Fatalf("previous source calls = %d, want no state reused across ticks", got)
	}
	if got := other.callCount(); got != 1 {
		t.Fatalf("new source calls = %d, want a fresh read for the new tick", got)
	}
}

func TestDifferentDashboardIdentitiesEachReadOncePerTick(t *testing.T) {
	roots := []string{"/forges/forge-a", "/forges/forge-b"}
	rooms := &fakeRooms{}
	approvalStores := map[string]ApprovalStore{}
	boards := map[string]BoardStore{}
	chats := map[string]ForgeStore{}
	for _, root := range roots {
		approvalStores[root] = &fakeApprovals{}
		boards[root] = &fakeBoard{}
		chats[root] = &fakeStore{}
	}
	built, _ := newTestBridgeWithStores(t, rooms, chats, approvalStores, boards, roots...)
	sources := map[string]SnapshotStore{}
	fakes := map[string]*fakeSnapshotSource{}
	for _, root := range roots {
		fake := &fakeSnapshotSource{identity: root + "/dashboard"}
		fakes[root] = fake
		sources[root] = fake
	}
	built.SetSnapshotSources(sources)

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	for _, root := range roots {
		if got := fakes[root].callCount(); got != 1 {
			t.Errorf("%s snapshot calls = %d, want one", root, got)
		}
	}
}

func TestProjectClarificationsLeaveNestedProjectsToTheFamily(t *testing.T) {
	root := "/forges/forge-a"
	projectRoot := root + "/projects/agent"
	project := Project{
		Name:           "agent",
		Root:           projectRoot,
		Store:          &fakeStore{},
		Clarifications: fakeSnapshotClarifications{fakeClarifications: &fakeClarifications{}, source: nil},
	}
	source := &fakeSnapshotSource{
		identity: projectRoot + "/dashboard",
		snapshot: relay.DashboardSnapshot{TypedClarifications: []relay.Clarification{
			clarificationOf("agent", "clar-1", "coder", "which stage is next?"),
		}},
	}
	project.Clarifications = fakeSnapshotClarifications{fakeClarifications: &fakeClarifications{}, source: source}
	rooms := &fakeRooms{}
	built := projectBridge(t, newTestConfig(t, root), rooms, fakeProjectSource{})
	room := projectRoom("!space-forge-a", "!project-agent-forge-a")
	seen := roomEvents{messages: map[string][]relay.RoomEvent{}, reactions: map[string][]relay.Reaction{}}

	carriedOut, err := built.carryOutProjectClarifications(context.Background(), root, project, room, seen, snapshotCache{})
	if err != nil {
		t.Fatalf("carryOutProjectClarifications: %v", err)
	}
	if carriedOut != 0 || len(rooms.sentMessages()) != 0 {
		t.Errorf("carried out = %d, sent = %+v, want the Family-only authority", carriedOut, rooms.sentMessages())
	}
	if source.callCount() != 0 {
		t.Error("the nested project snapshot was read instead of leaving typed work to the Family")
	}
}

func TestProjectAtTheForgeRootRequiresItsOwnAutomationSnapshot(t *testing.T) {
	root := "/forges/forge-a"
	project := Project{
		Name:           "forge-a",
		Root:           root,
		Clarifications: &fakeClarifications{},
	}
	built, _ := newTestBridgeWithStores(t, &fakeRooms{},
		map[string]ForgeStore{root: &fakeStore{}},
		map[string]ApprovalStore{root: &fakeApprovals{}},
		map[string]BoardStore{root: &fakeBoard{}}, root)
	carriedOut, err := built.carryOutProjectClarifications(
		context.Background(), root, project, Room{ClarificationsRoomID: "!project"},
		roomEvents{}, snapshotCache{})
	if err == nil || !strings.Contains(err.Error(), "require an automation snapshot") {
		t.Fatalf("error = %v, want the missing snapshot refusal", err)
	}
	if carriedOut != 0 {
		t.Fatalf("carried out = %d, want none without a snapshot", carriedOut)
	}
}

func TestProjectSnapshotLeavesFamilyOwnedTypedClarificationsToTheForgeTick(t *testing.T) {
	root := "/forges/forge-a"
	rooms := &fakeRooms{}
	project := Project{
		Name:  "agent",
		Root:  root + "/projects/agent",
		Store: &fakeStore{},
		Clarifications: fakeSnapshotClarifications{
			fakeClarifications: &fakeClarifications{},
			source: &fakeSnapshotSource{
				identity: "project-dashboard",
				snapshot: relay.DashboardSnapshot{TypedClarifications: []relay.Clarification{
					typedClarificationOf(
						"agent", "clar-typed", "agent", "which stage?", relay.ClarificationOpen, "unseen"),
				}},
			},
		},
	}
	built, _ := newTestBridgeWithStores(t, rooms,
		map[string]ForgeStore{root: &fakeStore{}},
		map[string]ApprovalStore{root: &fakeApprovals{}},
		map[string]BoardStore{root: &fakeBoard{}}, root)
	built.SetProjectSource(fakeProjectSource{projects: []Project{project}})

	if _, err := built.carryOutProjectClarifications(
		context.Background(), root, project, Room{ClarificationsRoomID: "!project"}, roomEvents{},
		snapshotCache{}); err != nil {
		t.Fatal(err)
	}
	if messages := rooms.sentMessages(); len(messages) != 0 {
		t.Fatalf("messages = %+v, want no duplicate project-room typed notice", messages)
	}
}

func TestFamilyOwnsTypedClarificationsOnlyForNestedProjects(t *testing.T) {
	root := "/forges/forge-a"
	if !typedClarificationsHandledByFamily(root, Project{Root: root + "/projects/agent"}) {
		t.Error("a nested project's typed row was not left to the Family tick")
	}
	if typedClarificationsHandledByFamily(root, Project{Root: root}) {
		t.Error("a project configured as the forge root lost its typed row")
	}
}

func TestProjectClarificationSnapshotFailureCarriesNoWork(t *testing.T) {
	source := &fakeSnapshotSource{identity: "project-dashboard", err: errors.New("unavailable"), callLimit: 1}
	project := Project{
		Name: "agent",
		Root: "/forges/forge-a",
		Clarifications: fakeSnapshotClarifications{
			fakeClarifications: &fakeClarifications{},
			source:             source,
		},
	}
	built := &Bridge{}
	carriedOut, err := built.carryOutProjectClarifications(
		context.Background(), "/forges/forge-a", project, Room{ClarificationsRoomID: "!project"}, roomEvents{}, snapshotCache{})
	if err == nil {
		t.Fatal("carryOutProjectClarifications accepted a failed snapshot")
	}
	if carriedOut != 0 {
		t.Errorf("carried out = %d, want none from a failed project snapshot", carriedOut)
	}
}
