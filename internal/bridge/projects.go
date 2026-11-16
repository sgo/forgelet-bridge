package bridge

import (
	"context"
	"fmt"

	"github.com/unclebob/forgelet-bridge/internal/relay"
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

// carryOutProjectClarifications reads the project dashboard once when it
// supports snapshots, then carries its clarification work.
func (b *Bridge) carryOutProjectClarifications(ctx context.Context, forgeRoot string, project Project, room Room, seen roomEvents, snapshots snapshotCache) (carriedOut int, err error) {
	source, snapshotsEnabled := project.Clarifications.(SnapshotStore)
	if !snapshotsEnabled {
		return b.carryOutClarifications(
			ctx, project.Root, project.Clarifications, room,
			seen.messages[room.ClarificationsRoomID], relay.DashboardSnapshot{}, false)
	}
	snapshot, err := b.snapshotFrom(ctx, snapshots, source)
	if err != nil {
		return
	}
	if snapshot.ProjectionFallback {
		b.compatibilityFallbacks[forgeRoot] = true
	}
	if typedClarificationsHandledByFamily(forgeRoot, project) {
		snapshot.TypedClarifications = nil
	}
	return b.carryOutClarifications(
		ctx, project.Root, project.Clarifications, room,
		seen.messages[room.ClarificationsRoomID], snapshot, true)
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
// {"version":1,"tested_at":"2026-11-16T18:26:38+01:00","module_hash":"a33a476d49d0d1deee5fbb7f693ad924afa58d14260d4f15fd00495cbcc16ac4","functions":[{"id":"func/Bridge.tickProject","name":"Bridge.tickProject","line":14,"end_line":29,"hash":"801a1358646ee588e5c0cbd56b05b26e249f063e898517e91f64b45daa6b5508"},{"id":"func/Bridge.carryOutProjectClarifications","name":"Bridge.carryOutProjectClarifications","line":33,"end_line":53,"hash":"e78803543727e3cdcf0676131df144c6c76ed148af7bed7f2a03af50363188e9"},{"id":"func/typedClarificationsHandledByFamily","name":"typedClarificationsHandledByFamily","line":58,"end_line":60,"hash":"a8d15efbd63bc3a2bad5e24da06b7fd9e8dd9bf57d87ab9a78a1ae56b284cfd5"},{"id":"func/Bridge.projectRoomFor","name":"Bridge.projectRoomFor","line":65,"end_line":74,"hash":"21c6b7fc803665b6ef275d3ebc3aaaabe8562959e65cecfa8744ff185cbf105b"},{"id":"func/Bridge.projectForge","name":"Bridge.projectForge","line":78,"end_line":90,"hash":"a4cbd6d0fa459d357bd30844058b067a024446423f398b3acc34ec84f55616b3"},{"id":"func/Bridge.refreshProjectRoom","name":"Bridge.refreshProjectRoom","line":93,"end_line":100,"hash":"3e375a5ac248e1bba15983c6ab854d7c002a4b67c0681ac146ddde9fa7736289"},{"id":"func/Bridge.createProjectRoom","name":"Bridge.createProjectRoom","line":103,"end_line":117,"hash":"d7ef531616645f47012271c55dc63cbecdb6009242cc1cca7776b026000c1ff2"},{"id":"func/projectRoom","name":"projectRoom","line":119,"end_line":125,"hash":"bee1819d85260f11665850ff25dd2eb2a12906dd4b5cba05845505eb85ca4d5c"}]}
// mutate4go-manifest-end
