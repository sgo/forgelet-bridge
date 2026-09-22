//go:build property

package relay

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"testing/quick"
)

// TestPropertyPlanActivityStaysQuietWhenNothingChanged is the promise the card
// is built on: while no card moves, the phone hears nothing, so the silence
// between updates keeps meaning an agent may be stuck.
func TestPropertyPlanActivityStaysQuietWhenNothingChanged(t *testing.T) {
	property := func(cards []Card) bool {
		st := State{}
		st.EnsureMaps()

		// Everything the forge holds has already been reported.
		for _, card := range cards {
			st.Activity[card.Key] = CardState{
				Lane:     card.Lane,
				Reported: reportedKind(card),
				Project:  card.Project,
				Name:     card.Name,
			}
		}
		return len(PlanActivity(st, cards)) == 0
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 300,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randomCards(rnd))
		},
	}); err != nil {
		t.Error(err)
	}
}

// TestPropertyPlanActivityReportsEveryChangeOnce is the other half: every card
// the operator has not heard about - one that appeared, one that moved lanes,
// one that finished - is reported, and carrying the plan out leaves nothing
// more to report, so a restart cannot replay an update.
func TestPropertyPlanActivityReportsEveryChangeOnce(t *testing.T) {
	property := func(st State, cards []Card) bool {
		st = cloneActivityState(st)
		before := cloneActivityState(st)

		planned := map[string]ActivityKind{}
		for _, action := range PlanActivity(st, cards) {
			if _, twice := planned[action.Card.Key]; twice {
				return false // a card is only reported once
			}
			planned[action.Card.Key] = action.Kind
			state := st.Activity[action.Card.Key]
			state.Lane = action.Card.Lane
			state.Reported = string(action.Kind)
			state.Project = action.Card.Project
			state.Name = action.Card.Name
			st.Activity[action.Card.Key] = state
		}

		for _, card := range cards {
			want, changed := changeFor(before, card)
			got, plannedFor := planned[card.Key]
			if changed != plannedFor || (changed && got != want) {
				return false
			}
		}

		// Carrying the plan out leaves nothing to report, so a restart that
		// reads the same board hears nothing again.
		return len(PlanActivity(st, cards)) == 0
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 300,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randomActivityState(rnd))
			values[1] = reflect.ValueOf(randomCards(rnd))
		},
	}); err != nil {
		t.Error(err)
	}
}

// TestPropertyPlanActivityKeepsTheBoardOrder checks that the updates reach the
// room in the order the board lists the cards, so the log reads in board order.
func TestPropertyPlanActivityKeepsTheBoardOrder(t *testing.T) {
	property := func(st State, cards []Card) bool {
		st = cloneActivityState(st)
		actions := PlanActivity(st, cards)

		at := 0
		for _, card := range cards {
			if at == len(actions) {
				break
			}
			if actions[at].Card.Key == card.Key {
				at++
			}
		}
		return at == len(actions)
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 300,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randomActivityState(rnd))
			values[1] = reflect.ValueOf(randomCards(rnd))
		},
	}); err != nil {
		t.Error(err)
	}
}

// changeFor is the update a card needs, and whether it needs one at all.
func changeFor(st State, card Card) (ActivityKind, bool) {
	known, seen := st.Activity[card.Key]
	switch {
	case !seen && card.Done:
		return CardFinished, true
	case !seen:
		return CardAppeared, true
	case card.Done && known.Reported != ReportedFinished:
		return CardFinished, true
	case !card.Done && known.Lane != card.Lane:
		return CardMovedOn, true
	}
	return "", false
}

// reportedKind is the update that leaves a card with nothing to report.
func reportedKind(card Card) string {
	if card.Done {
		return ReportedFinished
	}
	return ReportedMovedOn
}

func cloneActivityState(st State) State {
	cloned := State{Activity: map[string]CardState{}}
	for key, card := range st.Activity {
		cloned.Activity[key] = card
	}
	cloned.EnsureMaps()
	return cloned
}

func randomActivityState(rnd *rand.Rand) State {
	state := State{Activity: map[string]CardState{}}
	for count := rnd.Intn(4); count > 0; count-- {
		card := randomCard(rnd)
		state.Activity[card.Key] = CardState{
			Lane:     randomLane(rnd),
			Reported: randomReported(rnd),
			Project:  card.Project,
			Name:     card.Name,
		}
	}
	return state
}

func randomCards(rnd *rand.Rand) []Card {
	keys := []string{
		"forgelet-bridge/card-activity-feed",
		"forgelet-bridge/phone-approvals",
		"saibill/phone-approvals",
		"saibill/some-card",
	}
	rnd.Shuffle(len(keys), func(i, j int) { keys[i], keys[j] = keys[j], keys[i] })

	var cards []Card
	for _, key := range keys[:rnd.Intn(len(keys)+1)] {
		project, name, _ := strings.Cut(key, "/")
		cards = append(cards, Card{
			Key:     key,
			Project: project,
			Name:    name,
			Lane:    randomLane(rnd),
			Done:    rnd.Intn(3) == 0,
		})
	}
	return cards
}

func randomCard(rnd *rand.Rand) Card {
	projects := []string{"forgelet-bridge", "saibill", ""}
	names := []string{"card-activity-feed", "phone-approvals", "card-activity-feed", ""}
	project := projects[rnd.Intn(len(projects))]
	name := names[rnd.Intn(len(names))]
	return Card{
		Key:     project + "/" + name,
		Project: project,
		Name:    name,
		Lane:    randomLane(rnd),
		Done:    rnd.Intn(3) == 0,
	}
}

func randomLane(rnd *rand.Rand) string {
	lanes := []string{"", "specifier", "coder", "refactorer", doneLane}
	return lanes[rnd.Intn(len(lanes))]
}

// doneLane is the lane a finished card sits in, as the board names it.
const doneLane = "done"

func randomReported(rnd *rand.Rand) string {
	reported := []string{"", ReportedAppeared, ReportedMovedOn, ReportedFinished}
	return reported[rnd.Intn(len(reported))]
}
