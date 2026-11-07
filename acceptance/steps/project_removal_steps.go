package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"github.com/unclebob/forgelet-bridge/internal/state"
)

func adapterRemovesProject(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	target, err := w.declaredForge(captures[3])
	if err != nil {
		return err
	}
	return w.invokeProjectRemoval(captures[1], target.Root(), captures[2])
}

func adapterAskedToRemoveProject(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	if w.projectRemoval == nil {
		return fmt.Errorf("the scenario has not recorded managed projects")
	}
	if captures[2] == "extra-arguments" {
		return w.invokeProjectRemovalArgs(captures[1], w.projectRemoval.targetRoot,
			w.projectRemoval.targetName, "extra")
	}
	args, err := w.projectRemovalRootArguments(captures[2])
	if err != nil {
		return err
	}
	args = append(args, w.projectRemoval.targetName)
	return w.invokeProjectRemovalArgs(captures[1], args...)
}

func adapterAskedToRemoveProjectByCase(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	if w.projectRemoval == nil {
		return fmt.Errorf("the scenario has not recorded managed projects")
	}
	if _, err := w.declaredForge(captures[1]); err != nil {
		return err
	}
	target, err := w.declaredForge("forge-b")
	if err != nil {
		return err
	}
	project, err := w.projectRemovalProjectArgument(captures[2])
	if err != nil {
		return err
	}
	return w.invokeProjectRemoval(captures[1], target.Root(), project)
}

func (w *World) projectRemovalRootArguments(argumentCase string) ([]string, error) {
	switch argumentCase {
	case "blank":
		return []string{""}, nil
	case "relative":
		return []string{filepath.Base(w.projectRemoval.targetRoot)}, nil
	case "parent-traversal":
		return []string{filepath.Dir(w.projectRemoval.targetRoot) + string(filepath.Separator) +
			".." + string(filepath.Separator) + filepath.Base(w.projectRemoval.targetRoot)}, nil
	case "unknown-absolute":
		return []string{filepath.Join(w.workDir, "unknown-forge")}, nil
	case "configured-non-Forgelet":
		root := filepath.Join(w.workDir, "configured-not-a-forge")
		if err := os.MkdirAll(root, 0o755); err != nil {
			return nil, err
		}
		if err := addConfiguredRoot(w.configPath, root); err != nil {
			return nil, err
		}
		return []string{root}, nil
	default:
		return nil, fmt.Errorf("unknown project removal root case %q", argumentCase)
	}
}

func (w *World) projectRemovalProjectArgument(argumentCase string) (string, error) {
	switch argumentCase {
	case "blank":
		return "", nil
	case "padded":
		return " target ", nil
	case "parent-traversal":
		return "../target", nil
	case "absolute":
		return string(filepath.Separator) + "target", nil
	case "matrix-room-id":
		return w.projectRemoval.targetRoom, nil
	case "unknown":
		return "unknown-project", nil
	case "target":
		return "target", nil
	default:
		return "", fmt.Errorf("unknown project removal project case %q", argumentCase)
	}
}

func addConfiguredRoot(configPath, root string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}
	forges, _ := cfg["forges"].([]any)
	cfg["forges"] = append(forges, map[string]any{"root": root})
	body, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, append(body, '\n'), 0o644)
}

func matrixRefusesProjectRemoval(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	reason := map[string]string{
		"clearing the relay facts":                    "relay_cleanup_failed",
		"removing the operator":                       "operator_removal_failed",
		"leaving or forgetting the bridge membership": "bridge_membership_failed",
	}[captures[1]]
	if reason == "" {
		return fmt.Errorf("unknown project removal failure %q", captures[1])
	}
	w.projectRemoval.failureStage = reason
	w.adapterEnv = append(w.adapterEnv, "FORGELET_BRIDGE_REMOVE_TEST_FAILURE="+reason+"="+w.projectRemoval.targetRoom)
	return nil
}

