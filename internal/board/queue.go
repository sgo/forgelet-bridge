package board

import (
	"fmt"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

// Queue is a forge's boards seen the way the bridge's relay needs them.
type Queue struct {
	Store *Store
	// Forge is the forge these boards belong to. The bridge remembers what it
	// said about a card per forge, so two forges holding the same project and
	// card are two cards, not one.
	Forge string
}

// Boards lists every open project's board the way the bridge's relay needs it:
// the project, its lanes, and its cards, each card keyed across the bridge.
func (q Queue) Boards() ([]relay.Board, error) {
	boards, err := q.Store.Boards()
	if err != nil {
		return nil, err
	}
	relayed := make([]relay.Board, 0, len(boards))
	for _, board := range boards {
		cards := make([]relay.Card, 0, len(board.Cards))
		for _, card := range board.Cards {
			cards = append(cards, relayCard(q.Forge, card))
		}
		relayed = append(relayed, relay.Board{Project: board.Project, Lanes: board.Lanes, Cards: cards})
	}
	return relayed, nil
}

// relayCard is one card the way the bridge's relay needs it: keyed across the
// forge, with the lane it is in and whether it is done.
func relayCard(forge string, card Card) relay.Card {
	return relay.Card{
		Key:     Key(forge, card.Project, card.Name),
		Project: card.Project,
		Name:    card.Name,
		Lane:    card.Lane,
		Done:    card.Done(),
	}
}

// Key names a card across the bridge: the forge, the project and the card, so
// neither two projects nor two forges can be confused.
func Key(forge, project, name string) string {
	return fmt.Sprintf("%s/%s/%s", forge, project, name)
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-10-04T11:49:34+02:00","module_hash":"ea013ec382b288bc8ab8badad30200bd444c618761563e52d649c9eb7b6314a0","functions":[{"id":"func/Queue.Boards","name":"Queue.Boards","line":20,"end_line":34,"hash":"c7613aeed773dcb082f04f370d481fb594c64a1924a832824c5004d55017f360"},{"id":"func/relayCard","name":"relayCard","line":38,"end_line":46,"hash":"aa26c46fd0646298de1056003182f547ef3e2d776578b55a7af8fb0f8e54b510"},{"id":"func/Key","name":"Key","line":50,"end_line":52,"hash":"e694bf36e44ea1acf47fa1699bd1468d307f1c17d8373e7d878f2e2c2e8b3e54"}]}
// mutate4go-manifest-end
