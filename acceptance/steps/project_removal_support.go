package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/internal/bridge"
	"github.com/unclebob/forgelet-bridge/internal/relay"
	"github.com/unclebob/forgelet-bridge/internal/state"
)

type projectRemovalScenario struct {
	targetRoot    string
	targetName    string
	siblingName   string
	targetRoom    string
	siblingRoom   string
	historyEvent  string
	status        string
	reason        string
	rawOutput     string
	failureStage  string
	stateBefore   []byte
	configBefore  []byte
	targetBefore  map[string]string
	operationDone bool
	planStatus    string
	planReason    string
	planRawOutput string
	planBridgeLog string
	planDone      bool
}

func recordManagedProjects(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	target, err := w.declaredForge(captures[3])
	if err != nil {
		return err
	}
	loaded, err := w.waitForRemovalForge(ctx, target.Root())
	if err != nil {
		return err
	}
	w.stopAdapterBridge()

	tracked := loaded.Forges[target.Root()]
	if tracked.Projects == nil {
		tracked.Projects = map[string]state.Project{}
	}
	actors, err := w.removalActors(ctx)
	if err != nil {
		return err
	}
	if err := inviteRemovalObserver(ctx, actors.bridge, actors.observer, removalOrderedRooms(tracked)); err != nil {
		return err
	}
	targetRoom, err := createManagedProjectRoom(ctx, target.Root(), captures[1], tracked, actors)
	if err != nil {
		return err
	}
	siblingRoom, err := createManagedProjectRoom(ctx, target.Root(), captures[2], tracked, actors)
	if err != nil {
		return err
	}
	tracked.Projects[captures[1]] = state.Project{RoomID: targetRoom}
	tracked.Projects[captures[2]] = state.Project{RoomID: siblingRoom}
	loaded.Forges[target.Root()] = tracked
	if err := loaded.Save(w.removalStatePath()); err != nil {
		return err
	}
	openProjects := filepath.Join(target.Root(), ".swarmforge", "open-projects")
	if err := os.MkdirAll(filepath.Dir(openProjects), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(openProjects, []byte(captures[2]+"\n"), 0o644); err != nil {
		return err
	}

	history, err := actors.operator.Client().SendText(ctx, id.RoomID(targetRoom), "history the bridge must preserve")
	if err != nil {
		return fmt.Errorf("seed target room history: %w", err)
	}
	w.projectRemoval = &projectRemovalScenario{
		targetRoot:   target.Root(),
		targetName:   captures[1],
		siblingName:  captures[2],
		targetRoom:   targetRoom,
		siblingRoom:  siblingRoom,
		historyEvent: history.EventID.String(),
	}
	if _, err := w.adapterCommand(ctx, "start"); err != nil {
		return err
	}
	return waitFor(ctx, "the bridge did not restart after the fixture recorded projects", func() (bool, error) {
		_, err := os.Stat(filepath.Join(w.stateDir, bridge.StatusName))
		return err == nil, nil
	})
}

func createManagedProjectRoom(
	ctx context.Context,
	forgeRoot, project string,
	tracked state.Forge,
	actors removalActors,
) (string, error) {
	response, err := actors.operator.Client().CreateRoom(ctx, &mautrix.ReqCreateRoom{
		Name:         "Project " + project,
		Preset:       "private_chat",
		Invite:       []id.UserID{actors.bridge.UserID, id.UserID(removalObserver)},
		InitialState: removalPowerLevels(actors.bridge.UserID.String(), actors.operator.UserID, false),
	})
	if err != nil {
		return "", fmt.Errorf("create project room %s: %w", project, err)
	}
	if _, err := actors.bridge.JoinRoomByID(ctx, response.RoomID); err != nil {
		return "", fmt.Errorf("join project room %s: %w", project, err)
	}
	if _, err := actors.observer.Client().JoinRoomByID(ctx, response.RoomID); err != nil {
		return "", fmt.Errorf("observe project room %s: %w", project, err)
	}
	if _, err := actors.bridge.SendStateEvent(ctx, id.RoomID(tracked.SpaceID), event.StateSpaceChild,
		response.RoomID.String(), map[string]any{"via": []string{fixtures.ServerName}}); err != nil {
		return "", fmt.Errorf("put project room %s in the forge space: %w", project, err)
	}
	marker := filepath.Join(forgeRoot, "projects", project, ".swarmforge", "single-agent")
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(marker, []byte(project+"\n"), 0o644); err != nil {
		return "", err
	}
	return response.RoomID.String(), nil
}

func seedProjectRemovalFacts(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	target, err := w.declaredForge(captures[1])
	if err != nil {
		return err
	}
	if w.projectRemoval == nil || w.projectRemoval.targetRoot != target.Root() ||
		w.projectRemoval.targetName != captures[2] || w.projectRemoval.siblingName != captures[3] {
		return fmt.Errorf("the project rooms named by the step are not the recorded ones")
	}
	loaded, err := state.Load(w.removalStatePath())
	if err != nil {
		return err
	}
	w.stopAdapterBridge()
	forge := loaded.Forges[target.Root()]
	if err := sendProjectFact(ctx, w, w.projectRemoval.targetRoom, relay.BoardFactType,
		w.projectRemoval.targetName, map[string]any{"project": w.projectRemoval.targetName}); err != nil {
		return err
	}
	if err := sendProjectFact(ctx, w, w.projectRemoval.targetRoom, relay.ClarificationFactType,
		w.projectRemoval.targetName+"/question", map[string]any{"project": w.projectRemoval.targetName}); err != nil {
		return err
	}
	if err := sendProjectFact(ctx, w, w.projectRemoval.siblingRoom, relay.BoardFactType,
		w.projectRemoval.siblingName, map[string]any{"project": w.projectRemoval.siblingName}); err != nil {
		return err
	}
	if err := sendProjectFact(ctx, w, forge.ActivityRoomID, relay.BoardFactType,
		w.projectRemoval.targetName, map[string]any{"project": w.projectRemoval.targetName}); err != nil {
		return err
	}
	if err := sendProjectFact(ctx, w, forge.ActivityRoomID, relay.BoardFactType,
		w.projectRemoval.siblingName, map[string]any{"project": w.projectRemoval.siblingName}); err != nil {
		return err
	}
	if err := sendProjectFact(ctx, w, forge.ApprovalsRoomID, relay.ApprovalFactType,
		w.projectRemoval.targetName+"/approval", map[string]any{"project": w.projectRemoval.targetName}); err != nil {
		return err
	}
	if err := sendProjectFact(ctx, w, forge.ApprovalsRoomID, relay.ApprovalFactType,
		w.projectRemoval.siblingName+"/approval", map[string]any{"project": w.projectRemoval.siblingName}); err != nil {
		return err
	}
	if err := sendProjectFact(ctx, w, forge.RoomID, relay.BoardFactType,
		"forge", map[string]any{"project": "forge"}); err != nil {
		return err
	}

	seedProjectRelayState(loaded, forge, w.projectRemoval, target.Root())
	if err := loaded.Save(w.removalStatePath()); err != nil {
		return err
	}
	if _, err := w.adapterCommand(ctx, "start"); err != nil {
		return err
	}
	return waitFor(ctx, "the bridge did not restart after the project facts were seeded", func() (bool, error) {
		_, err := os.Stat(filepath.Join(w.stateDir, bridge.StatusName))
		return err == nil, nil
	})
}

func sendProjectFact(
	ctx context.Context,
	w *World,
	roomID, factType, stateKey string,
	content map[string]any,
) error {
	synapse, err := w.homeserver(ctx)
	if err != nil {
		return err
	}
	client, err := mautrix.NewClient(synapse.BaseURL(), id.UserID(w.bridgeUserID), w.bridgeToken)
	if err != nil {
		return err
	}
	if _, err := client.SendStateEvent(ctx, id.RoomID(roomID),
		event.Type{Type: factType, Class: event.StateEventType}, stateKey, content); err != nil {
		return fmt.Errorf("seed %s in %s: %w", factType, roomID, err)
	}
	return nil
}

func seedProjectRelayState(loaded *state.State, forge state.Forge, scenario *projectRemovalScenario, root string) {
	loaded.Relay.EnsureMaps()
	loaded.Relay.Waiting[scenario.targetRoom+"\n"+relay.BoardFactType+"\n"+scenario.targetName] = `{"project":"target"}`
	loaded.Relay.Waiting[forge.ActivityRoomID+"\n"+relay.BoardFactType+"\n"+scenario.targetName] = `{"project":"target"}`
	loaded.Relay.Waiting[scenario.siblingRoom+"\n"+relay.BoardFactType+"\n"+scenario.siblingName] = `{"project":"sibling"}`
	loaded.Relay.Waiting[forge.ActivityRoomID+"\n"+relay.BoardFactType+"\n"+scenario.siblingName] = `{"project":"sibling"}`
	loaded.Relay.Waiting[forge.RoomID+"\n"+relay.BoardFactType+"\nforge"] = `{"project":"forge"}`
	loaded.Relay.Approvals[scenario.targetName+"/approval"] = relay.ApprovalState{
		ForgeRoot: root, RoomID: forge.ApprovalsRoomID, MessageID: "$target-approval",
	}
	loaded.Relay.Approvals[scenario.siblingName+"/approval"] = relay.ApprovalState{
		ForgeRoot: root, RoomID: forge.ApprovalsRoomID, MessageID: "$sibling-approval",
	}
	loaded.Relay.Clarifications[scenario.targetName+"/question"] = relay.ClarificationState{
		ForgeRoot: root, RoomID: scenario.targetRoom, MessageID: "$target-question",
	}
	loaded.Relay.Clarifications[scenario.siblingName+"/question"] = relay.ClarificationState{
		ForgeRoot: root, RoomID: scenario.siblingRoom, MessageID: "$sibling-question",
	}
	loaded.Relay.Clarifications["forge/question"] = relay.ClarificationState{
		ForgeRoot: root, RoomID: forge.ClarificationsRoomID, MessageID: "$forge-question",
	}
	loaded.Relay.Activity["target/card"] = relay.CardState{ForgeRoot: root, Project: scenario.targetName}
	loaded.Relay.Activity["sibling/card"] = relay.CardState{ForgeRoot: root, Project: scenario.siblingName}
	loaded.Relay.Threads["target/request"] = "$target-message"
	loaded.Relay.Threads["sibling/request"] = "$sibling-message"
	loaded.Relay.Threads["forge/request"] = "$forge-message"
	loaded.Relay.ScopeProject(relay.ScopeThreads, "target/request", root, scenario.targetName)
	loaded.Relay.ScopeProject(relay.ScopeThreads, "sibling/request", root, scenario.siblingName)
	loaded.Relay.Scope(relay.ScopeThreads, "forge/request", root)
}

func (w *World) invokeProjectRemoval(servedName, root, project string, extra ...string) error {
	return w.invokeProjectRemovalArgs(servedName, append([]string{root, project}, extra...)...)
}

func (w *World) invokeProjectRemovalArgs(servedName string, args ...string) error {
	served, err := w.declaredForge(servedName)
	if err != nil {
		return err
	}
	ctx, cancel := stepContext()
	defer cancel()
	if w.projectRemoval == nil {
		return fmt.Errorf("the scenario has not recorded managed projects")
	}
	w.projectRemoval.stateBefore, err = os.ReadFile(w.removalStatePath())
	if err != nil {
		return err
	}
	w.projectRemoval.configBefore, err = os.ReadFile(w.bridgeConfigOf(served.Root()))
	if err != nil {
		return err
	}
	w.projectRemoval.targetBefore, err = snapshotRemovalTree(w.projectRemoval.targetRoot)
	if err != nil {
		return err
	}
	w.projectRemoval.operationDone = false
	_, runErr := w.adapterCommand(ctx, append([]string{"remove-project"}, args...)...)
	if runErr != nil {
		return runErr
	}
	var report removalReport
	if err := json.Unmarshal([]byte(strings.TrimSpace(w.adapterStdout)), &report); err != nil {
		return fmt.Errorf("project removal did not return one JSON object: %q: %w", w.adapterStdout, err)
	}
	w.projectRemoval.status = report.Status
	w.projectRemoval.reason = report.Reason
	w.projectRemoval.rawOutput = w.adapterStdout
	w.projectRemoval.operationDone = true
	return nil
}