func projectRemovalResultReports(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	if w.projectRemoval == nil || !w.projectRemoval.operationDone {
		return fmt.Errorf("the adapter has not answered a project removal request")
	}
	if w.projectRemoval.status != captures[1] {
		return fmt.Errorf("project removal status = %q, want %q; output: %s",
			w.projectRemoval.status, captures[1], w.projectRemoval.rawOutput)
	}
	wantReason := ""
	if len(captures) > 2 {
		wantReason = captures[2]
	}
	if w.projectRemoval.reason != wantReason {
		return fmt.Errorf("project removal reason = %q, want %q; output: %s",
			w.projectRemoval.reason, wantReason, w.projectRemoval.rawOutput)
	}
	return nil
}

func projectAdapterLeftBridgeRunning(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	output, err := w.adapterCommand(ctx, "status")
	if err != nil {
		return err
	}
	if !strings.HasPrefix(strings.TrimSpace(output), "running ") {
		return fmt.Errorf("the adapter left the bridge stopped:\n%s", output)
	}
	return nil
}

func projectRemovalTargetedOnlyTarget(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	if w.projectRemoval == nil || w.projectRemoval.targetName != captures[1] {
		return fmt.Errorf("the removal did not name the recorded target project %s", captures[1])
	}
	if w.projectRemoval.targetRoom == "" || w.projectRemoval.targetRoom == w.projectRemoval.siblingRoom {
		return fmt.Errorf("target and sibling project rooms are not distinct")
	}
	return nil
}

func projectRemovalClearedFactsBeforeMembership(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	if w.projectRemoval.targetName != captures[1] {
		return fmt.Errorf("the removal named %s, want %s", w.projectRemoval.targetName, captures[1])
	}
	ctx, cancel := stepContext()
	defer cancel()
	if err := projectFactsEmpty(ctx, w, w.projectRemoval.targetRoom); err != nil {
		return err
	}
	return nil
}

func projectRemovalMembershipsGone(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	if w.projectRemoval.targetName != captures[1] {
		return fmt.Errorf("the removal named %s, want %s", w.projectRemoval.targetName, captures[1])
	}
	ctx, cancel := stepContext()
	defer cancel()
	observer, err := w.user(ctx, removalObserver)
	if err != nil {
		return err
	}
	for _, userID := range []string{w.operatorID, w.bridgeUserID} {
		membership, err := observer.Membership(ctx, w.projectRemoval.targetRoom, userID)
		if err != nil {
			return err
		}
		if membership == string(event.MembershipJoin) || membership == string(event.MembershipInvite) {
			return fmt.Errorf("%s is still %s in the removed project room", userID, membership)
		}
	}
	return nil
}

func observerCanReadProjectHistory(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	observer, err := w.user(ctx, removalObserver)
	if err != nil {
		return err
	}
	if _, err := observer.Client().GetEvent(ctx, id.RoomID(w.projectRemoval.targetRoom),
		id.EventID(w.projectRemoval.historyEvent)); err != nil {
		return fmt.Errorf("the removed project's history is no longer readable: %w", err)
	}
	return nil
}

func projectStateRemovedTargetKeptSibling(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	loaded, err := state.Load(w.removalStatePath())
	if err != nil {
		return err
	}
	forge := loaded.Forges[w.projectRemoval.targetRoot]
	if _, ok := forge.Projects[captures[1]]; ok {
		return fmt.Errorf("the target project record remains")
	}
	if forge.Projects[captures[2]].RoomID == "" {
		return fmt.Errorf("the sibling project record was removed")
	}
	return nil
}

