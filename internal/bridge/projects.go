package bridge

import (
	"context"
	"fmt"

	"github.com/unclebob/forgelet-bridge/internal/state"
)

// tickProject catches one single-agent project's room up with its dashboard
// queue and the queue up with its room. The project room is the chat and
// clarification room; it has no approvals or family activity of its own.
func (b *Bridge) tickProject(ctx context.Context, forgeRoot string, project Project, seen roomEvents, snapshots snapshotCache) (carriedOut int, err error) {
	room, err := b.projectRoomFor(ctx, forgeRoot, project)
	if err != nil {
		return
	}
	requests, err := project.Store.Requests()
	if err != nil {
		return 0, fmt.Errorf("read dashboard requests for %s: %w", project.Root, err)
	}
	carriedOut = b.carryOutChat(ctx, forgeRoot, project.Name, project.Store, room, seen, requests)
	clarifications, err := b.carryOutProjectClarifications(ctx, forgeRoot, project, room, seen, snapshots)
	if err != nil {
		return carriedOut, err
	}
	return carriedOut + clarifications, nil
}

// carryOutProjectClarifications leaves nested projects to the Family typed
// authority. A project configured as its own forge root can use its snapshot
// directly.
func (b *Bridge) carryOutProjectClarifications(ctx context.Context, forgeRoot string, project Project, room Room, seen roomEvents, snapshots snapshotCache) (carriedOut int, err error) {
	if typedClarificationsHandledByFamily(forgeRoot, project) {
		return 0, nil
	}
	source, snapshotsEnabled := project.Clarifications.(SnapshotStore)
	if !snapshotsEnabled {
		return 0, fmt.Errorf("typed clarifications require an automation snapshot for project root %s", project.Root)
	}
	snapshot, err := b.snapshotFrom(ctx, snapshots, source)
	if err != nil {
		return
	}
	return b.carryOutClarifications(
		ctx, project.Root, project.Clarifications, room,
		seen.messages[room.ClarificationsRoomID], snapshot)
}

// typedClarificationsHandledByFamily reports whether the forge-level snapshot
// owns the project's typed notifications. A nested project satisfies this; a
// project configured as the forge root is served by the forge-level tick.
func typedClarificationsHandledByFamily(forgeRoot string, project Project) bool {
	return forgeRoot != project.Root
}

// projectRoomFor returns the project's room inside its forge's space, creating
// and persisting it the first time and refreshing the remembered room after a
// restart.
func (b *Bridge) projectRoomFor(ctx context.Context, forgeRoot string, project Project) (Room, error) {
	forge, err := b.projectForge(ctx, forgeRoot)
	if err != nil {
		return Room{}, err
	}
	if stored, ok := b.state.ProjectFor(forgeRoot, project.Name); ok {
		return b.refreshProjectRoom(ctx, forgeRoot, forge.SpaceID, project, stored.RoomID)
	}
	return b.createProjectRoom(ctx, forgeRoot, forge.SpaceID, project)
}

// projectForge ensures the forge room state exists before its projects need
// the space it names.
func (b *Bridge) projectForge(ctx context.Context, forgeRoot string) (state.Forge, error) {
	if forge, ok := b.state.ForgeFor(forgeRoot); ok {
		return forge, nil
	}
	if _, err := b.roomFor(ctx, forgeRoot); err != nil {
		return state.Forge{}, err
	}
	forge, ok := b.state.ForgeFor(forgeRoot)
	if !ok {
		return state.Forge{}, fmt.Errorf("forge %s has no complete room state", forgeRoot)
	}
	return forge, nil
}

// refreshProjectRoom applies current configured names to a remembered room.
func (b *Bridge) refreshProjectRoom(ctx context.Context, forgeRoot, spaceID string, project Project, roomID string) (Room, error) {
	if err := b.rooms.RefreshProjectRoom(
		ctx, roomID, b.cfg.ForgeName(forgeRoot),
		project.Name, b.cfg.Operator); err != nil {
		return Room{}, fmt.Errorf("apply the project's name to %s: %w", project.Root, err)
	}
	return projectRoom(spaceID, roomID), nil
}

// createProjectRoom provisions and remembers a project's first room.
func (b *Bridge) createProjectRoom(ctx context.Context, forgeRoot, spaceID string, project Project) (Room, error) {
	roomID, err := b.rooms.EnsureProjectRoom(
		ctx, spaceID, b.cfg.ForgeName(forgeRoot),
		project.Name, b.cfg.Operator)
	if err != nil {
		return Room{}, fmt.Errorf("provision project %s: %w", project.Root, err)
	}
	b.state.RecordProject(forgeRoot, project.Name, state.Project{RoomID: roomID})
	if err := b.state.Save(b.statePath); err != nil {
		return Room{}, err
	}
	b.log.Info("provisioned project room",
		"root", project.Root, "space", spaceID, "room", roomID)
	return projectRoom(spaceID, roomID), nil
}

func projectRoom(spaceID, roomID string) Room {
	return Room{
		SpaceID:              spaceID,
		RoomID:               roomID,
		ClarificationsRoomID: roomID,
	}
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-11-17T15:19:43+01:00","module_hash":"d72db379e4b3e1ba71c98b569dc063626a1ba0b5ce27c3a60097b9e508032119","functions":[{"id":"func/Bridge.tickProject","name":"Bridge.tickProject","line":13,"end_line":28,"hash":"801a1358646ee588e5c0cbd56b05b26e249f063e898517e91f64b45daa6b5508"},{"id":"func/Bridge.carryOutProjectClarifications","name":"Bridge.carryOutProjectClarifications","line":33,"end_line":48,"hash":"641d4f1bd04d5507a46412c0d5f17212db422777bbc689c9cef8051cc2cbd34c"},{"id":"func/typedClarificationsHandledByFamily","name":"typedClarificationsHandledByFamily","line":53,"end_line":55,"hash":"a8d15efbd63bc3a2bad5e24da06b7fd9e8dd9bf57d87ab9a78a1ae56b284cfd5"},{"id":"func/Bridge.projectRoomFor","name":"Bridge.projectRoomFor","line":60,"end_line":69,"hash":"21c6b7fc803665b6ef275d3ebc3aaaabe8562959e65cecfa8744ff185cbf105b"},{"id":"func/Bridge.projectForge","name":"Bridge.projectForge","line":73,"end_line":85,"hash":"a4cbd6d0fa459d357bd30844058b067a024446423f398b3acc34ec84f55616b3"},{"id":"func/Bridge.refreshProjectRoom","name":"Bridge.refreshProjectRoom","line":88,"end_line":95,"hash":"3e375a5ac248e1bba15983c6ab854d7c002a4b67c0681ac146ddde9fa7736289"},{"id":"func/Bridge.createProjectRoom","name":"Bridge.createProjectRoom","line":98,"end_line":112,"hash":"d7ef531616645f47012271c55dc63cbecdb6009242cc1cca7776b026000c1ff2"},{"id":"func/projectRoom","name":"projectRoom","line":114,"end_line":120,"hash":"bee1819d85260f11665850ff25dd2eb2a12906dd4b5cba05845505eb85ca4d5c"}]}
// mutate4go-manifest-end
