package bridge

import (
	"context"
	"fmt"

	"github.com/unclebob/forgelet-bridge/internal/state"
)

// tickProject catches one single-agent project's room up with its dashboard
// queue and the queue up with its room. The project room is the chat and
// clarification room; it has no approvals or family activity of its own.
func (b *Bridge) tickProject(ctx context.Context, forgeRoot string, project Project, seen roomEvents) (int, error) {
	room, err := b.projectRoomFor(ctx, forgeRoot, project)
	if err != nil {
		return 0, err
	}
	requests, err := project.Store.Requests()
	if err != nil {
		return 0, fmt.Errorf("read dashboard requests for %s: %w", project.Root, err)
	}
	carriedOut := b.carryOutChat(ctx, forgeRoot, project.Name, project.Store, room, seen, requests)
	clarifications, err := b.carryOutClarifications(
		ctx, project.Root, project.Clarifications, room,
		seen.messages[room.ClarificationsRoomID])
	if err != nil {
		return carriedOut, err
	}
	return carriedOut + clarifications, nil
}

// projectRoomFor returns the project's room inside its forge's space, creating
// and persisting it the first time and refreshing the remembered room after a
// restart.
func (b *Bridge) projectRoomFor(ctx context.Context, forgeRoot string, project Project) (Room, error) {
	forge, ok := b.state.ForgeFor(forgeRoot)
	if !ok {
		if _, err := b.roomFor(ctx, forgeRoot); err != nil {
			return Room{}, err
		}
		forge, ok = b.state.ForgeFor(forgeRoot)
		if !ok {
			return Room{}, fmt.Errorf("forge %s has no complete room state", forgeRoot)
		}
	}

	if stored, ok := b.state.ProjectFor(forgeRoot, project.Name); ok {
		if err := b.rooms.RefreshProjectRoom(
			ctx, stored.RoomID, b.cfg.ForgeName(forgeRoot),
			project.Name, b.cfg.Operator); err != nil {
			return Room{}, fmt.Errorf("apply the project's name to %s: %w", project.Root, err)
		}
		return projectRoom(forge.SpaceID, stored.RoomID), nil
	}

	roomID, err := b.rooms.EnsureProjectRoom(
		ctx, forge.SpaceID, b.cfg.ForgeName(forgeRoot),
		project.Name, b.cfg.Operator)
	if err != nil {
		return Room{}, fmt.Errorf("provision project %s: %w", project.Root, err)
	}
	b.state.RecordProject(forgeRoot, project.Name, state.Project{RoomID: roomID})
	if err := b.state.Save(b.statePath); err != nil {
		return Room{}, err
	}
	b.log.Info("provisioned project room",
		"root", project.Root, "space", forge.SpaceID, "room", roomID)
	return projectRoom(forge.SpaceID, roomID), nil
}

func projectRoom(spaceID, roomID string) Room {
	return Room{
		SpaceID:              spaceID,
		RoomID:               roomID,
		ClarificationsRoomID: roomID,
	}
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-11-07T22:40:51+01:00","module_hash":"d866dbcbe0fb82f9714fe91ea1cf101b8e308f4f2e888a454157d81cda9dcff3","functions":[{"id":"func/Bridge.tickProject","name":"Bridge.tickProject","line":13,"end_line":30,"hash":"b215d8040dd3b335b1220dfe69bbf2b65bd222fc332c7d878ce2736b4792f2f7"},{"id":"func/Bridge.projectRoomFor","name":"Bridge.projectRoomFor","line":35,"end_line":69,"hash":"c1ea44f744b7d6ce89a0e2424587cdaaebc9fb0aeb942f827f10d7325a3387b5"},{"id":"func/projectRoom","name":"projectRoom","line":71,"end_line":77,"hash":"bee1819d85260f11665850ff25dd2eb2a12906dd4b5cba05845505eb85ca4d5c"}]}
// mutate4go-manifest-end
