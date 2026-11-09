package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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
		"homeserver_url": w.matrixServer.BaseURL(),
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
	actors, err := w.removalActors(ctx)
	if err != nil {
		return err
	}

	scenario := &removalScenario{
		target:   targetRoot,
		project:  "removal-project-1",
		rooms:    removalOrderedRooms(tracked),
		projects: map[string]string{},
	}
	if err := inviteRemovalObserver(ctx, actors.bridge, actors.observer, scenario.rooms); err != nil {
		return err
	}
	if err := createRemovalProjectRooms(ctx, count, tracked, scenario, actors); err != nil {
		return err
	}

	loaded.Forges[targetRoot] = tracked
	seedRemovalRelay(loaded, tracked, scenario, targetRoot, w.configured[0])
	if err := loaded.Save(w.removalStatePath()); err != nil {
		return err
	}
	w.removal = scenario
	return nil
}

type removalActors struct {
	operator *fixtures.User
	observer *fixtures.User
	bridge   *mautrix.Client
}

func (w *World) removalActors(ctx context.Context) (removalActors, error) {
	operator, err := w.operator(ctx)
	if err != nil {
		return removalActors{}, err
	}
	synapse, err := w.homeserver(ctx)
	if err != nil {
		return removalActors{}, err
	}
	bridgeClient, err := mautrix.NewClient(synapse.BaseURL(), id.UserID(w.bridgeUserID), w.bridgeToken)
	if err != nil {
		return removalActors{}, err
	}
	observer, err := w.user(ctx, removalObserver)
	if err != nil {
		return removalActors{}, err
	}
	return removalActors{operator: operator, observer: observer, bridge: bridgeClient}, nil
}

func inviteRemovalObserver(
	ctx context.Context,
	bridgeClient *mautrix.Client,
	observer *fixtures.User,
	rooms []string,
) error {
	for _, roomID := range rooms {
		if _, err := bridgeClient.InviteUser(ctx, id.RoomID(roomID),
			&mautrix.ReqInviteUser{UserID: id.UserID(removalObserver)}); err == nil {
			if _, err := observer.Client().JoinRoomByID(ctx, id.RoomID(roomID)); err != nil {
				return fmt.Errorf("observe removal forge room %s: %w", roomID, err)
			}
		}
	}
	return nil
}

func createRemovalProjectRooms(
	ctx context.Context,
	count int,
	tracked state.Forge,
	scenario *removalScenario,
	actors removalActors,
) error {
	for index := 1; index <= count; index++ {
		project := fmt.Sprintf("removal-project-%d", index)
		response, err := actors.operator.Client().CreateRoom(ctx, &mautrix.ReqCreateRoom{
			Name:         "Project " + project,
			Preset:       "private_chat",
			Invite:       []id.UserID{actors.bridge.UserID, id.UserID(removalObserver)},
			InitialState: removalPowerLevels(actors.bridge.UserID.String(), actors.operator.UserID, false),
		})
		if err != nil {
			return fmt.Errorf("create removal project room %s: %w", project, err)
		}
		if _, err := actors.bridge.JoinRoomByID(ctx, response.RoomID); err != nil {
			return fmt.Errorf("join removal project room %s: %w", project, err)
		}
		if _, err := actors.observer.Client().JoinRoomByID(ctx, response.RoomID); err != nil {
			return fmt.Errorf("observe removal project room %s: %w", project, err)
		}
		if _, err := actors.bridge.SendStateEvent(ctx, id.RoomID(tracked.SpaceID), event.StateSpaceChild,
			response.RoomID.String(), map[string]any{"via": []string{fixtures.ServerName}}); err != nil {
			return fmt.Errorf("put removal project room %s in the forge space: %w", project, err)
		}
		tracked.Projects[project] = state.Project{RoomID: response.RoomID.String()}
		scenario.rooms = append(scenario.rooms, response.RoomID.String())
		scenario.projects[project] = response.RoomID.String()
	}
	return nil
}

func seedRemovalRelay(
	loaded *state.State,
	tracked state.Forge,
	scenario *removalScenario,
	targetRoot, keptRoot string,
) {
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
	loaded.Relay.Scope(relay.ScopeThreads, "kept/request", keptRoot)
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
	bridgeClient, err := mautrix.NewClient(synapse.BaseURL(), id.UserID(w.bridgeUserID), w.bridgeToken)
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
