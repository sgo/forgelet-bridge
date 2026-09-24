package bridge

import (
	"context"
	"fmt"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

// cardUpdate is what the operator reads when a card moves through a project:
// the project, the card, and where it moved.
func cardUpdate(action relay.ActivityAction) string {
	card := action.Card
	switch action.Kind {
	case relay.CardAppeared:
		return fmt.Sprintf("card %s appeared in the project %s in the lane %s", card.Name, card.Project, card.Lane)
	case relay.CardMovedOn:
		return fmt.Sprintf("card %s moved on in the project %s to the lane %s", card.Name, card.Project, card.Lane)
	case relay.CardFinished:
		return fmt.Sprintf("card %s finished in the project %s", card.Name, card.Project)
	default:
		return ""
	}
}

// carryOutActivity posts the card updates the operator still has to hear, and
// nothing else: the room stays quiet while nothing changes. What is news - a
// card appearing and a card finishing - goes as an ordinary message, the kind
// clients notify on; the routine lane-to-lane step goes as a notice, worth
// seeing in the room and not worth waking anyone for.
func (b *Bridge) carryOutActivity(ctx context.Context, root string, room Room) (int, error) {
	store, ok := b.boards[root]
	if !ok {
		return 0, fmt.Errorf("no board configured for forge root %s", root)
	}
	cards, err := store.Cards()
	if err != nil {
		return 0, fmt.Errorf("read the board of %s: %w", root, err)
	}

	actions := relay.PlanActivity(b.state.Relay, cards)
	posted := map[string]bool{}
	for _, action := range actions {
		if err := b.postCardUpdate(ctx, room.ActivityRoomID, action); err != nil {
			return 0, fmt.Errorf("post the update for %s: %w", action.Card.Key, err)
		}
		posted[action.Card.Key] = true
		b.state.Relay.EnsureMaps()
		b.state.Relay.Activity[action.Card.Key] = relay.CardState{
			Lane:     action.Card.Lane,
			Reported: string(action.Kind),
			Project:  action.Card.Project,
			Name:     action.Card.Name,
		}
		if err := b.state.Save(b.statePath); err != nil {
			return 0, err
		}
	}
	if seeded := b.rememberQuietCards(cards, posted); seeded > 0 {
		if err := b.state.Save(b.statePath); err != nil {
			return 0, err
		}
	}
	return len(actions), nil
}

// postCardUpdate posts one card update as the kind of message it is: a card
// arriving or finishing notifies, a routine move between lanes does not.
func (b *Bridge) postCardUpdate(ctx context.Context, roomID string, action relay.ActivityAction) error {
	body := cardUpdate(action)
	if action.Kind == relay.CardMovedOn {
		_, err := b.rooms.SendNotice(ctx, roomID, body)
		return err
	}
	_, err := b.rooms.SendText(ctx, roomID, body, "")
	return err
}

// rememberQuietCards remembers the cards a forge arrived with that there was
// nothing to say about, so the board it arrives with is not narrated now and is
// not narrated on the next tick either. A card the bridge meets in flight is
// announced, so what it remembers quietly here is the history: cards that had
// already finished before the bridge got to them.
func (b *Bridge) rememberQuietCards(cards []relay.Card, posted map[string]bool) int {
	seeded := 0
	for _, card := range cards {
		if _, known := b.state.Relay.Activity[card.Key]; known {
			continue
		}
		if posted[card.Key] || !card.Done {
			continue
		}
		b.state.Relay.EnsureMaps()
		b.state.Relay.Activity[card.Key] = relay.CardState{
			Lane:     card.Lane,
			Reported: relay.ReportedFinished,
			Project:  card.Project,
			Name:     card.Name,
		}
		seeded++
	}
	return seeded
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-24T14:41:23+02:00","module_hash":"cfd6f01d47815c0a329d57be5c6c26c712e8eb1a9c6b2574929d4afe2413bb12","functions":[{"id":"func/cardUpdate","name":"cardUpdate","line":12,"end_line":24,"hash":"c2a615621bb4fc84f739407ebe1c7dcd74f51548e5749984cf6cd7aae76cb270"},{"id":"func/Bridge.carryOutActivity","name":"Bridge.carryOutActivity","line":31,"end_line":65,"hash":"8f8c3484c1f2e9f1f19125dd73ffff8947eaabd6299ab7e07706789047cec94e"},{"id":"func/Bridge.postCardUpdate","name":"Bridge.postCardUpdate","line":69,"end_line":77,"hash":"0c14925150103fe7276c52a86d051838de4d53db04e50914932417967dacbab9"},{"id":"func/Bridge.rememberQuietCards","name":"Bridge.rememberQuietCards","line":84,"end_line":103,"hash":"be1aaf8183103c4228e7c7340dbada838f2397dba12b593c0b40f5d97e19fd7f"}]}
// mutate4go-manifest-end
