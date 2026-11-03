package removal

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/unclebob/forgelet-bridge/internal/config"
	"github.com/unclebob/forgelet-bridge/internal/relay"
	"github.com/unclebob/forgelet-bridge/internal/state"
)

func TestInspectRefusesUnsafeUnknownAndAmbiguousRoots(t *testing.T) {
	cfg := removalConfig("/forges/one", "/teams/two/one")
	loaded := removalState("/forges/one")

	tests := []struct {
		name string
		root string
		want Result
	}{
		{name: "blank", root: "", want: refused("unsafe_root")},
		{name: "relative", root: "one", want: refused("ambiguous_root")},
		{name: "relative with no basename match", root: "missing", want: refused("unsafe_root")},
		{name: "parent traversal", root: "/forges/../forges/one", want: refused("unsafe_root")},
		{name: "unknown absolute", root: "/forges/three", want: refused("unknown_root")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := Inspect(cfg, loaded, test.root)
			if got.Result != test.want {
				t.Fatalf("Inspect(%q) = %+v, want %+v", test.root, got.Result, test.want)
			}
			if got.Root != "" || got.Forge != nil || len(got.Rooms) != 0 {
				t.Fatalf("refused inspection exposed removable work: %+v", got)
			}
		})
	}
}

func TestInspectDistinguishesNoRecordFromIncompleteRecord(t *testing.T) {
	cfg := removalConfig("/forges/one")

	empty := removalState("/forges/one")
	delete(empty.Forges, "/forges/one")
	if got := Inspect(cfg, empty, "/forges/one"); got.Result != (Result{Status: NothingToRemove}) {
		t.Fatalf("missing record = %+v, want nothing_to_remove", got.Result)
	}

	incompleteFields := []struct {
		name   string
		remove func(*state.Forge)
	}{
		{name: "space", remove: func(forge *state.Forge) { forge.SpaceID = "" }},
		{name: "chat", remove: func(forge *state.Forge) { forge.RoomID = "" }},
		{name: "approvals", remove: func(forge *state.Forge) { forge.ApprovalsRoomID = "" }},
		{name: "activity", remove: func(forge *state.Forge) { forge.ActivityRoomID = "" }},
		{name: "clarifications", remove: func(forge *state.Forge) { forge.ClarificationsRoomID = "" }},
	}
	for _, field := range incompleteFields {
		t.Run("missing "+field.name, func(t *testing.T) {
			incomplete := removalState("/forges/one")
			forge := incomplete.Forges["/forges/one"]
			field.remove(&forge)
			incomplete.Forges["/forges/one"] = forge
			if got := Inspect(cfg, incomplete, "/forges/one"); got.Result != refused("incomplete_state") {
				t.Fatalf("incomplete record = %+v, want incomplete_state", got.Result)
			}
		})
	}
}

func TestInspectPlansEveryDistinctRoomOnce(t *testing.T) {
	cfg := removalConfig("/forges/one")
	loaded := removalState("/forges/one")
	forge := loaded.Forges["/forges/one"]
	forge.Projects = map[string]state.Project{
		"second": {RoomID: "!project-2:example.org"},
		"first":  {RoomID: "!project-1:example.org"},
		"same":   {RoomID: "!project-1:example.org"},
	}
	loaded.Forges["/forges/one"] = forge

	got := Inspect(cfg, loaded, "/forges/one")
	want := []string{
		"!space:example.org",
		"!chat:example.org",
		"!approvals:example.org",
		"!activity:example.org",
		"!clarifications:example.org",
		"!project-1:example.org",
		"!project-2:example.org",
	}
	if got.Result != (Result{Status: Ready}) {
		t.Fatalf("Inspect = %+v, want ready", got.Result)
	}
	if !reflect.DeepEqual(got.Rooms, want) {
		t.Fatalf("rooms = %v, want %v", got.Rooms, want)
	}
}

