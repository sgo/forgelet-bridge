package steps

import (
	"context"
	"encoding/json"
)

// The rooms carry two events for one item: the message a client shows, and the
// state event that carries the item itself. A phone reads the rooms' state, so
// the state event is the one it can act on - a check reaction on it, or a reply
// that names it. The approvals room and the clarifications room are read and
// answered the same way, so that shape lives here once.

// stateQuote is what a phone shows for a state event it swipes: the event's own
// facts, as the JSON the room holds. The bridge reads only the operator's own
// words out of a reply, so this is the client's own rendering of the quote.
func stateQuote(content map[string]any) string {
	data, err := json.Marshal(content)
	if err != nil {
		return "the room's state event"
	}
	return string(data)
}

// stateEventToSwipe is the state event a phone reads and acts on for one item,
// with the quote a client shows for it when it swipes: the room's event under
// the item's own key, the same event the step reads as the item's fact.
func stateEventToSwipe(ctx context.Context, w *World, roomID, eventType, stateKey string) (eventID, quote string, err error) {
	fact, err := waitForFact(ctx, w, roomID, eventType, stateKey, func(map[string]any) error { return nil })
	if err != nil {
		return "", "", err
	}
	return fact.eventID, stateQuote(fact.content), nil
}

// replyToStateEvent sends the operator's decision as a reply that names the
// state event a phone swiped: their own words under the quote of that event.
func replyToStateEvent(ctx context.Context, w *World, roomID, eventID, quote, words string) error {
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	_, err = operator.SwipeReplyTo(ctx, roomID, eventID, quote, words)
	return err
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-10-07T01:39:46+02:00","module_hash":"eb5204ed9f02041f72e3c25fed97cea1a340324eab1368c4680fc641d0b6d2a9","functions":[{"id":"func/stateQuote","name":"stateQuote","line":17,"end_line":23,"hash":"00952c4fdfd9d789887d2d6f351c52df436f49afa2b13c9ce6d5082e38ffc8ba"},{"id":"func/stateEventToSwipe","name":"stateEventToSwipe","line":28,"end_line":34,"hash":"482d461977d81ff41b1da263dd425454db8ed6be354f2eb1c045a25434143633"},{"id":"func/replyToStateEvent","name":"replyToStateEvent","line":38,"end_line":45,"hash":"587132a00839961cd5bade3fb6a614bf4ced8e3bcbdb2795343f221b6ae5c9b2"}]}
// mutate4go-manifest-end
