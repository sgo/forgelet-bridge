package relay

// Card is one card on a forge project's board.
type Card struct {
	Key     string
	Project string
	Name    string
	Lane    string
	Done    bool
}

// CardState is what the bridge remembers about one card, so that the activity
// room only hears about real changes.
type CardState struct {
	Lane     string `json:"lane,omitempty"`
	Reported string `json:"reported,omitempty"`
	Project  string `json:"project,omitempty"`
	Name     string `json:"name,omitempty"`
}

// What the bridge has told the operator about a card.
const (
	ReportedAppeared = "appeared"
	ReportedMovedOn  = "moved_on"
	ReportedFinished = "finished"
)

// ActivityKind names the work a card update asks for.
type ActivityKind string

const (
	// CardAppeared reports a card the bridge has not seen before.
	CardAppeared ActivityKind = "appeared"
	// CardMovedOn reports a card that changed lane.
	CardMovedOn ActivityKind = "moved_on"
	// CardFinished reports a card that reached the done lane.
	CardFinished ActivityKind = "finished"
)

// ActivityAction is one card update for the bridge to post.
type ActivityAction struct {
	Kind ActivityKind
	Card Card
}

// PlanActivity works out which card updates the operator still has to hear.
// Only real changes produce an update: a card the bridge has not seen, a card
// that changed lane, and a card that finished. Anything else stays quiet, so
// the silence between updates keeps meaning something.
func PlanActivity(st State, cards []Card) []ActivityAction {
	var actions []ActivityAction
	for _, card := range cards {
		known, seen := st.Activity[card.Key]
		switch {
		case !seen && card.Done:
			// A card the bridge first meets already finished: the operator
			// only has to hear that it finished.
			actions = append(actions, ActivityAction{Kind: CardFinished, Card: card})
		case !seen:
			actions = append(actions, ActivityAction{Kind: CardAppeared, Card: card})
		case finishedNow(known, card):
			actions = append(actions, ActivityAction{Kind: CardFinished, Card: card})
		case movedOn(known, card):
			actions = append(actions, ActivityAction{Kind: CardMovedOn, Card: card})
		}
	}
	return actions
}

// finishedNow reports whether the operator still has to hear that a card the
// bridge has seen before has finished.
func finishedNow(known CardState, card Card) bool {
	return card.Done && known.Reported != ReportedFinished
}

// movedOn reports whether a card the bridge has seen before changed lane.
func movedOn(known CardState, card Card) bool {
	return !card.Done && known.Lane != card.Lane
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-22T16:13:59+02:00","module_hash":"e82b1672b5394af9fb44321fc83bef16ca7fe88b8d238987433793197eb82483","functions":[{"id":"func/PlanActivity","name":"PlanActivity","line":50,"end_line":68,"hash":"8adc6478827e40f36c7dd11e6b6e343fb3cc3d839be63ef9440606a89281d7dd"},{"id":"func/finishedNow","name":"finishedNow","line":72,"end_line":74,"hash":"44a3c3170fed35ac8423de273bfd38c7d393ce1c5d00b7d043aea9e74f2fae57"},{"id":"func/movedOn","name":"movedOn","line":77,"end_line":79,"hash":"2a9e1e2920311d1c109f8c1cedc2fab7b9b09ed1d52be7ab1776c267631d8dba"}]}
// mutate4go-manifest-end