func TestRemoveCleansEachRoomInOrderAndKeepsProgress(t *testing.T) {
	loaded := removalState("/forges/one")
	forge := loaded.Forges["/forges/one"]
	forge.Projects = map[string]state.Project{"project": {RoomID: "!project:example.org"}}
	loaded.Forges["/forges/one"] = forge
	path := writeRemovalState(t, loaded)
	cfg := removalConfig("/forges/one")

	cleaner := &fakeCleaner{fail: map[string]error{
		"facts !project:example.org": errors.New("homeserver said no"),
	}}
	service := Service{
		StatePath: path,
		Cleaner:   cleaner,
		Operator:  "@operator:example.org",
		Config:    cfg,
	}

	got := service.Remove(context.Background(), Inspect(cfg, loaded, "/forges/one"))
	if got != refusedRetryable("relay_cleanup_failed") {
		t.Fatalf("first removal = %+v, want retryable relay_cleanup_failed", got)
	}
	if !containsCall(cleaner.calls, "facts !space:example.org") ||
		!containsCall(cleaner.calls, "leave !space:example.org") ||
		!containsCall(cleaner.calls, "facts !project:example.org") {
		t.Fatalf("calls = %v, want the space completed before the project tried", cleaner.calls)
	}
	facts := callIndex(cleaner.calls, "facts !space:example.org")
	operator := callIndex(cleaner.calls, "operator !space:example.org")
	leave := callIndex(cleaner.calls, "leave !space:example.org")
	if !(facts < operator && operator < leave) {
		t.Fatalf("space order = facts:%d operator:%d leave:%d, want that order", facts, operator, leave)
	}

	saved, err := state.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	savedForge := saved.Forges["/forges/one"]
	if savedForge.SpaceID == "" || savedForge.Projects["project"].RoomID == "" {
		t.Fatalf("a retry lost the room ids: %+v", savedForge)
	}
	if !savedForge.Cleanup.Rooms["!space:example.org"].Done() {
		t.Fatalf("completed space progress was not saved: %+v", savedForge.Cleanup)
	}
	if savedForge.Cleanup.Rooms["!project:example.org"].FactsCleared {
		t.Fatalf("a failed fact cleanup was recorded as complete: %+v", savedForge.Cleanup)
	}

	cleaner.fail = nil
	cleaner.calls = nil
	got = service.Remove(context.Background(), Inspect(cfg, saved, "/forges/one"))
	if got != (Result{Status: Removed}) {
		t.Fatalf("retry = %+v, want removed", got)
	}
	for _, call := range cleaner.calls {
		if strings.HasSuffix(call, "!space:example.org") {
			t.Fatalf("retry targeted the completed space again: %v", cleaner.calls)
		}
	}
	if !containsCall(cleaner.calls, "facts !project:example.org") {
		t.Fatalf("retry calls = %v, want it to resume at the failed project", cleaner.calls)
	}

	final, err := state.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := final.Forges["/forges/one"]; ok {
		t.Fatalf("successful removal retained the forge record: %+v", final.Forges)
	}
	if _, ok := final.Relay.Waiting["!chat:example.org\n"+relay.BoardFactType+"\nproject"]; ok {
		t.Fatalf("successful removal retained relay waiting facts: %+v", final.Relay.Waiting)
	}
	if final.Relay.Waiting["!other:example.org\n"+relay.BoardFactType+"\nother"] == "" {
		t.Fatalf("successful removal cleared another forge's relay facts: %+v", final.Relay.Waiting)
	}
	if _, ok := final.Relay.Approvals["mine"]; ok {
		t.Fatalf("successful removal retained this forge's approval: %+v", final.Relay.Approvals)
	}
	if _, ok := final.Relay.Approvals["other"]; !ok {
		t.Fatalf("successful removal cleared another forge's approval: %+v", final.Relay.Approvals)
	}
	if _, ok := final.Relay.Activity["mine"]; ok {
		t.Fatalf("successful removal retained this forge's activity: %+v", final.Relay.Activity)
	}
	if _, ok := final.Relay.Activity["other"]; !ok {
		t.Fatalf("successful removal cleared another forge's activity with the same project name: %+v", final.Relay.Activity)
	}
	if _, ok := final.Relay.Threads["mine"]; ok {
		t.Fatalf("successful removal retained this forge's scoped thread: %+v", final.Relay.Threads)
	}
	if _, ok := final.Relay.Threads["other"]; !ok {
		t.Fatalf("successful removal cleared another forge's scoped thread: %+v", final.Relay.Threads)
	}
}

