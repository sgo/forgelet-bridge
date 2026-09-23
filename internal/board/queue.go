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
// {"version":1,"tested_at":"2026-09-23T14:24:40+02:00","module_hash":"a003e46195211e37719349156a967417d51064baa14f8acc6fc4de94634025bd","functions":[{"id":"func/Queue.Cards","name":"Queue.Cards","line":19,"end_line":35,"hash":"085c5a72c3ee970500db9b1a4cfe774e5fa0f24b7350fb3f28886b1a9a383316"},{"id":"func/Key","name":"Key","line":39,"end_line":41,"hash":"e694bf36e44ea1acf47fa1699bd1468d307f1c17d8373e7d878f2e2c2e8b3e54"}]}
// mutate4go-manifest-end
