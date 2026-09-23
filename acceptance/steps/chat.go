package steps

import (
	"context"
	"fmt"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
)

// settle is how long an "exactly one" step watches for a duplicate after the
// first copy arrives.
const settle = 1500 * time.Millisecond

func operatorDecryptsChatMessage(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomID, operator, message, err := w.waitForChatMessage(ctx, captures[1])
	if err != nil {
		return err
	}
	if !message.Encrypted {
		return fmt.Errorf("the chat message %q was not decrypted from an encrypted event", captures[1])
	}
	w.anchors[captures[1]] = message.EventID
	_ = operator
	_ = roomID
	return nil
}

func bridgeSentEncrypted(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomID, operator, message, err := w.waitForChatMessage(ctx, captures[1])
	if err != nil {
		return err
	}
	raw, err := operator.Client().Messages(ctx, id.RoomID(roomID), "", "", mautrix.DirectionBackward, nil, 50)
	if err != nil {
		return err
	}
	for _, evt := range raw.Chunk {
		if evt.ID.String() != message.EventID {
			continue
		}
		if evt.Type != event.EventEncrypted {
			return fmt.Errorf("the bridge sent %q as %s, want %s", captures[1], evt.Type, event.EventEncrypted)
		}
		return nil
	}
	return fmt.Errorf("the room holds no event %s for the chat message %q", message.EventID, captures[1])
}

func operatorDecryptsThreadReply(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	anchor, err := w.chatMessageAnchor(ctx, captures[2])
	if err != nil {
		return err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	roomID, err := w.chatRoom(ctx, "Chat")
	if err != nil {
		return err
	}
	reply, err := operator.WaitForThreadReply(ctx, roomID, anchor, captures[1], stepTimeout)
	if err != nil {
		return err
	}
	if !reply.Encrypted {
		return fmt.Errorf("the thread reply %q was not decrypted from an encrypted event", captures[1])
	}
	return nil
}

// chatMessageAnchor is the chat message a thread reply belongs under. The
// operator has to have read that message, so this waits for it.
func (w *World) chatMessageAnchor(ctx context.Context, body string) (string, error) {
	if anchor, ok := w.anchors[body]; ok {
		return anchor, nil
	}
	_, _, message, err := w.waitForChatMessage(ctx, body)
	if err != nil {
		return "", err
	}
	w.anchors[body] = message.EventID
	return message.EventID, nil
}

func operatorSendsMessage(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	return oneUserSends(w, w.operatorID, captures[2], captures[1])
}

func userSendsMessage(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	return oneUserSends(w, captures[1], captures[3], captures[2])
}

// oneUserSends has one Matrix user send a message into a chat room.
func oneUserSends(w *World, userID, roomName, body string) error {
	ctx, cancel := stepContext()
	defer cancel()
	user, err := w.user(ctx, userID)
	if err != nil {
		return err
	}
	roomID, err := w.chatRoom(ctx, roomName)
	if err != nil {
		return err
	}
	_, err = user.Send(ctx, roomID, body)
	return err
}

func userJoinedChatRoom(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomID, err := w.chatRoom(ctx, captures[2])
	if err != nil {
		return err
	}
	return w.addUserToRoom(ctx, captures[1], roomID, captures[2])
}

// operatorSwipesReplyToChatMessage sends the operator's message the way a phone
// does: by quoting the chat message, which carries the words back to the forge
// without the quote the phone wrote into the body.
func operatorSwipesReplyToChatMessage(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	anchor, err := w.chatMessageAnchor(ctx, captures[2])
	if err != nil {
		return err
	}
	roomID, err := w.chatRoom(ctx, "Chat")
	if err != nil {
		return err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	_, err = operator.SwipeReply(ctx, roomID, anchor, captures[1])
	return err
}

func roomHoldsOneChatMessage(_ context.Context, world any, captures []string) error {
	return expectMessages(world.(*World), captures[1], captures[2], false)
}

func roomHoldsOneThreadReply(_ context.Context, world any, captures []string) error {
	return expectMessages(world.(*World), captures[1], captures[2], true)
}

// expectMessages waits for a chat message, then checks that the room holds
// exactly one copy of it.
func expectMessages(w *World, roomName, body string, threadOnly bool) error {
	ctx, cancel := stepContext()
	defer cancel()
	roomID, err := w.chatRoom(ctx, roomName)
	if err != nil {
		return err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	if err := waitFor(ctx, fmt.Sprintf("chat room %s never held %q", roomName, body), func() (bool, error) {
		return countMessages(operator, roomID, body, threadOnly) >= 1, nil
	}); err != nil {
		return err
	}
	if err := fixtures.Sleep(ctx, settle); err != nil {
		return err
	}
	if found := countMessages(operator, roomID, body, threadOnly); found != 1 {
		kind := "chat messages"
		if threadOnly {
			kind = "thread replies"
		}
		return fmt.Errorf("chat room %s holds %d %s reading %q, want exactly one", roomName, found, kind, body)
	}
	return nil
}

// countMessages counts the messages in a room that read body. It ignores the
// ones that are not thread replies when threadOnly is set.
func countMessages(operator *fixtures.User, roomID, body string, threadOnly bool) int {
	found := 0
	for _, message := range operator.Messages(roomID) {
		if message.Body != body {
			continue
		}
		if threadOnly && message.ThreadRoot == "" {
			continue
		}
		found++
	}
	return found
}

// waitForChatMessage waits until the operator has decrypted a message.
func (w *World) waitForChatMessage(ctx context.Context, body string) (string, *fixtures.User, fixtures.Message, error) {
	operator, err := w.operator(ctx)
	if err != nil {
		return "", nil, fixtures.Message{}, err
	}
	roomID, err := w.chatRoom(ctx, "Chat")
	if err != nil {
		return "", nil, fixtures.Message{}, err
	}
	message, err := operator.WaitForMessage(ctx, roomID, body, stepTimeout)
	if err != nil {
		return "", nil, fixtures.Message{}, err
	}
	return roomID, operator, message, nil
}
