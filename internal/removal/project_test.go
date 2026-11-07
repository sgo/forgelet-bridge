package removal

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/unclebob/forgelet-bridge/internal/config"
	"github.com/unclebob/forgelet-bridge/internal/relay"
	"github.com/unclebob/forgelet-bridge/internal/state"
)

func TestInspectProjectRefusesUnsafeAndUnknownIdentity(t *testing.T) {
	cfg := projectConfig("/forges/one")
	loaded := projectState("/forges/one")
	catalog := fakeProjectCatalog{forgeExists: true, managed: true}

	tests := []struct {
		name    string
		root    string
		project string
		catalog ProjectCatalog
		want    Result
	}{
		{name: "blank root", root: "", project: "target", catalog: catalog, want: refused("unsafe_root")},
		{name: "relative root", root: "one", project: "target", catalog: catalog, want: refused("unsafe_root")},
		{name: "traversing root", root: "/forges/../one", project: "target", catalog: catalog, want: refused("unsafe_root")},
		{name: "unknown root", root: "/forges/two", project: "target", catalog: catalog, want: refused("unknown_root")},
		{name: "configured non-Forgelet", root: "/forges/one", project: "target", catalog: fakeProjectCatalog{managed: true}, want: refused("unknown_root")},
		{name: "blank project", root: "/forges/one", project: "", catalog: catalog, want: refused("unsafe_project")},
		{name: "padded project", root: "/forges/one", project: " target ", catalog: catalog, want: refused("unsafe_project")},
		{name: "parent project", root: "/forges/one", project: "../target", catalog: catalog, want: refused("unsafe_project")},
		{name: "absolute project", root: "/forges/one", project: "/target", catalog: catalog, want: refused("unsafe_project")},
		{name: "matrix room id", root: "/forges/one", project: "!room:example.org", catalog: catalog, want: refused("unsafe_project")},
		{name: "backslash", root: "/forges/one", project: "target\\child", catalog: catalog, want: refused("unsafe_project")},
		{name: "unknown project", root: "/forges/one", project: "missing", catalog: fakeProjectCatalog{forgeExists: true}, want: refused("unknown_project")},
		{name: "no catalog", root: "/forges/one", project: "target", catalog: nil, want: refused("state_unavailable")},
		{name: "forge lookup failure", root: "/forges/one", project: "target", catalog: fakeProjectCatalog{forgeErr: errors.New("read failed")}, want: refused("state_unavailable")},
		{name: "project lookup failure", root: "/forges/one", project: "target", catalog: fakeProjectCatalog{forgeExists: true, managedErr: errors.New("read failed")}, want: refused("state_unavailable")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := InspectProject(cfg, loaded, test.root, test.project, test.catalog)
			if got.Result != test.want {
				t.Fatalf("InspectProject = %+v, want %+v", got.Result, test.want)
			}
			if got.Root != "" || got.ProjectName != "" || got.RoomID != "" {
				t.Fatalf("refused inspection exposed work: %+v", got)
			}
		})
	}
}

func TestInspectProjectDistinguishesNothingFromIncompleteState(t *testing.T) {
	cfg := projectConfig("/forges/one")
	catalog := fakeProjectCatalog{forgeExists: true, managed: true}

	missingForge := projectState("/forges/one")
	delete(missingForge.Forges, "/forges/one")
	if got := InspectProject(cfg, missingForge, "/forges/one", "target", catalog); got.Result != (Result{Status: NothingToRemove}) {
		t.Fatalf("missing forge = %+v, want nothing_to_remove", got.Result)
	}

	missingProject := projectState("/forges/one")
	forge := missingProject.Forges["/forges/one"]
	delete(forge.Projects, "target")
	missingProject.Forges["/forges/one"] = forge
	if got := InspectProject(cfg, missingProject, "/forges/one", "target", catalog); got.Result != (Result{Status: NothingToRemove}) {
		t.Fatalf("missing project = %+v, want nothing_to_remove", got.Result)
	}

	incomplete := projectState("/forges/one")
	forge = incomplete.Forges["/forges/one"]
	project := forge.Projects["target"]
	project.RoomID = ""
	forge.Projects["target"] = project
	incomplete.Forges["/forges/one"] = forge
	if got := InspectProject(cfg, incomplete, "/forges/one", "target", catalog); got.Result != refused("incomplete_project_state") {
		t.Fatalf("incomplete project = %+v", got.Result)
	}

	forgeIncomplete := projectState("/forges/one")
	forge = forgeIncomplete.Forges["/forges/one"]
	forge.ActivityRoomID = ""
	forgeIncomplete.Forges["/forges/one"] = forge
	if got := InspectProject(cfg, forgeIncomplete, "/forges/one", "target", catalog); got.Result != refused("incomplete_project_state") {
		t.Fatalf("incomplete forge = %+v", got.Result)
	}
}

