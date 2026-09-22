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
			SpaceID:         forge.SpaceID,
			RoomID:          forge.RoomID,
			ApprovalsRoomID: forge.ApprovalsRoomID,
			ActivityRoomID:  forge.ActivityRoomID,
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
		SpaceID:         room.SpaceID,
		RoomID:          room.RoomID,
		ApprovalsRoomID: room.ApprovalsRoomID,
		ActivityRoomID:  room.ActivityRoomID,
	})
	if err := b.state.Save(b.statePath); err != nil {
		return Room{}, err
	}
	b.provisioned[root] = room
	b.log.Info("provisioned forge", "root", root, "space", room.SpaceID, "room", room.RoomID)
	return room, nil
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-22T21:33:18+02:00","module_hash":"59ef2d62d3bf4c0e89a54c4b75b922e2337ea293846b0921c3183fe0bf9096e7","functions":[{"id":"func/Bridge.roomFor","name":"Bridge.roomFor","line":12,"end_line":46,"hash":"39bc446b316019f97fd51935cea22d3e2e645d2137baab4c01ea0dd067c5859f"}]}
// mutate4go-manifest-end
