package bridge

import (
	"context"
	"fmt"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

// postRequestMessage posts a forge chat request into the room and remembers
// the message its answer belongs under.
func (b *Bridge) postRequestMessage(ctx context.Context, room Room, action relay.Action) error {
	eventID, err := b.rooms.SendText(ctx, room.RoomID, action.Body, "")
	if err != nil {
		return fmt.Errorf("post chat message for %s: %w", action.RequestID, err)
	}
	b.state.Relay.Threads[action.RequestID] = eventID
	return nil
}

// postRequestReply posts the lieutenant's answer as a reply in the request's
// thread.
func (b *Bridge) postRequestReply(ctx context.Context, room Room, action relay.Action) error {
	anchor := action.AnchorEventID
	if anchor == "" {
		anchor, _ = b.state.Relay.Anchor(action.RequestID)
	}
	if anchor == "" {
		return fmt.Errorf("no chat message to thread the answer of %s under", action.RequestID)
	}
	eventID, err := b.rooms.SendText(ctx, room.RoomID, action.Body, anchor)
	if err != nil {
		return fmt.Errorf("post chat reply for %s: %w", action.RequestID, err)
	}
	b.state.Relay.Replied[action.RequestID] = eventID
	return nil
}

// createForgeRequest hands the operator's message to the forge as a chat
// request, threaded under the operator's own message.
func (b *Bridge) createForgeRequest(store ForgeStore, root string, action relay.Action) error {
	requestID, err := store.CreateRequest(action.Body)
	if err != nil {
		return fmt.Errorf("queue chat request for %s: %w", root, err)
	}
	b.state.Relay.Relayed[action.SourceEventID] = requestID
	b.state.Relay.Threads[requestID] = action.SourceEventID
	return nil
}
