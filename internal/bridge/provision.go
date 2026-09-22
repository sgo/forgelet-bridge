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
// {"version":1,"tested_at":"2026-09-22T15:00:06+02:00","module_hash":"3a0bb420c394505184216b421c1af7e958948051a1822f792a9e8dc69a767665","functions":[{"id":"func/Bridge.roomFor","name":"Bridge.roomFor","line":12,"end_line":33,"hash":"2a13a0da44f33d9ab190cc0325ed9ac02108922a17cda4b657d5878d94bfd754"}]}
// mutate4go-manifest-end