func projectForgeAndSiblingPreserved(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	loaded, err := state.Load(w.removalStatePath())
	if err != nil {
		return err
	}
	forge := loaded.Forges[w.projectRemoval.targetRoot]
	observer, err := w.user(ctx, removalObserver)
	if err != nil {
		return err
	}
	rooms := []string{
		forge.SpaceID, forge.RoomID, forge.ApprovalsRoomID,
		forge.ActivityRoomID, forge.ClarificationsRoomID,
		w.projectRemoval.siblingRoom,
	}
	for _, roomID := range rooms {
		for _, userID := range []string{w.operatorID, w.bridgeUserID} {
			membership, err := observer.Membership(ctx, roomID, userID)
			if err != nil {
				return err
			}
			if membership != string(event.MembershipJoin) && membership != string(event.MembershipInvite) {
				return fmt.Errorf("preserved room %s lost %s (%s)", roomID, userID, membership)
			}
		}
	}
	if err := factNonEmpty(ctx, w, w.projectRemoval.siblingRoom, "com.forgelet.board", w.projectRemoval.siblingName); err != nil {
		return err
	}
	if err := factNonEmpty(ctx, w, forge.ActivityRoomID, "com.forgelet.board", w.projectRemoval.siblingName); err != nil {
		return err
	}
	if err := factNonEmpty(ctx, w, forge.ApprovalsRoomID, "com.forgelet.approval", w.projectRemoval.siblingName+"/approval"); err != nil {
		return err
	}
	if err := factNonEmpty(ctx, w, forge.RoomID, "com.forgelet.board", "forge"); err != nil {
		return err
	}
	_, targetActivity := loaded.Relay.Activity["target/card"]
	if w.projectRemoval.status == "removed" && targetActivity {
		return fmt.Errorf("target activity bookkeeping remains after removal")
	}
	if w.projectRemoval.status == "retryable" && !targetActivity {
		return fmt.Errorf("retryable removal lost target activity bookkeeping")
	}
	if _, ok := loaded.Relay.Activity["sibling/card"]; !ok {
		return fmt.Errorf("sibling activity bookkeeping was removed")
	}
	_, targetThread := loaded.Relay.Threads["target/request"]
	if w.projectRemoval.status == "removed" && targetThread {
		return fmt.Errorf("target project-scoped thread remains after removal")
	}
	if w.projectRemoval.status == "retryable" && !targetThread {
		return fmt.Errorf("retryable removal lost the target project-scoped thread")
	}
	if _, ok := loaded.Relay.Threads["sibling/request"]; !ok {
		return fmt.Errorf("sibling project-scoped thread was removed")
	}
	if _, ok := loaded.Relay.Threads["forge/request"]; !ok {
		return fmt.Errorf("forge-level relay thread was removed")
	}
	return nil
}

func projectRemovalConfigAndFilesUnchanged(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	config, err := os.ReadFile(w.configPath)
	if err != nil {
		return err
	}
	if string(config) != string(w.projectRemoval.configBefore) {
		return fmt.Errorf("project removal changed bridge configuration")
	}
	after, err := snapshotRemovalTree(w.projectRemoval.targetRoot)
	if err != nil {
		return err
	}
	if !sameSnapshot(w.projectRemoval.targetBefore, after) {
		return fmt.Errorf("project removal changed the forge or project files")
	}
	return nil
}

func projectRemovalResultSanitized(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	output := w.projectRemoval.rawOutput
	for _, forbidden := range []string{
		"!", "@", w.projectRemoval.targetName, w.projectRemoval.siblingName,
		w.projectRemoval.targetRoot, w.bridgeToken,
	} {
		if forbidden != "" && strings.Contains(output, forbidden) {
			return fmt.Errorf("the project removal result contains %q: %s", forbidden, output)
		}
	}
	return nil
}

func projectStateRetainsProgress(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	loaded, err := state.Load(w.removalStatePath())
	if err != nil {
		return err
	}
	project, ok := loaded.Forges[w.projectRemoval.targetRoot].Projects[w.projectRemoval.targetName]
	if !ok || project.RoomID != w.projectRemoval.targetRoom {
		return fmt.Errorf("retry lost the target project or its room id")
	}
	progress := project.Cleanup
	switch w.projectRemoval.failureStage {
	case "relay_cleanup_failed":
		if progress != nil && progress.FactsCleared {
			return fmt.Errorf("failed fact cleanup was recorded complete: %+v", progress)
		}
	case "operator_removal_failed":
		if progress == nil || !progress.FactsCleared || progress.OperatorRemoved {
			return fmt.Errorf("operator failure progress = %+v", progress)
		}
	case "bridge_membership_failed":
		if progress == nil || !progress.FactsCleared || !progress.OperatorRemoved || progress.BridgeLeft {
			return fmt.Errorf("membership failure progress = %+v", progress)
		}
	default:
		return fmt.Errorf("the scenario recorded no known Matrix failure")
	}
	return nil
}

