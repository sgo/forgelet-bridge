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

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-21T23:21:13+02:00","module_hash":"60cfc865bbce16cd05a046b7ecd302bbee89793fa91f4d762abe93ef5dee02d8","functions":[{"id":"func/Bridge.postRequestMessage","name":"Bridge.postRequestMessage","line":12,"end_line":19,"hash":"052960d9985c540dd7d3746816bf408c5f752037dc511a9155800b4966cbe57c"},{"id":"func/Bridge.postRequestReply","name":"Bridge.postRequestReply","line":23,"end_line":37,"hash":"b1e6e90a779873656c907eb073609f031afeb3aa4e428cd801b9c9c2047e4769"},{"id":"func/Bridge.createForgeRequest","name":"Bridge.createForgeRequest","line":41,"end_line":49,"hash":"bed62b85346e54e9e7aee203de39ba9b69f1198151461ac2429c2386c3ee30e1"}]}
// mutate4go-manifest-end
