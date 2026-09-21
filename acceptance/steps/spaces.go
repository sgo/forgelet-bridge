package steps

import (
	"context"
	"fmt"
)

func fixturesRunning(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	for _, name := range forgeNames(captures[1]) {
		if _, err := w.forge(context.Background(), name); err != nil {
			return err
		}
		if err := w.startDashboard(name); err != nil {
			return err
		}
	}
	return nil
}

func configuredForgeAndOperator(_ context.Context, world any, captures []string) error {
	if err := configuredForges(context.Background(), world, []string{captures[0], captures[1]}); err != nil {
		return err
	}
	return configuredOperator(context.Background(), world, []string{captures[0], captures[2]})
}

func configuredOperator(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	w.operatorID = captures[1]
	return nil
}

func configuredForges(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	for _, name := range forgeNames(captures[1]) {
		if err := w.configureForge(ctx, name); err != nil {
			return err
		}
	}
	return w.configure(ctx)
}

func bridgeStarted(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	return w.startBridge(ctx)
}

func bridgeRestarted(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	w.stopBridge()
	ctx, cancel := stepContext()
	defer cancel()
	return w.startBridge(ctx)
}

func bridgeCaughtUp(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	return w.caughtUp(ctx)
}

func bridgeCreatedSpace(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	forgeName, roomName := captures[1], captures[2]
	ctx, cancel := stepContext()
	defer cancel()
	if err := w.startBridge(ctx); err != nil {
		return err
	}
	if _, err := w.waitForSpaceChild(ctx, forgeName, roomName); err != nil {
		return err
	}
	return nil
}

func operatorSeesSpace(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	spaces, err := w.spaces(ctx, captures[1])
	if err != nil {
		return err
	}
	if len(spaces) == 0 {
		return fmt.Errorf("the operator does not see a forge space named %s", captures[1])
	}
	return nil
}

func operatorSeesOneSpace(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	spaces, err := w.spaces(ctx, captures[1])
	if err != nil {
		return err
	}
	if len(spaces) != 1 {
		return fmt.Errorf("the operator sees %d forge spaces named %s, want exactly one", len(spaces), captures[1])
	}
	return nil
}

func operatorSeesSpaces(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	want := captures[1]
	return waitFor(ctx, fmt.Sprintf("the operator never saw %s forge spaces", want), func() (bool, error) {
		spaces, err := w.allSpaces(ctx)
		if err != nil {
			return false, err
		}
		return fmt.Sprintf("%d", len(spaces)) == want, nil
	})
}

func operatorInvitedToSpace(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	return invitedTo(ctx, w, captures[1], true)
}

func operatorInvitedToRoom(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	return invitedTo(ctx, w, captures[1], false)
}

// invitedTo reports whether the bridge invited the operator: either the invite
// is still open, or the operator accepted it and is now in the room.
func invitedTo(ctx context.Context, w *World, name string, space bool) error {
	var rooms []string
	var err error
	if space {
		rooms, err = w.spaces(ctx, name)
	} else {
		rooms, err = w.rooms(ctx, name)
	}
	if err != nil {
		return err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	for _, roomID := range rooms {
		if operator.EverInvited(roomID) {
			return nil
		}
		membership, err := operator.Membership(ctx, roomID, w.operatorID)
		if err == nil && (membership == "invite" || membership == "join") {
			return nil
		}
	}
	return fmt.Errorf("the operator was never invited to %s", name)
}

func spaceHoldsRoom(_ context.Context, world any, captures []string) error {
	return expectChatRooms(world.(*World), captures[1], captures[2], false)
}

func spaceHoldsOneRoom(_ context.Context, world any, captures []string) error {
	return expectChatRooms(world.(*World), captures[1], captures[2], true)
}

func expectChatRooms(w *World, spaceName, roomName string, exactlyOne bool) error {
	ctx, cancel := stepContext()
	defer cancel()
	if _, err := w.waitForSpaceChild(ctx, spaceName, roomName); err != nil {
		return err
	}
	spaces, err := w.spaces(ctx, spaceName)
	if err != nil {
		return err
	}
	if len(spaces) == 0 {
		return fmt.Errorf("the operator does not see a forge space named %s", spaceName)
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	found := 0
	for _, spaceID := range spaces {
		children, err := operator.SpaceChildren(ctx, spaceID)
		if err != nil {
			return err
		}
		for _, child := range children {
			if name, err := operator.RoomName(ctx, child); err == nil && name == roomName {
				found++
			}
		}
	}
	if exactlyOne && found != 1 {
		return fmt.Errorf("the forge space %s holds %d chat rooms named %s, want exactly one", spaceName, found, roomName)
	}
	return nil
}

func chatRoomEncrypted(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomID, err := w.chatRoom(ctx, captures[1])
	if err != nil {
		return err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	algorithm, err := operator.EncryptionAlgorithm(ctx, roomID)
	if err != nil {
		return err
	}
	if algorithm != "m.megolm.v1.aes-sha2" {
		return fmt.Errorf("chat room %s is not encrypted: %q", captures[1], algorithm)
	}
	return nil
}
