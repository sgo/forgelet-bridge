package bridge

import (
	"context"
	"fmt"
	"sort"

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

// createForgeRequest hands the operator's message to the forge, which is what
// wakes the lieutenant. The forge answers that it took the message, not which
// request it became, so the pairing waits for the queue to show it.
func (b *Bridge) createForgeRequest(ctx context.Context, store ForgeStore, root string, action relay.Action) error {
	if _, err := store.CreateRequest(ctx, action.Body); err != nil {
		return fmt.Errorf("queue chat request for %s: %w", root, err)
	}
	b.state.Relay.EnsureMaps()
	b.state.Relay.Pending[action.SourceEventID] = action.Body
	if action.SourceThread != "" {
		b.state.Relay.PendingThreads[action.SourceEventID] = action.SourceThread
	}
	return nil
}

// pairPendingRequests pairs each message the forge has taken with the request
// it became, so the answer can be threaded under the operator's own message.
func (b *Bridge) pairPendingRequests(store ForgeStore) (int, error) {
	b.state.Relay.EnsureMaps()
	if len(b.state.Relay.Pending) == 0 {
		return 0, nil
	}
	requests, err := store.Requests()
	if err != nil {
		return 0, fmt.Errorf("read the queue back: %w", err)
	}
	taken := map[string]bool{}
	for _, requestID := range b.state.Relay.Relayed {
		taken[requestID] = true
	}
	paired := 0
	for _, eventID := range pendingEvents(b.state.Relay.Pending) {
		requestID, ok := takeRequest(requests, b.state.Relay.Pending[eventID], taken)
		if !ok {
			continue
		}
		taken[requestID] = true
		b.state.Relay.Relayed[eventID] = requestID
		// The answer belongs in the thread the operator wrote in. When their
		// message was itself a reply, that thread is where it sits, not the
		// message: a thread cannot start from an event that already carries a
		// relation, and the room answers 400 when we try.
		if thread := b.state.Relay.PendingThreads[eventID]; thread != "" {
			b.state.Relay.Threads[requestID] = thread
		} else {
			b.state.Relay.Threads[requestID] = eventID
		}
		delete(b.state.Relay.PendingThreads, eventID)
		delete(b.state.Relay.Pending, eventID)
		paired++
	}
	if paired == 0 {
		return 0, nil
	}
	return paired, b.state.Save(b.statePath)
}

// pendingEvents is the messages waiting to be paired, in a stable order so the
// same state pairs the same way on every tick.
func pendingEvents(pending map[string]string) []string {
	events := make([]string, 0, len(pending))
	for eventID := range pending {
		events = append(events, eventID)
	}
	sort.Strings(events)
	return events
}

// takeRequest is the first request in the queue that reads body and that no
// message has taken yet.
func takeRequest(requests []relay.Request, body string, taken map[string]bool) (string, bool) {
	for _, request := range requests {
		if request.Body == body && !taken[request.ID] {
			return request.ID, true
		}
	}
	return "", false
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-23T20:55:55+02:00","module_hash":"2a8ad62015fbae7745e46ee428a6fa28d7e9e040a1410b47c3975a9be3b2ab7c","functions":[{"id":"func/Bridge.postRequestMessage","name":"Bridge.postRequestMessage","line":13,"end_line":20,"hash":"052960d9985c540dd7d3746816bf408c5f752037dc511a9155800b4966cbe57c"},{"id":"func/Bridge.postRequestReply","name":"Bridge.postRequestReply","line":24,"end_line":38,"hash":"b1e6e90a779873656c907eb073609f031afeb3aa4e428cd801b9c9c2047e4769"},{"id":"func/Bridge.createForgeRequest","name":"Bridge.createForgeRequest","line":43,"end_line":53,"hash":"37e2b48899f3a3dc0e74cdfb78642c83f29dc96d8acb4730eaff966a75a45fc5"},{"id":"func/Bridge.pairPendingRequests","name":"Bridge.pairPendingRequests","line":57,"end_line":95,"hash":"20dcd726a0cbd275e3d46f9dee1015cd28c46ef131235f09ec0b3b92b7be07c4"},{"id":"func/pendingEvents","name":"pendingEvents","line":99,"end_line":106,"hash":"0addd14d4e12e79b889d0b0064d3abfa14af82a342b8e785cbb2d11d05df0ceb"},{"id":"func/takeRequest","name":"takeRequest","line":110,"end_line":117,"hash":"34e8a2cfb9ccda0290c931b0aa16c67783a39d3911dda62ad825c62d93d258d0"}]}
// mutate4go-manifest-end
