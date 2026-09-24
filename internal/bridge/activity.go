package bridge

import (
	"context"
	"fmt"
	"strings"

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
// nothing else: the room stays quiet while nothing changes. A tick's news
// travels as one message, so what the phone sees is bounded by ticks rather
// than by the size of a board - and the cards that appeared or finished are
// named in it, so nothing is dropped. A tick's routine lane moves are one
// notice, which is what they always were: worth seeing in the room and not
// worth waking anyone for, even when they share a tick with news.
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
	news, moves := splitCardUpdates(actions)
	posted := map[string]bool{}
	if err := b.postCardNews(ctx, room.ActivityRoomID, news, posted); err != nil {
		return 0, err
	}
	if err := b.postCardMoves(ctx, room.ActivityRoomID, moves, posted); err != nil {
		return 0, err
	}
	if seeded := b.rememberQuietCards(cards, posted); seeded > 0 {
		if err := b.state.Save(b.statePath); err != nil {
			return 0, err
		}
	}
	return len(actions), nil
}

// postCardNews posts a tick's card news as one message - the message the phone
// is woken for - and remembers that the room has it.
func (b *Bridge) postCardNews(ctx context.Context, roomID string, actions []relay.ActivityAction, posted map[string]bool) error {
	if len(actions) == 0 {
		return nil
	}
	if _, err := b.rooms.SendText(ctx, roomID, cardUpdates(actions), ""); err != nil {
		return fmt.Errorf("post the tick's card news: %w", err)
	}
	return b.rememberCardUpdates(actions, posted)
}

// postCardMoves posts a tick's routine lane moves as one notice, which the
// clients do not notify on, and remembers that the room has it.
func (b *Bridge) postCardMoves(ctx context.Context, roomID string, actions []relay.ActivityAction, posted map[string]bool) error {
	if len(actions) == 0 {
		return nil
	}
	if _, err := b.rooms.SendNotice(ctx, roomID, cardUpdates(actions)); err != nil {
		return fmt.Errorf("post the tick's card moves: %w", err)
	}
	return b.rememberCardUpdates(actions, posted)
}

// splitCardUpdates keeps the tick's news apart from its routine moves: the news
// notifies and the moves do not, and sharing a tick must never promote a move
// into something that wakes the operator.
func splitCardUpdates(actions []relay.ActivityAction) (news, moves []relay.ActivityAction) {
	for _, action := range actions {
		if action.Kind == relay.CardMovedOn {
			moves = append(moves, action)
			continue
		}
		news = append(news, action)
	}
	return news, moves
}

// cardUpdates is what the operator reads for one group of card updates: a line
// each, in the order the board is in, so a tick with one card's news reads
// exactly as it did before the news was batched.
func cardUpdates(actions []relay.ActivityAction) string {
	lines := make([]string, 0, len(actions))
	for _, action := range actions {
		lines = append(lines, cardUpdate(action))
	}
	return strings.Join(lines, "\n")
}

// rememberCardUpdates records what the operator has been told about every card
// in a group, once that group is in the room: a restart then repeats nothing.
func (b *Bridge) rememberCardUpdates(actions []relay.ActivityAction, posted map[string]bool) error {
	b.state.Relay.EnsureMaps()
	for _, action := range actions {
		posted[action.Card.Key] = true
		b.state.Relay.Activity[action.Card.Key] = relay.CardState{
			Lane:     action.Card.Lane,
			Reported: string(action.Kind),
			Project:  action.Card.Project,
			Name:     action.Card.Name,
		}
	}
	return b.state.Save(b.statePath)
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
// {"version":1,"tested_at":"2026-09-24T16:48:15+02:00","module_hash":"94ceb6f3c608b8c144cbb14d03d8c4bc1b73796df55731c70e5f145a812e1459","functions":[{"id":"func/cardUpdate","name":"cardUpdate","line":13,"end_line":25,"hash":"c2a615621bb4fc84f739407ebe1c7dcd74f51548e5749984cf6cd7aae76cb270"},{"id":"func/Bridge.carryOutActivity","name":"Bridge.carryOutActivity","line":34,"end_line":59,"hash":"561cefbd055cd363f469f344afebaeb0f19a8b632ca0513dcb53270a5ccbaecf"},{"id":"func/Bridge.postCardNews","name":"Bridge.postCardNews","line":63,"end_line":71,"hash":"fec9ad990d30ec371c780b904f737b4b43c7d22a52583a3cf1bac28d517abfd0"},{"id":"func/Bridge.postCardMoves","name":"Bridge.postCardMoves","line":75,"end_line":83,"hash":"94779c2910f9013e99a9ae95b30025c719abca71114304588b24df0d7011c44a"},{"id":"func/splitCardUpdates","name":"splitCardUpdates","line":88,"end_line":97,"hash":"4620f53099784418e7d7ceedad005d2477009835aa82bc610b2ef326a89b9273"},{"id":"func/cardUpdates","name":"cardUpdates","line":102,"end_line":108,"hash":"555c1b76e5c103dc2b037dc4cd1d73d452f7aea7019f4906eecd50d7a88b7a16"},{"id":"func/Bridge.rememberCardUpdates","name":"Bridge.rememberCardUpdates","line":112,"end_line":124,"hash":"ec86323227b9181d1fca033afc2a02773657fcd1b444fe7d378c9b4402296cd5"},{"id":"func/Bridge.rememberQuietCards","name":"Bridge.rememberQuietCards","line":131,"end_line":150,"hash":"be1aaf8183103c4228e7c7340dbada838f2397dba12b593c0b40f5d97e19fd7f"}]}
// mutate4go-manifest-end
