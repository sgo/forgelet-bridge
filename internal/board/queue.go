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

// Cards lists the cards every open project holds.
func (q Queue) Cards() ([]relay.Card, error) {
	cards, err := q.Store.Cards()
	if err != nil {
		return nil, err
	}
	relayed := make([]relay.Card, 0, len(cards))
	for _, card := range cards {
		relayed = append(relayed, relay.Card{
			Key:     Key(q.Forge, card.Project, card.Name),
			Project: card.Project,
			Name:    card.Name,
			Lane:    card.Lane,
			Done:    card.Done(),
		})
	}
	return relayed, nil
}

// Key names a card across the bridge: the forge, the project and the card, so
// neither two projects nor two forges can be confused.
func Key(forge, project, name string) string {
	return fmt.Sprintf("%s/%s/%s", forge, project, name)
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-22T16:08:47+02:00","module_hash":"569311f25fbf0c36e1d740c5be069c682d8f36ecb1547e069a2e83f608b9a0bc","functions":[{"id":"func/Queue.Cards","name":"Queue.Cards","line":15,"end_line":31,"hash":"8af945e1f2fe9680b2bf887a256be3883051c6c8c954c0a40ea4b2b358e62710"},{"id":"func/Key","name":"Key","line":35,"end_line":37,"hash":"ac132cedb0aea369e3970faa4b4db252fc9b3227684c5b9292b321a23456f149"}]}
// mutate4go-manifest-end
