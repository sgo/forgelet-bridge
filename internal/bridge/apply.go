package bridge

import (
	"context"
	"fmt"
	"sort"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

// postRequestMessage posts a forge chat request into the room and remembers
// the message its answer belongs under.
func (b *Bridge) postRequestMessage(ctx context.Context, root, project string, room Room, action relay.Action) error {
	eventID, err := b.rooms.SendText(ctx, room.RoomID, action.Body, "")
	if err != nil {
		return fmt.Errorf("post chat message for %s: %w", action.RequestID, err)
	}
	b.state.Relay.Threads[action.RequestID] = eventID
	b.scope(relay.ScopeThreads, action.RequestID, root, project)
	return nil
}

// postRequestReply posts the lieutenant's answer as a reply in the request's
// thread.
func (b *Bridge) postRequestReply(ctx context.Context, root, project string, room Room, action relay.Action) error {
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
	b.scope(relay.ScopeReplied, action.RequestID, root, project)
	return nil
}

// createForgeRequest hands the operator's message to the forge, which is what
// wakes the lieutenant. The forge answers that it took the message, not which
// request it became, so the pairing waits for the queue to show it.
func (b *Bridge) createForgeRequest(ctx context.Context, store ForgeStore, root, project string, action relay.Action) error {
	if _, err := store.CreateRequest(ctx, action.Body); err != nil {
		return fmt.Errorf("queue chat request for %s: %w", root, err)
	}
	b.state.Relay.EnsureMaps()
	b.state.Relay.Pending[action.SourceEventID] = action.Body
	b.scope(relay.ScopePending, action.SourceEventID, root, project)
	if action.SourceThread != "" {
		b.state.Relay.PendingThreads[action.SourceEventID] = action.SourceThread
		b.scope(relay.ScopePendingThreads, action.SourceEventID, root, project)
	}
	return nil
}

func (b *Bridge) scope(section, key, root, project string) {
	if project == "" {
		b.state.Relay.Scope(section, key, root)
		return
	}
	b.state.Relay.ScopeProject(section, key, root, project)
}

// pairPendingRequests pairs each message the forge has taken with the request
// it became, so the answer can be threaded under the operator's own message.
func (b *Bridge) pairPendingRequests(root, project string, store ForgeStore) (int, error) {
	b.state.Relay.EnsureMaps()
	if len(b.state.Relay.Pending) == 0 {
		return 0, nil
	}
	requests, err := store.Requests()
	if err != nil {
		return 0, fmt.Errorf("read the queue back: %w", err)
	}
	taken := b.takenRequests(root, project)
	paired := 0
	for _, eventID := range pendingEvents(b.state.Relay.Pending) {
		if !b.pendingBelongsTo(root, project, eventID) {
			continue
		}
		requestID, ok := takeRequest(requests, b.state.Relay.Pending[eventID], taken)
		if !ok {
			continue
		}
		taken[requestID] = true
		b.recordPair(root, project, eventID, requestID)
		paired++
	}
	if paired == 0 {
		return 0, nil
	}
	return paired, b.state.Save(b.statePath)
}

func (b *Bridge) takenRequests(root, project string) map[string]bool {
	taken := map[string]bool{}
	for eventID, requestID := range b.state.Relay.Relayed {
		if b.state.Relay.InScope(relay.ScopeRelayed, eventID, root) &&
			(project == "" || b.state.Relay.ProjectScopeOwner(relay.ScopeRelayed, eventID) == project) ||
			b.state.Relay.ScopeOwner(relay.ScopeRelayed, eventID) == "" {
			taken[requestID] = true
		}
	}
	return taken
}

func (b *Bridge) pendingBelongsTo(root, project, eventID string) bool {
	owner := b.state.Relay.ScopeOwner(relay.ScopePending, eventID)
	projectOwner := b.state.Relay.ProjectScopeOwner(relay.ScopePending, eventID)
	return owner == "" || owner == root && (project == "" || projectOwner == project)
}

func (b *Bridge) recordPair(root, project, eventID, requestID string) {
	b.state.Relay.Relayed[eventID] = requestID
	b.scope(relay.ScopeRelayed, eventID, root, project)
	// The answer belongs in the thread the operator wrote in. When their
	// message was itself a reply, that thread is where it sits, not the
	// message: a thread cannot start from an event that already carries a
	// relation, and the room answers 400 when we try.
	if thread := b.state.Relay.PendingThreads[eventID]; thread != "" {
		b.state.Relay.Threads[requestID] = thread
	} else {
		b.state.Relay.Threads[requestID] = eventID
	}
	b.scope(relay.ScopeThreads, requestID, root, project)
	delete(b.state.Relay.PendingThreads, eventID)
	b.state.Relay.Unscope(relay.ScopePendingThreads, eventID)
	delete(b.state.Relay.Pending, eventID)
	b.state.Relay.Unscope(relay.ScopePending, eventID)
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
// {"version":1,"tested_at":"2026-11-07T22:40:09+01:00","module_hash":"74a9762c05ec71a596f80aef109a1772f1057266e1eb5698c720b842dad977b5","functions":[{"id":"func/Bridge.postRequestMessage","name":"Bridge.postRequestMessage","line":13,"end_line":21,"hash":"272c28dd171a0c8caee30a351ed37b0d219adbbd492fb32f8ebc1b3ad381d2b8"},{"id":"func/Bridge.postRequestReply","name":"Bridge.postRequestReply","line":25,"end_line":40,"hash":"b8df845642dc2490d99ad0451f53ab4fb6721fe4f02736957682a8e45c883208"},{"id":"func/Bridge.createForgeRequest","name":"Bridge.createForgeRequest","line":45,"end_line":57,"hash":"a93dd5dca792fd87bb3fc0f34cbe8f465b6ffca56b6e2715a8e44cec5f269e71"},{"id":"func/Bridge.scope","name":"Bridge.scope","line":59,"end_line":65,"hash":"c47ad201d7f15141eca6476f84776cf574b0263a7d9c56094ee1e8ec496f6a58"},{"id":"func/Bridge.pairPendingRequests","name":"Bridge.pairPendingRequests","line":69,"end_line":96,"hash":"0e39f7adb5166c986cbd1376cfb688261c6f0001becada7a3a06c4fda8c610fe"},{"id":"func/Bridge.takenRequests","name":"Bridge.takenRequests","line":98,"end_line":108,"hash":"4766170c853e89d7f5363d49e5698bf45fc9bcab4b165c49d6e51df19c992075"},{"id":"func/Bridge.pendingBelongsTo","name":"Bridge.pendingBelongsTo","line":110,"end_line":114,"hash":"53c23fe7ab7dee07deac51698927928fd68cfe88b5c5f78d992d4dc9583d93ae"},{"id":"func/Bridge.recordPair","name":"Bridge.recordPair","line":116,"end_line":133,"hash":"a46ab12b9e4d10e362814f6e64e3dc5ec17b96304591b4509bda7e61e6be2c5c"},{"id":"func/pendingEvents","name":"pendingEvents","line":137,"end_line":144,"hash":"0addd14d4e12e79b889d0b0064d3abfa14af82a342b8e785cbb2d11d05df0ceb"},{"id":"func/takeRequest","name":"takeRequest","line":148,"end_line":155,"hash":"34e8a2cfb9ccda0290c931b0aa16c67783a39d3911dda62ad825c62d93d258d0"}]}
// mutate4go-manifest-end
