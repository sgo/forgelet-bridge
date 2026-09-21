package bridge

import (
	"context"
	"fmt"

	"github.com/unclebob/forgelet-bridge/internal/config"
	"github.com/unclebob/forgelet-bridge/internal/state"
)

// roomFor returns the forge's Matrix space and chat room, creating them the
// first time it is asked for a forge root and reusing them after a restart.
func (b *Bridge) roomFor(ctx context.Context, root string) (Room, error) {
	if room, ok := b.provisioned[root]; ok {
		return room, nil
	}
	if forge, ok := b.state.ForgeFor(root); ok {
		room := Room{SpaceID: forge.SpaceID, RoomID: forge.RoomID}
		b.provisioned[root] = room
		return room, nil
	}

	room, err := b.rooms.EnsureForge(ctx, config.ForgeName(root), b.cfg.Operator)
	if err != nil {
		return Room{}, fmt.Errorf("provision forge %s: %w", root, err)
	}
	b.state.RecordForge(root, state.Forge{SpaceID: room.SpaceID, RoomID: room.RoomID})
	if err := b.state.Save(b.statePath); err != nil {
		return Room{}, err
	}
	b.provisioned[root] = room
	b.log.Info("provisioned forge", "root", root, "space", room.SpaceID, "room", room.RoomID)
	return room, nil
}