func TestInspectProjectRefusesEveryRoomIdCollision(t *testing.T) {
	cfg := projectConfig("/forges/one")
	catalog := fakeProjectCatalog{forgeExists: true, managed: true}
	collisions := map[string]string{
		"space":          "!space:example.org",
		"chat":           "!chat:example.org",
		"approvals":      "!approvals:example.org",
		"activity":       "!activity:example.org",
		"clarifications": "!clarifications:example.org",
		"sibling":        "!sibling:example.org",
	}
	for name, roomID := range collisions {
		t.Run(name, func(t *testing.T) {
			loaded := projectState("/forges/one")
			forge := loaded.Forges["/forges/one"]
			project := forge.Projects["target"]
			project.RoomID = roomID
			forge.Projects["target"] = project
			loaded.Forges["/forges/one"] = forge
			if got := InspectProject(cfg, loaded, "/forges/one", "target", catalog); got.Result != refused("incomplete_project_state") {
				t.Fatalf("collision %s = %+v", name, got.Result)
			}
		})
	}
}

func TestInspectProjectPlansOnlyTheRecordedTargetRoom(t *testing.T) {
	cfg := projectConfig("/forges/one")
	loaded := projectState("/forges/one")
	got := InspectProject(cfg, loaded, "/forges/one", "target",
		fakeProjectCatalog{forgeExists: true, managed: true})
	if got.Result != (Result{Status: Ready}) {
		t.Fatalf("InspectProject = %+v, want ready", got.Result)
	}
	if got.ProjectName != "target" || got.RoomID != "!target:example.org" ||
		got.Forge.ActivityRoomID != "!activity:example.org" {
		t.Fatalf("inspection = %+v, want only target room and forge context", got)
	}
	if got.RoomID == loaded.Forges["/forges/one"].Projects["sibling"].RoomID {
		t.Fatal("target inspection selected the sibling room")
	}
}

func TestRemoveProjectCleansOneRoomAndPreservesEverySiblingFact(t *testing.T) {
	loaded := projectState("/forges/one")
	path := writeRemovalState(t, loaded)
	cfg := projectConfig("/forges/one")
	catalog := fakeProjectCatalog{forgeExists: true, managed: true}
	cleaner := &fakeProjectCleaner{}
	service := Service{
		StatePath:      path,
		ProjectCleaner: cleaner,
		Projects:       catalog,
		Operator:       "@operator:example.org",
		Config:         cfg,
	}

	got := service.RemoveProject(context.Background(),
		InspectProject(cfg, loaded, "/forges/one", "target", catalog))
	if got != (Result{Status: Removed}) {
		t.Fatalf("RemoveProject = %+v, want removed", got)
	}
	wantCalls := []string{
		"facts !target:example.org !activity:example.org !approvals:example.org target",
		"operator !target:example.org",
		"leave !target:example.org",
	}
	if !reflect.DeepEqual(cleaner.calls, wantCalls) {
		t.Fatalf("calls = %v, want %v", cleaner.calls, wantCalls)
	}

	final, err := state.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	forge := final.Forges["/forges/one"]
	if _, ok := forge.Projects["target"]; ok {
		t.Fatalf("target project record remains: %+v", forge.Projects)
	}
	if forge.Projects["sibling"].RoomID != "!sibling:example.org" {
		t.Fatalf("sibling project changed: %+v", forge.Projects)
	}
	if forge.SpaceID != "!space:example.org" || forge.RoomID != "!chat:example.org" ||
		forge.ApprovalsRoomID != "!approvals:example.org" ||
		forge.ActivityRoomID != "!activity:example.org" ||
		forge.ClarificationsRoomID != "!clarifications:example.org" {
		t.Fatalf("forge-level rooms changed: %+v", forge)
	}

	if _, ok := final.Relay.Waiting["!target:example.org\n"+relay.BoardFactType+"\ntarget"]; ok {
		t.Error("target project-room fact remains")
	}
	if _, ok := final.Relay.Waiting["!activity:example.org\n"+relay.BoardFactType+"\ntarget"]; ok {
		t.Error("target activity board fact remains")
	}
	if _, ok := final.Relay.Approvals["target/approval"]; ok {
		t.Error("target approval bookkeeping remains")
	}
	if _, ok := final.Relay.Clarifications["target/clarification"]; ok {
		t.Error("target clarification bookkeeping remains")
	}
	if _, ok := final.Relay.Clarifications["orphan-in-target-room"]; ok {
		t.Error("a clarification in the target room was not removed")
	}
	if _, ok := final.Relay.Clarifications["target/orphan-in-sibling-room"]; ok {
		t.Error("a target-prefixed clarification was not removed")
	}
	if _, ok := final.Relay.Activity["target/card"]; ok {
		t.Error("target activity bookkeeping remains")
	}
	if _, ok := final.Relay.Threads["target-request"]; ok {
		t.Error("target project-scoped thread remains")
	}
	if final.Relay.Waiting["!activity:example.org\n"+relay.BoardFactType+"\nsibling"] == "" ||
		final.Relay.Waiting["!sibling:example.org\n"+relay.BoardFactType+"\nsibling"] == "" ||
		final.Relay.Approvals["sibling/approval"].MessageID == "" ||
		final.Relay.Clarifications["sibling/clarification"].MessageID == "" ||
		final.Relay.Activity["sibling/card"].Project != "sibling" ||
		final.Relay.Threads["sibling-request"] == "" {
		t.Fatalf("sibling relay facts were not preserved: %+v", final.Relay)
	}
	if final.Relay.Approvals["forge/approval"].MessageID == "" ||
		final.Relay.Clarifications["forge/clarification"].MessageID == "" ||
		final.Relay.Threads["forge-request"] == "" {
		t.Fatalf("forge-level relay facts were not preserved: %+v", final.Relay)
	}
}

