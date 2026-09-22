package board

import (
	"fmt"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

// Queue is a forge's boards seen the way the bridge's relay needs them.
type Queue struct {
	Store *Store
}

// Cards lists the cards every open project holds.
func (q Queue) Cards() ([]relay.Card, error) {
	cards, err := q.Store.Cards()
	if err != nil {
		return nil, err
	}
	relayed := make([]relay.Card, 0, len(cards))
	for _, card := range cards {
		relayed = append(relayed, relay.Card{
			Key:     Key(card.Project, card.Name),
			Project: card.Project,
			Name:    card.Name,
			Lane:    card.Lane,
			Done:    card.Done(),
		})
	}
	return relayed, nil
}

// Key names a card across the bridge: project and card, so two projects cannot
// be confused.
func Key(project, name string) string {
	return fmt.Sprintf("%s/%s", project, name)
}
