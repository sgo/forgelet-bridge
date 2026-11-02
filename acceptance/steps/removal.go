package steps

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/internal/relay"
	"github.com/unclebob/forgelet-bridge/internal/state"
)

const removalObserver = "@observer:example.org"

type removalScenario struct {
	status   string
	reason   string
	target   string
	rooms    []string
	project  string
	projects map[string]string

	stateBefore   []byte
	configBefore  []byte
	targetBefore  map[string]string
	roomsBefore   []string
	operationDone bool
}

type removalReport struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

func configureAdapterForges(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	served, err := w.declaredForge(captures[1])
	if err != nil {
		return err
	}
	names := forgeNames(captures[2])
	if len(names) == 0 {
		return fmt.Errorf("the step named no forge roots")
	}
	for _, name := range names {
		if _, err := w.declaredForge(name); err != nil {
			return err
		}
		if err := w.configureForge(name); err != nil {
			return err
		}
	}
	if _, err := w.operator(ctx); err != nil {
		return err
	}

	cfg := map[string]any{
		"homeserver_url": w.synapse.URL,
		"user_id":        w.bridgeUserID,
		"password":       w.bridgePassword,
		"operator":       w.operatorID,
		"forges":         []map[string]string{},
		"state_dir":      filepath.Join(served.Root(), ".swarmforge", "matrix-bridge"),
	}
	for _, root := range w.configured {
		cfg["forges"] = append(cfg["forges"].([]map[string]string), map[string]string{"root": root})
	}
	body, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	root := served.Root()
	path := w.bridgeConfigOf(root)
	if err := os.WriteFile(path, append(body, '\n'), 0o644); err != nil {
		return err
	}
	w.servedRoot = root
	w.stateDir = filepath.Join(root, ".swarmforge", "matrix-bridge")
	w.configPath = path
	return nil
}

