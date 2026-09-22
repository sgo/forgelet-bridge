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

	room, err := b.rooms.EnsureForge(ctx, b.forgeName(root), b.cfg.Operator)
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

// forgeName is the name the operator knows a forge root by: the configured
// display name, or the folder's name when the configuration does not name it.
func (b *Bridge) forgeName(root string) string {
	for _, forge := range b.cfg.Forges {
		if forge.Root == root {
			return forge.DisplayName()
		}
	}
	return config.Forge{ // a root the configuration no longer names
		Root: root,
	}.DisplayName()
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-21T23:21:32+02:00","module_hash":"b626fe1027778c3335a2f4b45c7be3f8e44b0b935e92256ae211265331fcb83f","functions":[{"id":"func/Bridge.roomFor","name":"Bridge.roomFor","line":13,"end_line":34,"hash":"9eeb180eb4443fdc97190c3094c33263463e4f1ec8f7f37d1ea8070d5ec00782"}]}
// mutate4go-manifest-end
