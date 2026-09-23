package bridge

import (
	"context"
	"fmt"

	"github.com/unclebob/forgelet-bridge/internal/state"
)

// roomFor returns the forge's Matrix space and chat room, creating them the
// first time it is asked for a forge root and reusing them after a restart.
func (b *Bridge) roomFor(ctx context.Context, root string) (Room, error) {
	if room, ok := b.provisioned[root]; ok {
		return room, nil
	}
	if forge, ok := b.state.ForgeFor(root); ok {
		room := Room{
			SpaceID:              forge.SpaceID,
			RoomID:               forge.RoomID,
			ApprovalsRoomID:      forge.ApprovalsRoomID,
			ActivityRoomID:       forge.ActivityRoomID,
			ClarificationsRoomID: forge.ClarificationsRoomID,
		}
		if err := b.rooms.RefreshForge(ctx, room, b.cfg.ForgeName(root), b.cfg.Operator); err != nil {
			return Room{}, fmt.Errorf("apply the forge's name to %s: %w", root, err)
		}
		b.provisioned[root] = room
		return room, nil
	}

	room, err := b.rooms.EnsureForge(ctx, b.cfg.ForgeName(root), b.cfg.Operator)
	if err != nil {
		return Room{}, fmt.Errorf("provision forge %s: %w", root, err)
	}
	b.state.RecordForge(root, state.Forge{
		SpaceID:              room.SpaceID,
		RoomID:               room.RoomID,
		ApprovalsRoomID:      room.ApprovalsRoomID,
		ActivityRoomID:       room.ActivityRoomID,
		ClarificationsRoomID: room.ClarificationsRoomID,
	})
	if err := b.state.Save(b.statePath); err != nil {
		return Room{}, err
	}
	b.provisioned[root] = room
	b.log.Info("provisioned forge", "root", root, "space", room.SpaceID, "room", room.RoomID)
	return room, nil
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-23T13:34:17+02:00","module_hash":"93598fd9aff958246c26d571e74a42bf8ab075bbcb8a795a2dbf28f2cb47405b","functions":[{"id":"func/Bridge.roomFor","name":"Bridge.roomFor","line":12,"end_line":48,"hash":"9a21cd2bad53702f176f5bb2caa20f9388453645e9437bbfe2eabb6bcdcd1e84"}]}
// mutate4go-manifest-end
