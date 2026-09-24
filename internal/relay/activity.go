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
			// A card the bridge first meets already finished is history. A forge
			// joining reports from the moment it joined, so what it was already
			// carrying is remembered rather than narrated: a first appearance
			// must not produce a burst the size of the board it arrives with.
			continue
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
// {"version":1,"tested_at":"2026-09-24T14:42:05+02:00","module_hash":"c980098a39aa3bfcbe188188a9a4bb2caea107a08e9ffd4f846739e171a82965","functions":[{"id":"func/PlanActivity","name":"PlanActivity","line":50,"end_line":70,"hash":"7179c769e23fa562154ed95e5095feda885828ecadcba580951e508303b8c504"},{"id":"func/finishedNow","name":"finishedNow","line":74,"end_line":76,"hash":"44a3c3170fed35ac8423de273bfd38c7d393ce1c5d00b7d043aea9e74f2fae57"},{"id":"func/movedOn","name":"movedOn","line":79,"end_line":81,"hash":"2a9e1e2920311d1c109f8c1cedc2fab7b9b09ed1d52be7ab1776c267631d8dba"}]}
// mutate4go-manifest-end
