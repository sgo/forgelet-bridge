package bridge

import (
	"context"
	"errors"
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
		Key:      "/forges/forge-a/forgelet-bridge/snapshot-clarification",
		ID:       "snapshot-clarification",
		Project:  "forgelet-bridge",
		Role:     "coder",
		Question: "which lane?",
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
			Approvals:      []relay.Approval{snapshotApproval()},
			Clarifications: []relay.Clarification{snapshotClarification()},
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
			Approvals:      []relay.Approval{snapshotApproval()},
			Clarifications: []relay.Clarification{snapshotClarification()},
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
		snapshot: relay.DashboardSnapshot{Clarifications: []relay.Clarification{snapshotClarification()}},
	}
	project := Project{
		Name:           "project-a",
		Root:           "/forges/forge-a/projects/project-a",
		Store:          &fakeStore{},
		Clarifications: &fakeClarifications{},
		Snapshot:       source,
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
	project.Snapshot = other
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
