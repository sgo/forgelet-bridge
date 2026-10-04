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
func (b *Bridge) carryOutFacts(ctx context.Context, roomID, factType string, facts []relay.Fact) (int, error) {
	b.state.Relay.EnsureMaps()
	desired := factsByKey(facts)

	written, err := b.writeChangedFacts(ctx, roomID, factType, desired)
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
// hold and remembers it, so the same facts twice are left alone.
func (b *Bridge) writeChangedFacts(ctx context.Context, roomID, factType string, desired map[string]relay.Fact) (int, error) {
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
		if _, err := b.rooms.SetState(ctx, roomID, factType, key, fact.Content); err != nil {
			return written, fmt.Errorf("write the %s state %s: %w", factType, key, err)
		}
		b.state.Relay.Waiting[remembered] = content
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