func forgeRepairsProjectRemoval(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	w.adapterEnv = nil
	return nil
}

func repeatedProjectRemovalChangedNothing(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	after, err := os.ReadFile(w.removalStatePath())
	if err != nil {
		return err
	}
	if string(after) != string(w.projectRemoval.stateBefore) {
		return fmt.Errorf("the repeated project removal changed bridge state")
	}
	if err := projectRemovalConfigurationUnchanged(w); err != nil {
		return err
	}
	return nil
}

func refusedProjectRemovalTouchedNoRoom(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	observer, err := w.user(ctx, removalObserver)
	if err != nil {
		return err
	}
	for _, userID := range []string{w.operatorID, w.bridgeUserID} {
		membership, err := observer.Membership(ctx, w.projectRemoval.targetRoom, userID)
		if err != nil {
			return err
		}
		if membership != string(event.MembershipJoin) && membership != string(event.MembershipInvite) {
			return fmt.Errorf("a refused removal changed %s membership to %s", userID, membership)
		}
	}
	return nil
}

func refusedProjectRemovalChangedNothing(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	afterState, err := os.ReadFile(w.removalStatePath())
	if err != nil {
		return err
	}
	if string(afterState) != string(w.projectRemoval.stateBefore) {
		return fmt.Errorf("a refused project removal changed bridge state")
	}
	if err := projectRemovalConfigurationUnchanged(w); err != nil {
		return err
	}
	after, err := snapshotRemovalTree(w.projectRemoval.targetRoot)
	if err != nil {
		return err
	}
	if !sameSnapshot(w.projectRemoval.targetBefore, after) {
		return fmt.Errorf("a refused project removal changed forge or project files")
	}
	return nil
}

func projectRemovalConfigurationUnchanged(w *World) error {
	config, err := os.ReadFile(w.configPath)
	if err != nil {
		return err
	}
	if string(config) != string(w.projectRemoval.configBefore) {
		return fmt.Errorf("project removal changed bridge configuration")
	}
	return nil
}

func projectStateMissingRoom(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	w.stopAdapterBridge()
	loaded, err := state.Load(w.removalStatePath())
	if err != nil {
		return err
	}
	forge := loaded.Forges[w.projectRemoval.targetRoot]
	project := forge.Projects[w.projectRemoval.targetName]
	project.RoomID = ""
	forge.Projects[w.projectRemoval.targetName] = project
	loaded.Forges[w.projectRemoval.targetRoot] = forge
	if err := loaded.Save(w.removalStatePath()); err != nil {
		return err
	}
	if _, err := w.adapterCommand(ctx, "start"); err != nil {
		return err
	}
	return nil
}

func projectFactsEmpty(ctx context.Context, w *World, roomID string) error {
	observer, err := w.user(ctx, removalObserver)
	if err != nil {
		return err
	}
	current, err := observer.Client().State(ctx, id.RoomID(roomID))
	if err != nil {
		return err
	}
	for eventType, byKey := range current {
		if !strings.HasPrefix(eventType.Type, "com.forgelet.") {
			continue
		}
		for key, evt := range byKey {
			if evt != nil && len(evt.Content.Raw) != 0 {
				return fmt.Errorf("project fact %s/%s remains in its room", eventType.Type, key)
			}
		}
	}
	return nil
}

func factNonEmpty(ctx context.Context, w *World, roomID, eventType, stateKey string) error {
	observer, err := w.user(ctx, removalObserver)
	if err != nil {
		return err
	}
	evt, err := observer.Client().FullStateEvent(ctx, id.RoomID(roomID),
		event.Type{Type: eventType, Class: event.StateEventType}, stateKey)
	if err != nil {
		return fmt.Errorf("read preserved fact %s/%s: %w", eventType, stateKey, err)
	}
	if evt == nil || len(evt.Content.Raw) == 0 {
		return fmt.Errorf("preserved fact %s/%s is empty", eventType, stateKey)
	}
	return nil
}
