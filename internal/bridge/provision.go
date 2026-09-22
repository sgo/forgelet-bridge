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
		room := Room{SpaceID: forge.SpaceID, RoomID: forge.RoomID, ApprovalsRoomID: forge.ApprovalsRoomID}
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
	})
	if err := b.state.Save(b.statePath); err != nil {
		return Room{}, err
	}
	b.provisioned[root] = room
	b.log.Info("provisioned forge", "root", root, "space", room.SpaceID, "room", room.RoomID)
	return room, nil
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-22T15:45:29+02:00","module_hash":"0972754a1860ce0a9e4bcbc805c5e693842a3aaa6d5cc7dd145b58c04272893b","functions":[{"id":"func/Bridge.roomFor","name":"Bridge.roomFor","line":12,"end_line":37,"hash":"b9fe2be422c50208b9c71533f33085c2b0de2c9f411db1f939ec75cea6261999"}]}
// mutate4go-manifest-end
