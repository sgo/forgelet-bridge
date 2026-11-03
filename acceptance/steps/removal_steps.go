package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"github.com/unclebob/forgelet-bridge/internal/state"
)

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
	arguments, err := w.removalArguments(served.Root(), target.Root(), captures[2])
	if err != nil {
		return err
	}
	return w.invokeRemovalArgs(captures[1], arguments...)
}

func (w *World) removalArguments(servedRoot, targetRoot, argumentCase string) ([]string, error) {
	argument := targetRoot
	switch argumentCase {
	case "blank":
		argument = ""
	case "parent-traversal":
		argument = filepath.Dir(targetRoot) + string(os.PathSeparator) + ".." + string(os.PathSeparator) + filepath.Base(targetRoot)
	case "relative":
		argument = filepath.Base(targetRoot)
	case "basename shared by two configured forges":
		argument = filepath.Base(targetRoot)
		if err := w.duplicateConfiguredBasename(servedRoot, argument); err != nil {
			return nil, err
		}
	case "extra":
		return []string{targetRoot, "extra"}, nil
	default:
		return nil, fmt.Errorf("unknown removal argument case %q", argumentCase)
	}
	return []string{argument}, nil
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

func removalTargetedEveryRoom(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	if w.removal == nil || !slices.Equal(w.removal.roomsBefore, w.removal.rooms) {
		return fmt.Errorf("removal room plan changed: before=%v after=%v", w.removalRoomsBefore(), w.removal.rooms)
	}
	wantProjects, err := strconv.Atoi(captures[2])
	if err != nil {
		return fmt.Errorf("expected project-room count %q: %w", captures[2], err)
	}
	if projects := len(w.removal.rooms) - 5; projects != wantProjects {
		return fmt.Errorf("removal targeted %d project rooms, want exactly %d", projects, wantProjects)
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
	if lostRoomID(forge) {
		return fmt.Errorf("retry lost a forge room id: %+v", forge)
	}
	for name, project := range w.removal.projects {
		if forge.Projects[name].RoomID != project {
			return fmt.Errorf("retry lost project %s room id %s", name, project)
		}
	}
	return nil
}

func lostRoomID(forge state.Forge) bool {
	return forge.SpaceID == "" || forge.RoomID == "" || forge.ApprovalsRoomID == "" ||
		forge.ActivityRoomID == "" || forge.ClarificationsRoomID == ""
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
	return refusedRemovalKeptTrees(w)
}

func refusedRemovalKeptTrees(w *World) error {
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
