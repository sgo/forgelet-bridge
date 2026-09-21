package steps

import (
	"context"
	"fmt"
)

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

// expectChatRooms checks the chat rooms a forge space holds, waiting for the
// room to show up first.
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
	found, err := countChildren(ctx, w, spaces, roomName)
	if err != nil {
		return err
	}
	if exactlyOne && found != 1 {
		return fmt.Errorf("the forge space %s holds %d chat rooms named %s, want exactly one", spaceName, found, roomName)
	}
	return nil
}

// countChildren counts the chat rooms named roomName across the given spaces.
func countChildren(ctx context.Context, w *World, spaceIDs []string, roomName string) (int, error) {
	operator, err := w.operator(ctx)
	if err != nil {
		return 0, err
	}
	found := 0
	for _, spaceID := range spaceIDs {
		children, err := operator.SpaceChildren(ctx, spaceID)
		if err != nil {
			return 0, err
		}
		for _, child := range children {
			if name, err := operator.RoomName(ctx, child); err == nil && name == roomName {
				found++
			}
		}
	}
	return found, nil
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
