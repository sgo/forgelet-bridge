package bridge

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/unclebob/forgelet-bridge/internal/relay"
	"github.com/unclebob/forgelet-bridge/internal/state"
)

type fakeSnapshotApprovals struct {
	*fakeApprovals
	source *fakeSnapshotSource
}

func (f fakeSnapshotApprovals) Identity() (string, error) {
	return f.source.Identity()
}

func (f fakeSnapshotApprovals) Snapshot(ctx context.Context) (relay.DashboardSnapshot, error) {
	return f.source.Snapshot(ctx)
}

func TestRunTickReportsWhetherThePassFailed(t *testing.T) {
	failed := &fakeRooms{drainErr: errors.New("rooms unavailable")}
	built, _ := newTestBridge(t, failed, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}}, "/forges/forge-a")
	if !built.runTick(context.Background()) {
		t.Error("runTick reported success for a failed pass")
	}

	built, _ = newTestBridge(t, &fakeRooms{}, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}}, "/forges/forge-a")
	if built.runTick(context.Background()) {
		t.Error("runTick reported failure for a successful pass")
	}
}

func TestBridgeRefusesAnUnreadableStateFile(t *testing.T) {
	path := t.TempDir()
	if err := os.WriteFile(filepath.Join(path, "bridge-state.json"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := newTestConfig(t, "/forges/forge-a")
	cfg.StateDir = path
	if _, err := New(cfg, &fakeRooms{}, nil, nil, nil, nil, nil); err == nil {
		t.Fatal("New accepted an unreadable state file")
	}
}

func TestForgeTickInputsReportsAForgeRoomFailure(t *testing.T) {
	root := "/forges/forge-a"
	rooms := &fakeRooms{ensureErr: errors.New("matrix unavailable")}
	built, _ := newTestBridge(t, rooms, map[string]ForgeStore{root: &fakeStore{}}, root)
	if _, _, _, err := built.forgeTickInputs(context.Background(), root); err == nil {
		t.Fatal("forgeTickInputs accepted an unavailable forge room")
	}
}

func TestForgeAndProjectCountsIncludeAllCarriedWork(t *testing.T) {
	root := "/forges/forge-a"
	forgeStore := &fakeStore{requests: []relay.Request{{ID: "forge-request", Body: "forge request"}}}
	projectStore := &fakeStore{requests: []relay.Request{{ID: "project-request", Body: "project request"}}}
	project := Project{
		Name:           "agent",
		Root:           root + "/projects/agent",
		Store:          projectStore,
		Clarifications: &fakeClarifications{},
	}
	rooms := &fakeRooms{}
	built := projectBridge(t, newTestConfig(t, root), rooms, fakeProjectSource{projects: []Project{project}})
	built.stores[root] = forgeStore
	seen := roomEvents{messages: map[string][]relay.RoomEvent{}, reactions: map[string][]relay.Reaction{}}

	if got := built.tickForgeAndProjects(context.Background(), root, seen, snapshotCache{}); got != 2 {
		t.Errorf("forge and project work = %d, want two messages", got)
	}
}

func TestFailedForgeCarriesNoProjectWork(t *testing.T) {
	root := "/forges/forge-a"
	project := Project{
		Name:           "agent",
		Root:           root + "/projects/agent",
		Store:          &fakeStore{requests: []relay.Request{{Body: "must not post"}}},
		Clarifications: &fakeClarifications{},
	}
	rooms := &fakeRooms{}
	built := projectBridge(t, newTestConfig(t, root), rooms, fakeProjectSource{projects: []Project{project}})
	built.stores = map[string]ForgeStore{}

	if got := built.tickForgeAndProjects(context.Background(), root, roomEvents{}, snapshotCache{}); got != 0 {
		t.Errorf("work after forge failure = %d, want none", got)
	}
	if len(rooms.ensuredProjectRooms()) != 0 {
		t.Error("a failed forge still touched its project")
	}
}

func TestProjectDiscoveryFailureCarriesNoWork(t *testing.T) {
	root := "/forges/forge-a"
	rooms := &fakeRooms{}
	built := projectBridge(t, newTestConfig(t, root), rooms, fakeProjectSource{err: errors.New("list unavailable")})
	built.stores[root] = &fakeStore{}
	if got := built.tickProjects(context.Background(), root, roomEvents{}, snapshotCache{}); got != 0 {
		t.Errorf("work after project discovery failure = %d, want none", got)
	}
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if built.unhappy[root] == "" {
		t.Error("project discovery failure was not reported")
	}
	if len(rooms.ensuredProjectRooms()) != 0 {
		t.Error("a failed project source still provisioned a room")
	}
}

func TestProjectAlreadyBeingRemovedCarriesNoWork(t *testing.T) {
	root := "/forges/forge-a"
	rooms := &fakeRooms{}
	built := projectBridge(t, newTestConfig(t, root), rooms, fakeProjectSource{})
	built.State().RecordForge(root, state.Forge{
		SpaceID:              "!space-forge-a",
		RoomID:               "!room-forge-a",
		ApprovalsRoomID:      "!approvals-forge-a",
		ActivityRoomID:       "!activity-forge-a",
		ClarificationsRoomID: "!clarifications-forge-a",
	})
	built.State().RecordProject(root, "agent", state.Project{
		RoomID:  "!project-agent-forge-a",
		Cleanup: &state.RoomCleanup{FactsCleared: true},
	})
	project := Project{Name: "agent", Root: root + "/projects/agent", Store: &fakeStore{}, Clarifications: &fakeClarifications{}}

	if got := built.tickProjectIfOpen(context.Background(), root, project, roomEvents{}, snapshotCache{}); got != 0 {
		t.Errorf("work for a removing project = %d, want none", got)
	}
}

func TestFailedProjectCarriesNoWork(t *testing.T) {
	root := "/forges/forge-a"
	rooms := &fakeRooms{}
	built := projectBridge(t, newTestConfig(t, root), rooms, fakeProjectSource{})
	project := Project{
		Name:           "agent",
		Root:           root + "/projects/agent",
		Store:          &fakeStore{readErr: errors.New("queue unavailable")},
		Clarifications: &fakeClarifications{},
	}

	if got := built.tickProjectIfOpen(context.Background(), root, project, roomEvents{}, snapshotCache{}); got != 0 {
		t.Errorf("work after project failure = %d, want none", got)
	}
	if built.unhappy[project.Root] == "" {
		t.Error("the project failure was not reported")
	}
}

func TestProjectRoomFailureCarriesNoWork(t *testing.T) {
	root := "/forges/forge-a"
	rooms := &fakeRooms{ensureErr: errors.New("matrix unavailable")}
	built := projectBridge(t, newTestConfig(t, root), rooms, fakeProjectSource{})
	project := Project{
		Name:           "agent",
		Root:           root + "/projects/agent",
		Store:          &fakeStore{},
		Clarifications: &fakeClarifications{},
	}

	carriedOut, err := built.tickProject(context.Background(), root, project, roomEvents{}, snapshotCache{})
	if err == nil {
		t.Fatal("tickProject accepted an unavailable project room")
	}
	if carriedOut != 0 {
		t.Errorf("carried out = %d, want none when the project room fails", carriedOut)
	}
}

func TestSnapshotCapabilityIsDiscoveredFromTheApprovalAdapter(t *testing.T) {
	root := "/forges/forge-a"
	source := &fakeSnapshotSource{identity: root + "/dashboard"}
	store := fakeSnapshotApprovals{fakeApprovals: &fakeApprovals{}, source: source}

	sources := snapshotStores(map[string]ApprovalStore{root: store})
	if sources[root] == nil {
		t.Fatal("snapshot capability was not discovered")
	}
	identity, err := sources[root].Identity()
	if err != nil || identity != source.identity {
		t.Errorf("snapshot identity = %q, %v, want %q", identity, err, source.identity)
	}
}

func TestSnapshotFailureCarriesNoStateWork(t *testing.T) {
	root := "/forges/forge-a"
	rooms := &fakeRooms{}
	built, _ := newTestBridge(t, rooms, map[string]ForgeStore{root: &fakeStore{}}, root)
	built.SetSnapshotSources(map[string]SnapshotStore{root: &fakeSnapshotSource{
		identity:  root + "/dashboard",
		err:       errors.New("unavailable"),
		callLimit: 1,
	}})
	room, err := built.roomFor(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	_, fromSnapshot, carriedOut, err := built.carryOutStateConsumers(
		context.Background(), root, room,
		roomEvents{messages: map[string][]relay.RoomEvent{}, reactions: map[string][]relay.Reaction{}},
		snapshotCache{})
	if err == nil {
		t.Fatal("carryOutStateConsumers accepted a failed snapshot")
	}
	if carriedOut != 0 {
		t.Errorf("carried out = %d, want none from a failed snapshot", carriedOut)
	}
	if !fromSnapshot {
		t.Error("a configured snapshot source was not reported after its read failed")
	}
}

func TestSnapshotSuccessReportsThatActivitySharesTheProjection(t *testing.T) {
	root := "/forges/forge-a"
	built, _ := newTestBridge(t, &fakeRooms{}, map[string]ForgeStore{root: &fakeStore{}}, root)
	built.SetSnapshotSources(map[string]SnapshotStore{root: &fakeSnapshotSource{
		identity: root + "/dashboard",
		snapshot: relay.DashboardSnapshot{},
	}})
	room, err := built.roomFor(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	_, fromSnapshot, _, err := built.carryOutStateConsumers(
		context.Background(), root, room,
		roomEvents{messages: map[string][]relay.RoomEvent{}, reactions: map[string][]relay.Reaction{}},
		snapshotCache{})
	if err != nil {
		t.Fatalf("carryOutStateConsumers: %v", err)
	}
	if !fromSnapshot {
		t.Error("a configured snapshot source was not reported after a successful read")
	}
}