func TestRemoveProjectResumesAtEachFailedMatrixStage(t *testing.T) {
	tests := []struct {
		failure    string
		reason     string
		completed  func(*testing.T, *state.State)
		retryCalls []string
	}{
		{
			failure: "facts",
			reason:  "relay_cleanup_failed",
			completed: func(t *testing.T, loaded *state.State) {
				t.Helper()
				if loaded.Forges["/forges/one"].Projects["target"].Cleanup != nil {
					t.Fatal("failed fact cleanup was recorded as complete")
				}
			},
			retryCalls: []string{
				"facts !target:example.org !activity:example.org !approvals:example.org target",
				"operator !target:example.org",
				"leave !target:example.org",
			},
		},
		{
			failure: "operator",
			reason:  "operator_removal_failed",
			completed: func(t *testing.T, loaded *state.State) {
				t.Helper()
				progress := loaded.Forges["/forges/one"].Projects["target"].Cleanup
				if progress == nil || !progress.FactsCleared || progress.OperatorRemoved {
					t.Fatalf("fact progress = %+v", progress)
				}
			},
			retryCalls: []string{
				"operator !target:example.org",
				"leave !target:example.org",
			},
		},
		{
			failure: "leave",
			reason:  "bridge_membership_failed",
			completed: func(t *testing.T, loaded *state.State) {
				t.Helper()
				progress := loaded.Forges["/forges/one"].Projects["target"].Cleanup
				if progress == nil || !progress.FactsCleared || !progress.OperatorRemoved || progress.BridgeLeft {
					t.Fatalf("stage progress = %+v", progress)
				}
			},
			retryCalls: []string{
				"leave !target:example.org",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.failure, func(t *testing.T) {
			loaded := projectState("/forges/one")
			path := writeRemovalState(t, loaded)
			cfg := projectConfig("/forges/one")
			catalog := fakeProjectCatalog{forgeExists: true, managed: true}
			cleaner := &fakeProjectCleaner{fail: test.failure}
			service := Service{
				StatePath:      path,
				ProjectCleaner: cleaner,
				Projects:       catalog,
				Operator:       "@operator:example.org",
				Config:         cfg,
			}
			inspection := InspectProject(cfg, loaded, "/forges/one", "target", catalog)
			if got := service.RemoveProject(context.Background(), inspection); got != refusedRetryable(test.reason) {
				t.Fatalf("first removal = %+v, want %s", got, test.reason)
			}
			failed, err := state.Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := failed.Forges["/forges/one"].Projects["target"]; !ok {
				t.Fatal("retry lost the target project record")
			}
			test.completed(t, failed)

			cleaner.fail = ""
			cleaner.calls = nil
			retry := InspectProject(cfg, failed, "/forges/one", "target", catalog)
			if got := service.RemoveProject(context.Background(), retry); got != (Result{Status: Removed}) {
				t.Fatalf("retry = %+v, want removed", got)
			}
			if !reflect.DeepEqual(cleaner.calls, test.retryCalls) {
				t.Fatalf("retry calls = %v, want %v", cleaner.calls, test.retryCalls)
			}
		})
	}
}

func TestRemoveProjectIsIdempotentAndNeverTouchesMatrixAgain(t *testing.T) {
	loaded := projectState("/forges/one")
	path := writeRemovalState(t, loaded)
	cfg := projectConfig("/forges/one")
	catalog := fakeProjectCatalog{forgeExists: true, managed: true}
	cleaner := &fakeProjectCleaner{}
	service := Service{
		StatePath: path, ProjectCleaner: cleaner, Projects: catalog,
		Operator: "@operator:example.org", Config: cfg,
	}
	inspection := InspectProject(cfg, loaded, "/forges/one", "target", catalog)
	if got := service.RemoveProject(context.Background(), inspection); got != (Result{Status: Removed}) {
		t.Fatalf("first removal = %+v", got)
	}
	cleaner.calls = nil
	final, err := state.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := service.RemoveProject(context.Background(),
		InspectProject(cfg, final, "/forges/one", "target", catalog)); got != (Result{Status: NothingToRemove}) {
		t.Fatalf("second removal = %+v", got)
	}
	if len(cleaner.calls) != 0 {
		t.Fatalf("second removal touched Matrix: %v", cleaner.calls)
	}
}

func TestRemoveProjectReportsUnavailablePrerequisites(t *testing.T) {
	cfg := projectConfig("/forges/one")
	ready := ProjectInspection{
		Result: Result{Status: Ready}, Root: "/forges/one", ProjectName: "target", RoomID: "!target:example.org",
	}
	if got := (Service{Config: cfg}).RemoveProject(context.Background(), ready); got != refusedRetryable("matrix_unavailable") {
		t.Fatalf("missing cleaner = %+v", got)
	}
	service := Service{
		StatePath:      t.TempDir(),
		ProjectCleaner: &fakeProjectCleaner{},
		Projects:       fakeProjectCatalog{forgeExists: true, managed: true},
		Config:         cfg,
	}
	if got := service.RemoveProject(context.Background(), ready); got != refusedRetryable("state_unavailable") {
		t.Fatalf("unreadable state = %+v", got)
	}
	if _, result := service.projectProgress(ready); result != refusedRetryable("state_unavailable") {
		t.Fatalf("project progress = %+v", result)
	}
	if result := service.saveProjectProgress(ready, state.RoomCleanup{}); result != refusedRetryable("state_unavailable") {
		t.Fatalf("save progress = %+v", result)
	}
	if result := service.finalizeProject(ready.Root, ready.ProjectName, ready.RoomID); result != refusedRetryable("state_unavailable") {
		t.Fatalf("finalize = %+v", result)
	}
}

func TestSaveProjectProgressRejectsAChangedRecord(t *testing.T) {
	loaded := projectState("/forges/one")
	path := writeRemovalState(t, loaded)
	cfg := projectConfig("/forges/one")
	service := Service{StatePath: path, Config: cfg}
	inspection := ProjectInspection{
		Root: "/forges/one", ProjectName: "target", RoomID: "!sibling:example.org",
	}
	if got := service.saveProjectProgress(inspection, state.RoomCleanup{}); got != refusedRetryable("state_unavailable") {
		t.Fatalf("changed record = %+v", got)
	}
}

func TestProjectStagesStopWhenProgressCannotBeRead(t *testing.T) {
	service := Service{StatePath: t.TempDir()}
	inspection := ProjectInspection{Root: "/forges/one", ProjectName: "target", RoomID: "!target:example.org"}
	stages := map[string]func() Result{
		"facts":    func() Result { return service.cleanProjectFacts(context.Background(), inspection) },
		"operator": func() Result { return service.removeProjectOperator(context.Background(), inspection) },
		"leave":    func() Result { return service.leaveProject(context.Background(), inspection) },
	}
	for name, stage := range stages {
		t.Run(name, func(t *testing.T) {
			if got := stage(); got != refusedRetryable("state_unavailable") {
				t.Fatalf("stage = %+v", got)
			}
		})
	}
}

func TestReloadProjectDoesNotDeclareUnreadableStateReady(t *testing.T) {
	service := Service{
		StatePath: t.TempDir(), ProjectCleaner: &fakeProjectCleaner{},
		Projects: fakeProjectCatalog{forgeExists: true, managed: true},
		Config:   projectConfig("/forges/one"),
	}
	if _, result, ready := service.reloadProject("/forges/one", "target"); ready ||
		result != refusedRetryable("state_unavailable") {
		t.Fatalf("unreadable reload = %+v, ready=%t", result, ready)
	}
	service.ProjectCleaner = nil
	if _, result, ready := service.reloadProject("/forges/one", "target"); ready ||
		result != refusedRetryable("matrix_unavailable") {
		t.Fatalf("missing cleaner reload = %+v, ready=%t", result, ready)
	}
}

func TestCurrentProjectRequiresTheRecordedRoom(t *testing.T) {
	loaded := projectState("/forges/one")
	inspection := ProjectInspection{Root: "/forges/one", ProjectName: "target", RoomID: "!target:example.org"}
	if _, ok := currentProject(loaded, inspection); !ok {
		t.Fatal("currentProject rejected the recorded room")
	}
	inspection.RoomID = "!sibling:example.org"
	if _, ok := currentProject(loaded, inspection); ok {
		t.Fatal("currentProject accepted a sibling room")
	}
	inspection.Root = "/forges/missing"
	if _, ok := currentProject(loaded, inspection); ok {
		t.Fatal("currentProject accepted a missing forge")
	}
}

func TestProjectProgressWritersRejectAnotherProjectRoom(t *testing.T) {
	loaded := projectState("/forges/one")
	inspection := ProjectInspection{
		Root: "/forges/one", ProjectName: "target", RoomID: "!sibling:example.org",
	}
	if recordProjectProgress(loaded, inspection, state.RoomCleanup{}) {
		t.Fatal("recordProjectProgress accepted a sibling room")
	}
	inspection.ProjectName = "missing"
	inspection.RoomID = "!target:example.org"
	if recordProjectProgress(loaded, inspection, state.RoomCleanup{}) {
		t.Fatal("recordProjectProgress accepted a missing project")
	}
	inspection.Root = "/forges/missing"
	if recordProjectProgress(loaded, inspection, state.RoomCleanup{}) {
		t.Fatal("recordProjectProgress accepted a missing forge")
	}
}

func TestRemoveProjectRecordRejectsAnotherProjectRoom(t *testing.T) {
	loaded := projectState("/forges/one")
	if result, done := removeProjectRecord(loaded, "/forges/one", "target", "!sibling:example.org"); !done ||
		result != (Result{Status: NothingToRemove}) {
		t.Fatalf("wrong target room = %+v, done=%t", result, done)
	}
	if result, done := removeProjectRecord(loaded, "/forges/one", "missing", "!target:example.org"); !done ||
		result != (Result{Status: NothingToRemove}) {
		t.Fatalf("missing project = %+v, done=%t", result, done)
	}
	if result, done := removeProjectRecord(loaded, "/forges/missing", "target", "!target:example.org"); !done ||
		result != (Result{Status: NothingToRemove}) {
		t.Fatalf("missing forge = %+v, done=%t", result, done)
	}
}

type fakeProjectCatalog struct {
	forgeExists bool
	managed     bool
	forgeErr    error
	managedErr  error
}

func (f fakeProjectCatalog) ForgeExists(string) (bool, error) {
	return f.forgeExists, f.forgeErr
}

func (f fakeProjectCatalog) ManagedProject(string, string) (bool, error) {
	return f.managed, f.managedErr
}

type fakeProjectCleaner struct {
	calls []string
	fail  string
}

func (f *fakeProjectCleaner) ClearProjectFacts(_ context.Context, projectRoomID, activityRoomID, approvalsRoomID, project string) error {
	call := "facts " + projectRoomID + " " + activityRoomID + " " + approvalsRoomID + " " + project
	f.calls = append(f.calls, call)
	if f.fail == "facts" {
		return errors.New("injected failure")
	}
	return nil
}

func (f *fakeProjectCleaner) RemoveOperator(_ context.Context, roomID, _ string) error {
	f.calls = append(f.calls, "operator "+roomID)
	if f.fail == "operator" {
		return errors.New("injected failure")
	}
	return nil
}

func (f *fakeProjectCleaner) LeaveAndForget(_ context.Context, roomID string) error {
	f.calls = append(f.calls, "leave "+roomID)
	if f.fail == "leave" {
		return errors.New("injected failure")
	}
	return nil
}

func projectConfig(roots ...string) config.Config {
	cfg := config.Config{
		HomeserverURL: "https://matrix.example.org",
		UserID:        "@bridge:example.org",
		Operator:      "@operator:example.org",
	}
	for _, root := range roots {
		cfg.Forges = append(cfg.Forges, config.Forge{Root: root})
	}
	return cfg
}

func projectState(root string) *state.State {
	loaded := &state.State{
		Forges: map[string]state.Forge{
			root: {
				SpaceID:              "!space:example.org",
				RoomID:               "!chat:example.org",
				ApprovalsRoomID:      "!approvals:example.org",
				ActivityRoomID:       "!activity:example.org",
				ClarificationsRoomID: "!clarifications:example.org",
				Projects: map[string]state.Project{
					"target":  {RoomID: "!target:example.org"},
					"sibling": {RoomID: "!sibling:example.org"},
				},
			},
		},
	}
	loaded.EnsureMaps()
	loaded.Relay.Waiting["!target:example.org\n"+relay.BoardFactType+"\ntarget"] = `{"projects":[]}`
	loaded.Relay.Waiting["!sibling:example.org\n"+relay.BoardFactType+"\nsibling"] = `{"projects":[]}`
	loaded.Relay.Waiting["!activity:example.org\n"+relay.BoardFactType+"\ntarget"] = `{"projects":[]}`
	loaded.Relay.Waiting["!activity:example.org\n"+relay.BoardFactType+"\nsibling"] = `{"projects":[]}`
	loaded.Relay.Approvals["target/approval"] = relay.ApprovalState{
		ForgeRoot: root, RoomID: "!approvals:example.org", MessageID: "$target-approval",
	}
	loaded.Relay.Approvals["sibling/approval"] = relay.ApprovalState{
		ForgeRoot: root, RoomID: "!approvals:example.org", MessageID: "$sibling-approval",
	}
	loaded.Relay.Approvals["forge/approval"] = relay.ApprovalState{
		ForgeRoot: root, RoomID: "!approvals:example.org", MessageID: "$forge-approval",
	}
	loaded.Relay.Clarifications["target/clarification"] = relay.ClarificationState{
		ForgeRoot: root, RoomID: "!target:example.org", MessageID: "$target-clarification",
	}
	loaded.Relay.Clarifications["sibling/clarification"] = relay.ClarificationState{
		ForgeRoot: root, RoomID: "!sibling:example.org", MessageID: "$sibling-clarification",
	}
	loaded.Relay.Clarifications["forge/clarification"] = relay.ClarificationState{
		ForgeRoot: root, RoomID: "!clarifications:example.org", MessageID: "$forge-clarification",
	}
	loaded.Relay.Clarifications["orphan-in-target-room"] = relay.ClarificationState{
		ForgeRoot: "/forges/other", RoomID: "!target:example.org", MessageID: "$orphan-room",
	}
	loaded.Relay.Clarifications["target/orphan-in-sibling-room"] = relay.ClarificationState{
		ForgeRoot: root, RoomID: "!sibling:example.org", MessageID: "$orphan-key",
	}
	loaded.Relay.Activity["target/card"] = relay.CardState{ForgeRoot: root, Project: "target"}
	loaded.Relay.Activity["sibling/card"] = relay.CardState{ForgeRoot: root, Project: "sibling"}
	loaded.Relay.Threads["target-request"] = "$target-message"
	loaded.Relay.Threads["sibling-request"] = "$sibling-message"
	loaded.Relay.Threads["forge-request"] = "$forge-message"
	loaded.Relay.ScopeProject(relay.ScopeThreads, "target-request", root, "target")
	loaded.Relay.ScopeProject(relay.ScopeThreads, "sibling-request", root, "sibling")
	loaded.Relay.Scope(relay.ScopeThreads, "forge-request", root)
	return loaded
}