func TestRemoveReturnsSanitizedRetryReason(t *testing.T) {
	loaded := removalState("/forges/one")
	path := writeRemovalState(t, loaded)
	service := Service{
		StatePath: path,
		Operator:  "@operator:example.org",
		Config:    removalConfig("/forges/one"),
		Cleaner: &fakeCleaner{fail: map[string]error{
			"operator !space:example.org": errors.New("failed for @operator:example.org at /forges/one"),
		}},
	}
	got := service.Remove(context.Background(), Inspect(removalConfig("/forges/one"), loaded, "/forges/one"))
	data, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"status":"retryable","reason":"operator_removal_failed"}` {
		t.Fatalf("result JSON = %s, want a stable sanitized reason", data)
	}
}

func TestRefusedAndNothingToRemoveNeverTouchMatrix(t *testing.T) {
	cfg := removalConfig("/forges/one")
	loaded := removalState("/forges/one")
	path := writeRemovalState(t, loaded)
	cleaner := &fakeCleaner{}
	service := Service{
		StatePath: path,
		Cleaner:   cleaner,
		Operator:  "@operator:example.org",
		Config:    cfg,
	}

	for _, inspection := range []Inspection{
		Inspect(cfg, loaded, "/forges/unknown"),
		Inspect(cfg, stateWithNoForge(), "/forges/one"),
	} {
		_ = service.Remove(context.Background(), inspection)
	}
	if len(cleaner.calls) != 0 {
		t.Fatalf("refused or already removed forge touched Matrix: %v", cleaner.calls)
	}
}

func removalConfig(roots ...string) config.Config {
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

func removalState(root string) *state.State {
	loaded := &state.State{
		Forges: map[string]state.Forge{
			root: {
				SpaceID:              "!space:example.org",
				RoomID:               "!chat:example.org",
				ApprovalsRoomID:      "!approvals:example.org",
				ActivityRoomID:       "!activity:example.org",
				ClarificationsRoomID: "!clarifications:example.org",
				Projects: map[string]state.Project{
					"project": {RoomID: "!project:example.org"},
				},
			},
			"/forges/other": {
				SpaceID:              "!other-space:example.org",
				RoomID:               "!other:example.org",
				ApprovalsRoomID:      "!other-approvals:example.org",
				ActivityRoomID:       "!other-activity:example.org",
				ClarificationsRoomID: "!other-clarifications:example.org",
			},
		},
	}
	loaded.EnsureMaps()
	loaded.Relay.Waiting["!chat:example.org\n"+relay.BoardFactType+"\nproject"] = `{"projects":[]}`
	loaded.Relay.Waiting["!other:example.org\n"+relay.BoardFactType+"\nother"] = `{"projects":[]}`
	loaded.Relay.Approvals["mine"] = relay.ApprovalState{RoomID: "!approvals:example.org", MessageID: "$mine"}
	loaded.Relay.Approvals["other"] = relay.ApprovalState{RoomID: "!other-approvals:example.org", MessageID: "$other"}
	loaded.Relay.Activity["mine"] = relay.CardState{ForgeRoot: root, Project: "project"}
	loaded.Relay.Activity["other"] = relay.CardState{ForgeRoot: "/forges/other", Project: "project"}
	loaded.Relay.Threads["mine"] = "$mine"
	loaded.Relay.Threads["other"] = "$other"
	loaded.Relay.Scope(relay.ScopeThreads, "mine", root)
	loaded.Relay.Scope(relay.ScopeThreads, "other", "/forges/other")
	return loaded
}

func stateWithNoForge() *state.State {
	loaded := removalState("/forges/one")
	delete(loaded.Forges, "/forges/one")
	return loaded
}

func writeRemovalState(t *testing.T, loaded *state.State) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bridge-state.json")
	if err := loaded.Save(path); err != nil {
		t.Fatal(err)
	}
	return path
}

func containsCall(calls []string, want string) bool {
	return callIndex(calls, want) >= 0
}

func callIndex(calls []string, want string) int {
	for index, call := range calls {
		if call == want {
			return index
		}
	}
	return -1
}

func refused(reason string) Result {
	return Result{Status: Refused, Reason: reason}
}

func refusedRetryable(reason string) Result {
	return Result{Status: Retryable, Reason: reason}
}

type fakeCleaner struct {
	calls []string
	fail  map[string]error
}

func (f *fakeCleaner) ClearFacts(_ context.Context, roomID string) error {
	return f.record("facts", roomID)
}

func (f *fakeCleaner) RemoveOperator(_ context.Context, roomID, _ string) error {
	return f.record("operator", roomID)
}

func (f *fakeCleaner) LeaveAndForget(_ context.Context, roomID string) error {
	return f.record("leave", roomID)
}

func (f *fakeCleaner) record(operation, roomID string) error {
	call := operation + " " + roomID
	f.calls = append(f.calls, call)
	return f.fail[call]
}

func TestRemovalResultContainsNoPathsIDsOrCredentials(t *testing.T) {
	result := Result{Status: Retryable, Reason: "relay_cleanup_failed"}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"/forges/", "!", "@", "token"} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("result %s contains %q", data, secret)
		}
	}
}

func TestWriteResultUsesOneJSONObject(t *testing.T) {
	var output strings.Builder
	if err := WriteResult(&output, Result{Status: Removed}); err != nil {
		t.Fatal(err)
	}
	if output.String() != "{\"status\":\"removed\"}\n" {
		t.Fatalf("output = %q", output.String())
	}
}

func TestSplitWaitingRequiresAndReturnsAllThreeParts(t *testing.T) {
	room, eventType, key, ok := splitWaiting("!room:example.org\nevent.type\nkey")
	if !ok || room != "!room:example.org" || eventType != "event.type" || key != "key" {
		t.Fatalf("split = room:%q type:%q key:%q ok:%t", room, eventType, key, ok)
	}
	if _, _, _, ok := splitWaiting("!room:example.org\nevent.type"); ok {
		t.Fatal("a waiting key with only two parts was accepted")
	}
}

func TestMissingStateIsAnIncompleteStateNotAFilesystemError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	service := Service{
		StatePath: path,
		Cleaner:   &fakeCleaner{},
		Operator:  "@operator:example.org",
		Config:    removalConfig("/forges/one"),
	}
	got, err := service.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("missing state loaded as nil")
	}
	result := service.Remove(context.Background(), Inspect(removalConfig("/forges/one"), got, "/forges/one"))
	if result != (Result{Status: NothingToRemove}) {
		t.Fatalf("missing state = %+v, want nothing_to_remove", result)
	}
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Load(); err == nil {
		t.Fatal("malformed state loaded without an error")
	}
}