func bridgeRecordedProjectRooms(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	count := 0
	if _, err := fmt.Sscanf(captures[1], "%d", &count); err != nil || count < 1 {
		return fmt.Errorf("project room count %q is invalid", captures[1])
	}
	target, err := w.declaredForge(captures[2])
	if err != nil {
		return err
	}
	targetRoot := target.Root()

	loaded, err := w.waitForRemovalForge(ctx, targetRoot)
	if err != nil {
		return err
	}
	// Seeding project ids and facts is deterministic only while the bridge is
	// not writing its own bookkeeping. It has been started by the background,
	// and is stopped here before the fixture supplies project rooms.
	w.stopAdapterBridge()

	tracked := loaded.Forges[targetRoot]
	if tracked.Projects == nil {
		tracked.Projects = map[string]state.Project{}
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	synapse, err := w.homeserver(ctx)
	if err != nil {
		return err
	}
	bridgeClient, err := mautrix.NewClient(synapse.URL, id.UserID(w.bridgeUserID), w.bridgeToken)
	if err != nil {
		return err
	}
	observer, err := w.user(ctx, removalObserver)
	if err != nil {
		return err
	}

	scenario := &removalScenario{
		target:        targetRoot,
		project:       "removal-project-1",
		rooms:         removalOrderedRooms(tracked),
		projects:      map[string]string{},
		stateBefore:   nil,
		configBefore:  nil,
		operationDone: false,
	}
	for _, roomID := range scenario.rooms {
		if _, err := bridgeClient.InviteUser(ctx, id.RoomID(roomID), &mautrix.ReqInviteUser{UserID: id.UserID(removalObserver)}); err == nil {
			if _, err := observer.Client().JoinRoomByID(ctx, id.RoomID(roomID)); err != nil {
				return fmt.Errorf("observe removal forge room %s: %w", roomID, err)
			}
		}
	}
	for index := 1; index <= count; index++ {
		project := fmt.Sprintf("removal-project-%d", index)
		if index == 1 {
			scenario.project = project
		}
		response, err := operator.Client().CreateRoom(ctx, &mautrix.ReqCreateRoom{
			Name:         "Project " + project,
			Preset:       "private_chat",
			Invite:       []id.UserID{id.UserID(w.bridgeUserID), id.UserID(removalObserver)},
			InitialState: removalPowerLevels(w.bridgeUserID, w.operatorID, false),
		})
		if err != nil {
			return fmt.Errorf("create removal project room %s: %w", project, err)
		}
		if _, err := bridgeClient.JoinRoomByID(ctx, response.RoomID); err != nil {
			return fmt.Errorf("join removal project room %s: %w", project, err)
		}
		if _, err := observer.Client().JoinRoomByID(ctx, response.RoomID); err != nil {
			return fmt.Errorf("observe removal project room %s: %w", project, err)
		}
		if _, err := bridgeClient.SendStateEvent(ctx, id.RoomID(tracked.SpaceID), event.StateSpaceChild,
			response.RoomID.String(), map[string]any{"via": []string{fixtures.ServerName}}); err != nil {
			return fmt.Errorf("put removal project room %s in the forge space: %w", project, err)
		}
		tracked.Projects[project] = state.Project{RoomID: response.RoomID.String()}
		scenario.rooms = append(scenario.rooms, response.RoomID.String())
		scenario.projects[project] = response.RoomID.String()
	}

	loaded.Forges[targetRoot] = tracked
	loaded.Relay.EnsureMaps()
	for _, roomID := range scenario.rooms {
		loaded.Relay.Waiting[roomID+"\n"+relay.BoardFactType+"\nremoval"] = `{"projects":[]}`
	}
	loaded.Relay.Approvals["removal/approval"] = relay.ApprovalState{
		RoomID: tracked.ApprovalsRoomID, MessageID: "$removal-approval",
	}
	loaded.Relay.Clarifications["removal/clarification"] = relay.ClarificationState{
		RoomID: tracked.ClarificationsRoomID, MessageID: "$removal-clarification",
	}
	loaded.Relay.Activity["removal/card"] = relay.CardState{
		ForgeRoot: targetRoot, Project: scenario.project, Name: "removal-card", Lane: "coder",
	}
	loaded.Relay.Threads["removal/request"] = "$removal-message"
	loaded.Relay.Scope(relay.ScopeThreads, "removal/request", targetRoot)
	loaded.Relay.Threads["kept/request"] = "$kept-message"
	loaded.Relay.Scope(relay.ScopeThreads, "kept/request", w.configured[0])
	if err := loaded.Save(w.removalStatePath()); err != nil {
		return err
	}
	w.removal = scenario
	return nil
}

func forgeHoldsRemovalRelayFacts(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	target, err := w.declaredForge(captures[1])
	if err != nil {
		return err
	}
	if w.removal == nil || w.removal.target != target.Root() {
		return fmt.Errorf("the bridge has not recorded project rooms for %s", captures[1])
	}
	synapse, err := w.homeserver(ctx)
	if err != nil {
		return err
	}
	bridgeClient, err := mautrix.NewClient(synapse.URL, id.UserID(w.bridgeUserID), w.bridgeToken)
	if err != nil {
		return err
	}
	for _, roomID := range w.removal.rooms {
		if _, err := bridgeClient.SendStateEvent(ctx, id.RoomID(roomID),
			event.Type{Type: relay.BoardFactType, Class: event.StateEventType}, "removal",
			map[string]any{"room": roomID}); err != nil {
			return fmt.Errorf("seed relay fact in %s: %w", roomID, err)
		}
	}
	return nil
}

func matrixRefusesRemovalStep(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	if w.removal == nil || w.removal.project == "" {
		return fmt.Errorf("no recorded project room is available for the failure")
	}
	projectRoom, err := w.removalProjectRoom()
	if err != nil {
		return err
	}
	reason := map[string]string{
		"clearing the relay facts":                    "relay_cleanup_failed",
		"removing the operator":                       "operator_removal_failed",
		"leaving or forgetting the bridge membership": "bridge_membership_failed",
	}[captures[1]]
	if reason == "" {
		return fmt.Errorf("unknown removal failure %q", captures[1])
	}
	w.adapterEnv = append(w.adapterEnv, "FORGELET_BRIDGE_REMOVE_TEST_FAILURE="+reason+"="+projectRoom)
	return nil
}

func adapterRemovesForge(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	target, err := w.declaredForge(captures[2])
	if err != nil {
		return err
	}
	return w.invokeRemoval(captures[1], target.Root())
}

func adapterAskedToRemoveForge(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	served, err := w.declaredForge(captures[1])
	if err != nil {
		return err
	}
	target, err := w.declaredForge("forge-b")
	if err != nil {
		return err
	}
	argument := target.Root()
	switch captures[2] {
	case "blank":
		argument = ""
	case "parent-traversal":
		argument = filepath.Dir(target.Root()) + string(os.PathSeparator) + ".." + string(os.PathSeparator) + filepath.Base(target.Root())
	case "relative":
		argument = filepath.Base(target.Root())
	case "basename shared by two configured forges":
		argument = filepath.Base(target.Root())
		if err := w.duplicateConfiguredBasename(served.Root(), argument); err != nil {
			return err
		}
	case "extra":
		return w.invokeRemovalArgs(captures[1], target.Root(), "extra")
	default:
		return fmt.Errorf("unknown removal argument case %q", captures[2])
	}
	if captures[2] == "blank" {
		return w.invokeRemovalArgs(captures[1], argument)
	}
	return w.invokeRemovalArgs(captures[1], argument)
}

func removalResultReports(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	if w.removal == nil || !w.removal.operationDone {
		return fmt.Errorf("the adapter has not answered a removal request")
	}
	if w.removal.status != captures[1] {
		return fmt.Errorf("removal status = %q, want %q; output: %s", w.removal.status, captures[1], w.adapterOutput)
	}
	wantReason := ""
	if len(captures) > 2 {
		wantReason = captures[2]
	}
	if w.removal.reason != wantReason {
		return fmt.Errorf("removal reason = %q, want %q; output: %s", w.removal.reason, wantReason, w.adapterOutput)
	}
	return nil
}

func removalTargetedEveryRoom(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	if w.removal == nil || !sameStrings(w.removal.roomsBefore, w.removal.rooms) {
		return fmt.Errorf("removal room plan changed: before=%v after=%v", w.removalRoomsBefore(), w.removal.rooms)
	}
	return nil
}

func removalClearedFacts(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	if w.removal == nil {
		return fmt.Errorf("no removal has run")
	}
	observer, err := w.user(context.Background(), removalObserver)
	if err != nil {
		return err
	}
	for _, roomID := range w.removal.rooms {
		state, err := observer.Client().State(context.Background(), id.RoomID(roomID))
		if err != nil {
			return err
		}
		for eventType, byKey := range state {
			if !strings.HasPrefix(eventType.Type, "com.forgelet.") {
				continue
			}
			for stateKey, current := range byKey {
				if current != nil && len(current.Content.Raw) != 0 {
					return fmt.Errorf("relay fact %s/%s remains in %s", eventType.Type, stateKey, roomID)
				}
			}
		}
	}
	return nil
}

func removalRemovedOperator(_ context.Context, world any, _ []string) error {
	return nil
}

func removalMembershipsGone(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	if w.removal == nil {
		return fmt.Errorf("no removal has run")
	}
	observer, err := w.user(context.Background(), removalObserver)
	if err != nil {
		return err
	}
	for _, roomID := range w.removal.rooms {
		for _, userID := range []string{w.operatorID, w.bridgeUserID} {
			membership, err := observer.Membership(context.Background(), roomID, userID)
			if err != nil {
				return fmt.Errorf("read %s membership in the removed %s room: %w", userID, captures[1], err)
			}
			if membership == string(event.MembershipJoin) || membership == string(event.MembershipInvite) {
				return fmt.Errorf("%s is still %s in removed room %s", userID, membership, roomID)
			}
		}
	}
	return nil
}

func removalRelayBookkeepingCleared(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	target, err := w.declaredForge(captures[1])
	if err != nil {
		return err
	}
	loaded, err := state.Load(w.removalStatePath())
	if err != nil {
		return err
	}
	if _, ok := loaded.Forges[target.Root()]; ok {
		return fmt.Errorf("relay bookkeeping was finalized before the forge record was removed")
	}
	for key := range loaded.Relay.Waiting {
		if strings.Contains(key, "/"+captures[1]) {
			return fmt.Errorf("relay waiting fact %q remains for %s", key, captures[1])
		}
	}
	if _, ok := loaded.Relay.Threads["removal/request"]; ok {
		return fmt.Errorf("removed forge's scoped relay thread remains")
	}
	if _, ok := loaded.Relay.Threads["kept/request"]; !ok {
		return fmt.Errorf("another forge's scoped relay thread was removed")
	}
	return nil
}

func removalStateHasNoForge(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	target, err := w.declaredForge(captures[1])
	if err != nil {
		return err
	}
	loaded, err := state.Load(w.removalStatePath())
	if err != nil {
		return err
	}
	if forge, ok := loaded.Forges[target.Root()]; ok {
		return fmt.Errorf("forge state remains after removal: %+v", forge)
	}
	return nil
}

func removalStateKeepsForge(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	kept, err := w.declaredForge(captures[1])
	if err != nil {
		return err
	}
	loaded, err := state.Load(w.removalStatePath())
	if err != nil {
		return err
	}
	forge, ok := loaded.Forges[kept.Root()]
	if !ok || forge.SpaceID == "" || forge.RoomID == "" {
		return fmt.Errorf("removal lost the other forge's state: %+v", forge)
	}
	return nil
}

func removalConfigurationUnchanged(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	if w.removal == nil || w.removal.configBefore == nil {
		return fmt.Errorf("no configuration snapshot was taken")
	}
	after, err := os.ReadFile(w.configPath)
	if err != nil {
		return err
	}
	if string(after) != string(w.removal.configBefore) {
		return fmt.Errorf("the bridge configuration changed")
	}
	return nil
}

func removalFilesUnchanged(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	if w.removal == nil || w.removal.targetBefore == nil {
		return fmt.Errorf("no forge file snapshot was taken")
	}
	after, err := snapshotRemovalTree(w.removal.target)
	if err != nil {
		return err
	}
	if !sameSnapshot(w.removal.targetBefore, after) {
		return fmt.Errorf("the %s files changed", captures[1])
	}
	return nil
}

func removalResultSanitized(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	output := w.adapterStdout
	for _, forbidden := range []string{"!", "@", w.removal.target, w.bridgeToken} {
		if forbidden != "" && strings.Contains(output, forbidden) {
			return fmt.Errorf("the removal result contains %q: %s", forbidden, output)
		}
	}
	return nil
}

func removalStateRetainsRooms(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	target, err := w.declaredForge(captures[1])
	if err != nil {
		return err
	}
	loaded, err := state.Load(w.removalStatePath())
	if err != nil {
		return err
	}
	forge, ok := loaded.Forges[target.Root()]
	if !ok {
		return fmt.Errorf("retry lost the forge record")
	}
	if forge.SpaceID == "" || forge.RoomID == "" || forge.ApprovalsRoomID == "" ||
		forge.ActivityRoomID == "" || forge.ClarificationsRoomID == "" {
		return fmt.Errorf("retry lost a forge room id: %+v", forge)
	}
	for name, project := range w.removal.projects {
		if forge.Projects[name].RoomID != project {
			return fmt.Errorf("retry lost project %s room id %s", name, project)
		}
	}
	return nil
}

func removalStateRetainsProgress(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	loaded, err := state.Load(w.removalStatePath())
	if err != nil {
		return err
	}
	forge := loaded.Forges[w.removal.target]
	completed := 0
	for _, progress := range forge.Cleanup.Rooms {
		if progress.Done() {
			completed++
		}
	}
	if completed == 0 {
		return fmt.Errorf("retry progress saved no completed room")
	}
	return nil
}

func forgeRepairsRemovalStep(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	w.adapterEnv = nil
	return nil
}

func repeatedRemovalTouchedNoRoom(_ context.Context, _ any, _ []string) error {
	return nil
}

func repeatedRemovalChangedNothing(_ context.Context, world any, captures []string) error {
	if err := removalConfigurationUnchanged(context.Background(), world, captures); err != nil {
		return err
	}
	if err := removalFilesUnchanged(context.Background(), world, captures); err != nil {
		return err
	}
	w := world.(*World)
	after, err := os.ReadFile(w.removalStatePath())
	if err != nil {
		return err
	}
	if string(after) != string(w.removal.stateBefore) {
		return fmt.Errorf("the repeated removal changed bridge state")
	}
	return nil
}

func refusedRemovalTouchedNoRoom(_ context.Context, _ any, _ []string) error {
	return nil
}

func refusedRemovalChangedNothing(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	afterState, err := os.ReadFile(w.removalStatePath())
	if err != nil {
		return err
	}
	afterConfig, err := os.ReadFile(w.configPath)
	if err != nil {
		return err
	}
	if string(afterState) != string(w.removal.stateBefore) || string(afterConfig) != string(w.removal.configBefore) {
		return fmt.Errorf("a refused removal changed bridge-owned files")
	}
	for _, root := range w.forgeRoots {
		before := w.removal.targetBefore
		if root != w.removal.target && !strings.Contains(root, "forge-b") {
			continue
		}
		after, err := snapshotRemovalTree(root)
		if err != nil {
			return err
		}
		if before != nil && !sameSnapshot(before, after) {
			return fmt.Errorf("a refused removal changed the files under %s", root)
		}
	}
	return nil
}

func removalStateMissingActivityRoom(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	target, err := w.declaredForge(captures[1])
	if err != nil {
		return err
	}
	loaded, err := state.Load(w.removalStatePath())
	if err != nil {
		return err
	}
	forge := loaded.Forges[target.Root()]
	forge.ActivityRoomID = ""
	loaded.Forges[target.Root()] = forge
	return loaded.Save(w.removalStatePath())
}

func (w *World) waitForRemovalForge(ctx context.Context, root string) (*state.State, error) {
	var loaded *state.State
	err := waitFor(ctx, "the bridge never recorded the forge's rooms", func() (bool, error) {
		var err error
		loaded, err = state.Load(w.removalStatePath())
		if err != nil {
			return false, err
		}
		forge, ok := loaded.Forges[root]
		return ok && forge.SpaceID != "" && forge.RoomID != "" && forge.ApprovalsRoomID != "" &&
			forge.ActivityRoomID != "" && forge.ClarificationsRoomID != "", nil
	})
	return loaded, err
}

func (w *World) invokeRemoval(servedName, targetRoot string) error {
	return w.invokeRemovalArgs(servedName, targetRoot)
}

func (w *World) invokeRemovalArgs(servedName string, args ...string) error {
	ctx, cancel := stepContext()
	defer cancel()
	served, err := w.declaredForge(servedName)
	if err != nil {
		return err
	}
	if w.removal == nil {
		target, err := w.declaredForge("forge-b")
		if err != nil {
			return err
		}
		w.removal = &removalScenario{target: target.Root()}
	}
	w.removal.stateBefore, err = os.ReadFile(w.removalStatePath())
	if err != nil {
		return err
	}
	w.removal.configBefore, err = os.ReadFile(w.bridgeConfigOf(served.Root()))
	if err != nil {
		return err
	}
	w.removal.targetBefore, err = snapshotRemovalTree(w.removal.target)
	if err != nil {
		return err
	}
	w.removal.roomsBefore = append([]string(nil), w.removal.rooms...)
	w.removal.operationDone = false

	_, runErr := w.adapterCommand(ctx, append([]string{"remove-forge"}, args...)...)
	if runErr != nil {
		return runErr
	}
	var report removalReport
	if err := json.Unmarshal([]byte(strings.TrimSpace(w.adapterStdout)), &report); err != nil {
		return fmt.Errorf("removal did not return one JSON object: %q: %w", w.adapterStdout, err)
	}
	w.removal.status = report.Status
	w.removal.reason = report.Reason
	w.removal.operationDone = true
	return nil
}

func (w *World) removalStatePath() string {
	return filepath.Join(w.stateDir, "bridge-state.json")
}

func (w *World) removalProjectRoom() (string, error) {
	if w.removal == nil {
		return "", fmt.Errorf("no removal scenario")
	}
	loaded, err := state.Load(w.removalStatePath())
	if err != nil {
		return "", err
	}
	forge := loaded.Forges[w.removal.target]
	project, ok := forge.Projects[w.removal.project]
	if !ok || project.RoomID == "" {
		return "", fmt.Errorf("the removal scenario has no recorded project room")
	}
	return project.RoomID, nil
}

func (w *World) removalRoomsBefore() []string {
	if w.removal == nil {
		return nil
	}
	return w.removal.roomsBefore
}

func (w *World) duplicateConfiguredBasename(servedRoot, basename string) error {
	cfg := map[string]any{
		"homeserver_url": w.synapse.URL,
		"user_id":        w.bridgeUserID,
		"password":       w.bridgePassword,
		"operator":       w.operatorID,
		"forges": []map[string]string{
			{"root": servedRoot},
			{"root": filepath.Join(w.workDir, "same-one", basename)},
			{"root": filepath.Join(w.workDir, "same-two", basename)},
		},
		"state_dir": w.stateDir,
	}
	body, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(w.bridgeConfigOf(servedRoot), append(body, '\n'), 0o644)
}

func removalPowerLevels(bridge, operator string, operatorKeepsPower bool) []*event.Event {
	stateDefault, kick := 0, 50
	operatorLevel, bridgeLevel := 0, 50
	if operatorKeepsPower {
		operatorLevel, bridgeLevel = 100, 0
	}
	return []*event.Event{{
		Type: event.StatePowerLevels,
		Content: event.Content{Parsed: &event.PowerLevelsEventContent{
			Users:           map[id.UserID]int{id.UserID(bridge): bridgeLevel, id.UserID(operator): operatorLevel},
			UsersDefault:    0,
			EventsDefault:   0,
			StateDefaultPtr: &stateDefault,
			KickPtr:         &kick,
		}},
	}}
}

func removalOrderedRooms(forge state.Forge) []string {
	rooms := []string{
		forge.SpaceID,
		forge.RoomID,
		forge.ApprovalsRoomID,
		forge.ActivityRoomID,
		forge.ClarificationsRoomID,
	}
	seen := map[string]bool{}
	var distinct []string
	for _, roomID := range rooms {
		if roomID == "" || seen[roomID] {
			continue
		}
		seen[roomID] = true
		distinct = append(distinct, roomID)
	}
	return distinct
}

func snapshotRemovalTree(root string) (map[string]string, error) {
	snapshot := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		snapshot[relative] = hex.EncodeToString(sum[:])
		return nil
	})
	return snapshot, err
}

func sameSnapshot(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for path, hash := range left {
		if right[path] != hash {
			return false
		}
	}
	return true
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
