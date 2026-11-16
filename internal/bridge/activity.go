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
	return b.carryOutActivityBoards(ctx, root, room, boards)
}

// activityFrom uses boards from the tick's one projection when a snapshot
// source exists, and the independent board store for adapters without one.
func (b *Bridge) activityFrom(ctx context.Context, root string, room Room, snapshot relay.DashboardSnapshot, fromSnapshot bool) (int, error) {
	if !fromSnapshot || snapshot.Boards == nil {
		return b.carryOutActivity(ctx, root, room)
	}
	return b.carryOutActivityBoards(ctx, root, room, snapshot.Boards)
}

// carryOutActivityBoards posts card updates and board facts from one board
// view, regardless of which public adapter produced that view.
func (b *Bridge) carryOutActivityBoards(ctx context.Context, root string, room Room, boards []relay.Board) (int, error) {
	cards := cardsOf(boards)

	actions := relay.PlanActivity(b.state.Relay, cards)
	news, moves := splitCardUpdates(actions)
	if err := b.postCardNews(ctx, root, room.ActivityRoomID, news); err != nil {
		return 0, err
	}
	if err := b.postCardMoves(ctx, root, room.ActivityRoomID, moves); err != nil {
		return 0, err
	}
	if err := b.saveQuietCards(root, cards); err != nil {
		return 0, err
	}
	written, err := b.carryOutFacts(ctx, room.ActivityRoomID, relay.BoardFactType, boardFacts(boards), nil)
	if err != nil {
		return written, err
	}
	return len(actions) + written, nil
}

func (b *Bridge) saveQuietCards(root string, cards []relay.Card) error {
	if b.rememberQuietCards(root, cards) > 0 {
		return b.state.Save(b.statePath)
	}
	return nil
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
func (b *Bridge) postCardNews(ctx context.Context, root, roomID string, actions []relay.ActivityAction) error {
	if len(actions) == 0 {
		return nil
	}
	if _, err := b.rooms.SendText(ctx, roomID, cardUpdates(actions), ""); err != nil {
		return fmt.Errorf("post the tick's card news: %w", err)
	}
	return b.rememberCardUpdates(root, actions)
}

// postCardMoves posts a tick's routine lane moves as one notice, which the
// clients do not notify on, and remembers that the room has it.
func (b *Bridge) postCardMoves(ctx context.Context, root, roomID string, actions []relay.ActivityAction) error {
	if len(actions) == 0 {
		return nil
	}
	if _, err := b.rooms.SendNotice(ctx, roomID, cardUpdates(actions)); err != nil {
		return fmt.Errorf("post the tick's card moves: %w", err)
	}
	return b.rememberCardUpdates(root, actions)
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
func (b *Bridge) rememberCardUpdates(root string, actions []relay.ActivityAction) error {
	b.state.Relay.EnsureMaps()
	for _, action := range actions {
		b.state.Relay.Activity[action.Card.Key] = relay.CardState{
			ForgeRoot: root,
			Lane:      action.Card.Lane,
			Reported:  string(action.Kind),
			Project:   action.Card.Project,
			Name:      action.Card.Name,
		}
	}
	return b.state.Save(b.statePath)
}

// rememberQuietCards remembers the cards a forge arrived with that there was
// nothing to say about, so the board it arrives with is not narrated now and is
// not narrated on the next tick either. A card the bridge meets in flight is
// announced, so what it remembers quietly here is the history: cards that had
// already finished before the bridge got to them.
func (b *Bridge) rememberQuietCards(root string, cards []relay.Card) int {
	seeded := 0
	for _, card := range cards {
		if _, known := b.state.Relay.Activity[card.Key]; known {
			continue
		}
		if !card.Done {
			continue
		}
		b.state.Relay.EnsureMaps()
		b.state.Relay.Activity[card.Key] = relay.CardState{
			ForgeRoot: root,
			Lane:      card.Lane,
			Reported:  relay.ReportedFinished,
			Project:   card.Project,
			Name:      card.Name,
		}
		seeded++
	}
	return seeded
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-11-16T15:14:19+01:00","module_hash":"da1167ca604f4bd921fd6e546f375e1d463020fad35608eb09b761348797ebe0","functions":[{"id":"func/cardUpdate","name":"cardUpdate","line":13,"end_line":25,"hash":"c2a615621bb4fc84f739407ebe1c7dcd74f51548e5749984cf6cd7aae76cb270"},{"id":"func/Bridge.carryOutActivity","name":"Bridge.carryOutActivity","line":34,"end_line":44,"hash":"267f456ba3b04fb26ca7e7c816cc1d474034bda5add9e022ef7fcc6562b3dc76"},{"id":"func/Bridge.activityFrom","name":"Bridge.activityFrom","line":48,"end_line":53,"hash":"7ee72662637ccf023cbbe763f21c4cf870633e807fd75c12a9c88d89afc599b8"},{"id":"func/Bridge.carryOutActivityBoards","name":"Bridge.carryOutActivityBoards","line":57,"end_line":76,"hash":"bdcb23a0f6164692360ba890124b4d9f920bf09918a2e2d266ad63647a7e8f01"},{"id":"func/Bridge.saveQuietCards","name":"Bridge.saveQuietCards","line":78,"end_line":83,"hash":"9a61fd2d4e3771fad22a658b17288bf6a5c8cecb5eaeb47d2afab160db0cf9e5"},{"id":"func/cardsOf","name":"cardsOf","line":87,"end_line":93,"hash":"5186c3160df27a8cf7ff74ec5eae2232ff829c974fbd7efc9751960fa2b1ec12"},{"id":"func/boardFacts","name":"boardFacts","line":97,"end_line":103,"hash":"97b90b0e2cab2cb546d31a6bd8c250251e2ea094a8aac40e2a18d02403287248"},{"id":"func/Bridge.postCardNews","name":"Bridge.postCardNews","line":107,"end_line":115,"hash":"1c6188ac909bf9c911fedde2fe15eb205359e086f0847cbf907c73c377ae63aa"},{"id":"func/Bridge.postCardMoves","name":"Bridge.postCardMoves","line":119,"end_line":127,"hash":"f853887095b0fae50c6c3973fca587a20e00f09cc1086637eee7a8bcad9e89c0"},{"id":"func/splitCardUpdates","name":"splitCardUpdates","line":132,"end_line":141,"hash":"4620f53099784418e7d7ceedad005d2477009835aa82bc610b2ef326a89b9273"},{"id":"func/cardUpdates","name":"cardUpdates","line":146,"end_line":152,"hash":"555c1b76e5c103dc2b037dc4cd1d73d452f7aea7019f4906eecd50d7a88b7a16"},{"id":"func/Bridge.rememberCardUpdates","name":"Bridge.rememberCardUpdates","line":156,"end_line":168,"hash":"3afff224687bcfce9e3d0a313fa2eb7a833ceedeb43bd831d66870c48244b03c"},{"id":"func/Bridge.rememberQuietCards","name":"Bridge.rememberQuietCards","line":175,"end_line":195,"hash":"67645e20e42acdf56a4e9cb2c3f54b3437315edcffc588348c23d1b23305c0a8"}]}
// mutate4go-manifest-end
