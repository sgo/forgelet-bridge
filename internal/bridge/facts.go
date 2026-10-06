package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

// carryOutFacts writes the waiting facts one room carries as state: an item
// whose facts changed has its event written again, an item that stopped waiting
// has its event cleared with empty content, and a fact the room already holds is
// left alone, so a restart writes nothing the rooms already have. It reports how
// many events it wrote or cleared.
//
// Every event it writes is handed to remember with the item's own state key, so
// a caller can keep the event's id - the phone reads the room's state, and the
// id is what a signal on that event names. A caller with nothing to remember
// passes nil.
func (b *Bridge) carryOutFacts(ctx context.Context, roomID, factType string, facts []relay.Fact, remember func(stateKey, eventID string)) (int, error) {
	b.state.Relay.EnsureMaps()
	desired := factsByKey(facts)

	written, err := b.writeChangedFacts(ctx, roomID, factType, desired, remember)
	if err != nil {
		return written, err
	}
	cleared, err := b.clearStoppedFacts(ctx, roomID, factType, desired)
	if err != nil {
		return written + cleared, err
	}
	if written+cleared == 0 {
		return 0, nil
	}
	if err := b.state.Save(b.statePath); err != nil {
		return written + cleared, err
	}
	return written + cleared, nil
}

// factsByKey indexes the facts a room should carry by their state key, so a
// fact the room already holds is compared with the one that replaces it.
func factsByKey(facts []relay.Fact) map[string]relay.Fact {
	desired := make(map[string]relay.Fact, len(facts))
	for _, fact := range facts {
		desired[fact.Key] = fact
	}
	return desired
}

// writeChangedFacts writes the state of every fact the room does not already
// hold and remembers it, so the same facts twice are left alone. Each event it
// writes is handed to remember with the item's state key.
func (b *Bridge) writeChangedFacts(ctx context.Context, roomID, factType string, desired map[string]relay.Fact, remember func(stateKey, eventID string)) (int, error) {
	written := 0
	for key, fact := range desired {
		content, err := factContent(fact)
		if err != nil {
			return written, err
		}
		remembered := waitingKey(roomID, factType, key)
		if b.state.Relay.Waiting[remembered] == content {
			continue
		}
		eventID, err := b.rooms.SetState(ctx, roomID, factType, key, fact.Content)
		if err != nil {
			return written, fmt.Errorf("write the %s state %s: %w", factType, key, err)
		}
		b.state.Relay.Waiting[remembered] = content
		if remember != nil {
			remember(key, eventID)
		}
		written++
	}
	return written, nil
}

// clearStoppedFacts clears the state of every fact this room carried that is no
// longer waiting, so reading the room's state never shows a stale item.
func (b *Bridge) clearStoppedFacts(ctx context.Context, roomID, factType string, desired map[string]relay.Fact) (int, error) {
	cleared := 0
	for remembered := range b.state.Relay.Waiting {
		room, typ, key, ok := splitWaitingKey(remembered)
		if !ok || room != roomID || typ != factType {
			continue
		}
		if _, waiting := desired[key]; waiting {
			continue
		}
		if _, err := b.rooms.SetState(ctx, roomID, factType, key, map[string]any{}); err != nil {
			return cleared, fmt.Errorf("clear the %s state %s: %w", factType, key, err)
		}
		delete(b.state.Relay.Waiting, remembered)
		cleared++
	}
	return cleared, nil
}

// waitingKey names one state event the bridge wrote, across the room it is in,
// the event type it carries and the item's own state key.
func waitingKey(roomID, factType, stateKey string) string {
	return roomID + "\n" + factType + "\n" + stateKey
}

// splitWaitingKey reads a waiting key back into the room, the event type and
// the state key it names.
func splitWaitingKey(key string) (roomID, factType, stateKey string, ok bool) {
	parts := strings.SplitN(key, "\n", 3)
	if len(parts) != 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}

// factContent is a fact's content as the one string two readings can be
// compared by, so the same facts twice are left alone.
func factContent(fact relay.Fact) (string, error) {
	data, err := json.Marshal(fact.Content)
	if err != nil {
		return "", fmt.Errorf("read the %s facts %s: %w", fact.Type, fact.Key, err)
	}
	return string(data), nil
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-10-04T12:04:28+02:00","module_hash":"e39f31e8f721174c8beb813fe33fce6cb76ae1def78f725312dbe0444b9d3141","functions":[{"id":"func/Bridge.carryOutFacts","name":"Bridge.carryOutFacts","line":17,"end_line":36,"hash":"f3b4917ca368c93942929f8fb3eb70aece1fc91d73f72a7ecab93087ddf5ca77"},{"id":"func/factsByKey","name":"factsByKey","line":40,"end_line":46,"hash":"fa79828aeccc46f24e7be9cfcb8aa051598617ceab47365d837e674cd5d55299"},{"id":"func/Bridge.writeChangedFacts","name":"Bridge.writeChangedFacts","line":50,"end_line":68,"hash":"f95128dee862979c6de1e5d4eaaf9b38afb9c99451813a0184fb6e2dd01db9f4"},{"id":"func/Bridge.clearStoppedFacts","name":"Bridge.clearStoppedFacts","line":72,"end_line":89,"hash":"1f52a582ceca2efdb3b1913712c1a2fe5d636609bdaa5393c80dbcca61badeab"},{"id":"func/waitingKey","name":"waitingKey","line":93,"end_line":95,"hash":"833943aa7cb9039a7e8c03cfb2b93606f6c148ad1d07f2d22663b62ce5046345"},{"id":"func/splitWaitingKey","name":"splitWaitingKey","line":99,"end_line":105,"hash":"11856d210d296c701b616263b72148c70924e61cdb069426e0bcd8c7964828c1"},{"id":"func/factContent","name":"factContent","line":109,"end_line":115,"hash":"8644f465369dc921b59ffc04b31abb3e54abeda090830a0aa8f5825e42c9c676"}]}
// mutate4go-manifest-end
