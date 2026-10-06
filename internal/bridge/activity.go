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
	boards, err := store.Boards()
	if err != nil {
		return 0, fmt.Errorf("read the board of %s: %w", root, err)
	}
	cards := cardsOf(boards)

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
	written, err := b.carryOutFacts(ctx, room.ActivityRoomID, relay.BoardFactType, boardFacts(boards), nil)
	if err != nil {
		return written, err
	}
	return len(actions) + written, nil
}

// cardsOf is every card the forge's open boards hold, in board order, which is
// what the activity planner names.
func cardsOf(boards []relay.Board) []relay.Card {
	var cards []relay.Card
	for _, board := range boards {
		cards = append(cards, board.Cards...)
	}
	return cards
}

// boardFacts is every open project's board as the waiting fact its room
// carries: the lanes the project runs and the cards each lane holds.
func boardFacts(boards []relay.Board) []relay.Fact {
	facts := make([]relay.Fact, 0, len(boards))
	for _, board := range boards {
		facts = append(facts, relay.BoardFact(board.Project, board.Lanes, board.Cards))
	}
	return facts
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
// {"version":1,"tested_at":"2026-10-04T12:07:14+02:00","module_hash":"a716f822ed16bfeeb779e8052a95a86692f88a513ef881607fe318d6e577f619","functions":[{"id":"func/cardUpdate","name":"cardUpdate","line":13,"end_line":25,"hash":"c2a615621bb4fc84f739407ebe1c7dcd74f51548e5749984cf6cd7aae76cb270"},{"id":"func/Bridge.carryOutActivity","name":"Bridge.carryOutActivity","line":34,"end_line":64,"hash":"5d91907286f5e7c5018245e675f38ed48895cbdb1247c4beea3866f4c4c61554"},{"id":"func/cardsOf","name":"cardsOf","line":68,"end_line":74,"hash":"5186c3160df27a8cf7ff74ec5eae2232ff829c974fbd7efc9751960fa2b1ec12"},{"id":"func/boardFacts","name":"boardFacts","line":78,"end_line":84,"hash":"97b90b0e2cab2cb546d31a6bd8c250251e2ea094a8aac40e2a18d02403287248"},{"id":"func/Bridge.postCardNews","name":"Bridge.postCardNews","line":88,"end_line":96,"hash":"fec9ad990d30ec371c780b904f737b4b43c7d22a52583a3cf1bac28d517abfd0"},{"id":"func/Bridge.postCardMoves","name":"Bridge.postCardMoves","line":100,"end_line":108,"hash":"94779c2910f9013e99a9ae95b30025c719abca71114304588b24df0d7011c44a"},{"id":"func/splitCardUpdates","name":"splitCardUpdates","line":113,"end_line":122,"hash":"4620f53099784418e7d7ceedad005d2477009835aa82bc610b2ef326a89b9273"},{"id":"func/cardUpdates","name":"cardUpdates","line":127,"end_line":133,"hash":"555c1b76e5c103dc2b037dc4cd1d73d452f7aea7019f4906eecd50d7a88b7a16"},{"id":"func/Bridge.rememberCardUpdates","name":"Bridge.rememberCardUpdates","line":137,"end_line":149,"hash":"ec86323227b9181d1fca033afc2a02773657fcd1b444fe7d378c9b4402296cd5"},{"id":"func/Bridge.rememberQuietCards","name":"Bridge.rememberQuietCards","line":156,"end_line":175,"hash":"be1aaf8183103c4228e7c7340dbada838f2397dba12b593c0b40f5d97e19fd7f"}]}
// mutate4go-manifest-end
