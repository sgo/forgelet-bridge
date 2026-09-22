package steps

import (
	"context"
	"fmt"
	"path/filepath"
)

// namedForge configures a fixture forge root with the name the operator knows
// it by.
func namedForge(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	if err := w.nameForge(captures[1], captures[2]); err != nil {
		return err
	}
	// The configuration a start reads is the one on disk, so a name the
	// operator gives a forge has to be written there.
	ctx, cancel := stepContext()
	defer cancel()
	return w.configure(ctx)
}

// noForgeSpace checks that no forge space carries a name: the name of the
// folder a forge happens to live in must never reach the operator.
func noForgeSpace(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()

	// The folder has to be one of the fixtures the scenario declared. Asking
	// about the folder of a forge that is not there would pass for the wrong
	// reason: a name no fixture uses is trivially absent.
	if _, err := w.declaredForge(captures[1]); err != nil {
		return err
	}

	// The bridge has usually shown its spaces by now; wait for one so that an
	// empty list is not mistaken for a clean one.
	if err := waitFor(ctx, "the operator never saw a forge space", func() (bool, error) {
		spaces, err := w.spacesNow(ctx)
		if err != nil {
			return false, err
		}
		return len(spaces) > 0, nil
	}); err != nil {
		return err
	}

	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	spaces, err := w.spacesNow(ctx)
	if err != nil {
		return err
	}
	for _, spaceID := range spaces {
		name, err := operator.RoomName(ctx, spaceID)
		if err == nil && name == captures[1] {
			return fmt.Errorf("the operator sees a forge space named %s", captures[1])
		}
	}
	return nil
}

// forgeSentChatMessage waits for a forge's chat room to hold a message the
// bridge posted under that forge's name.
func forgeSentChatMessage(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	return w.expectForgeSender(ctx, captures[2], captures[3], captures[1])
}

// forgeSentThreadReply does the same for the answer in a chat message's thread.
func forgeSentThreadReply(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()

	forgeName, body, senderName := captures[2], captures[1], captures[3]
	anchor, err := w.forgeChatMessageAnchor(ctx, forgeName, captures[4])
	if err != nil {
		return err
	}
	roomID, err := w.forgeChatRoom(ctx, forgeName)
	if err != nil {
		return err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	if _, err := operator.WaitForThreadReply(ctx, roomID, anchor, body, stepTimeout); err != nil {
		return err
	}
	return w.expectSender(ctx, roomID, senderName)
}

// expectForgeSender waits for a forge's chat room to hold a message and checks
// who the bridge appears as.
func (w *World) expectForgeSender(ctx context.Context, forgeName, senderName, body string) error {
	roomID, err := w.forgeChatRoom(ctx, forgeName)
	if err != nil {
		return err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	if _, err := operator.WaitForMessage(ctx, roomID, body, stepTimeout); err != nil {
		return err
	}
	return w.expectSender(ctx, roomID, senderName)
}

// expectSender checks the name the bridge shows up under in a room.
func (w *World) expectSender(ctx context.Context, roomID, senderName string) error {
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	shown, err := operator.DisplayName(ctx, roomID, w.bridgeUserID)
	if err != nil {
		return err
	}
	if shown != senderName {
		return fmt.Errorf("the bridge's messages in %s are sent under the name %q, want %q", roomID, shown, senderName)
	}
	return nil
}

// forgeChatRoom is the chat room inside the space a forge is named after.
func (w *World) forgeChatRoom(ctx context.Context, forgeName string) (string, error) {
	return w.waitForSpaceChild(ctx, forgeName, "Chat")
}

// forgeChatMessageAnchor is the chat message a reply in a forge's room belongs
// under, waiting for the operator to have read it.
func (w *World) forgeChatMessageAnchor(ctx context.Context, forgeName, body string) (string, error) {
	key := filepath.Join(forgeName, body)
	if anchor, ok := w.anchors[key]; ok {
		return anchor, nil
	}
	roomID, err := w.forgeChatRoom(ctx, forgeName)
	if err != nil {
		return "", err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return "", err
	}
	message, err := operator.WaitForMessage(ctx, roomID, body, stepTimeout)
	if err != nil {
		return "", err
	}
	w.anchors[key] = message.EventID
	return message.EventID, nil
}
