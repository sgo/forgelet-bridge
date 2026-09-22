package relay

import (
	"reflect"
	"testing"
)

func boardCard(lane string) Card {
	return Card{Key: "forgelet-bridge/card-activity-feed", Project: "forgelet-bridge", Name: "card-activity-feed", Lane: lane, Done: lane == "done"}
}

func TestPlanActivityReportsACardTheBridgeHasNotSeen(t *testing.T) {
	actions := PlanActivity(State{}, []Card{boardCard("specifier")})

	want := []ActivityAction{{Kind: CardAppeared, Card: boardCard("specifier")}}
	if !reflect.DeepEqual(actions, want) {
		t.Errorf("actions = %+v, want %+v", actions, want)
	}
}

func TestPlanActivityReportsACardThatMovedOn(t *testing.T) {
	state := State{Activity: map[string]CardState{
		"forgelet-bridge/card-activity-feed": {Lane: "specifier", Reported: ReportedAppeared},
	}}

	actions := PlanActivity(state, []Card{boardCard("coder")})

	want := []ActivityAction{{Kind: CardMovedOn, Card: boardCard("coder")}}
	if !reflect.DeepEqual(actions, want) {
		t.Errorf("actions = %+v, want %+v", actions, want)
	}
}

func TestPlanActivityReportsACardThatFinished(t *testing.T) {
	state := State{Activity: map[string]CardState{
		"forgelet-bridge/card-activity-feed": {Lane: "coder", Reported: ReportedMovedOn},
	}}

	actions := PlanActivity(state, []Card{boardCard("done")})

	want := []ActivityAction{{Kind: CardFinished, Card: boardCard("done")}}
	if !reflect.DeepEqual(actions, want) {
		t.Errorf("actions = %+v, want %+v", actions, want)
	}
}

func TestPlanActivityReportsACardFirstSeenAlreadyFinished(t *testing.T) {
	actions := PlanActivity(State{}, []Card{boardCard("done")})

	want := []ActivityAction{{Kind: CardFinished, Card: boardCard("done")}}
	if !reflect.DeepEqual(actions, want) {
		t.Errorf("actions = %+v, want %+v", actions, want)
	}
}

func TestPlanActivityStaysQuietWhenNothingChanged(t *testing.T) {
	state := State{Activity: map[string]CardState{
		"forgelet-bridge/card-activity-feed": {Lane: "coder", Reported: ReportedMovedOn},
	}}

	if actions := PlanActivity(state, []Card{boardCard("coder")}); len(actions) != 0 {
		t.Errorf("actions = %+v, want nothing at all", actions)
	}
}

func TestPlanActivityDoesNotRepeatAFinishedCard(t *testing.T) {
	state := State{Activity: map[string]CardState{
		"forgelet-bridge/card-activity-feed": {Lane: "done", Reported: ReportedFinished},
	}}

	if actions := PlanActivity(state, []Card{boardCard("done")}); len(actions) != 0 {
		t.Errorf("actions = %+v, want nothing after the card finished", actions)
	}
}

func TestPlanActivityKeepsCardsApart(t *testing.T) {
	state := State{Activity: map[string]CardState{
		"forgelet-bridge/card-activity-feed": {Lane: "specifier", Reported: ReportedAppeared},
	}}
	other := Card{Key: "forgelet-bridge/other-card", Project: "forgelet-bridge", Name: "other-card", Lane: "architect"}

	actions := PlanActivity(state, []Card{boardCard("specifier"), other})

	if len(actions) != 1 || actions[0].Card.Name != "other-card" {
		t.Errorf("actions = %+v, want only the other card", actions)
	}
}

func TestPlanActivityKeepsProjectsApart(t *testing.T) {
	state := State{Activity: map[string]CardState{
		"forgelet-bridge/card-activity-feed": {Lane: "specifier", Reported: ReportedAppeared},
	}}
	other := Card{Key: "saibill/card-activity-feed", Project: "saibill", Name: "card-activity-feed", Lane: "specifier"}

	actions := PlanActivity(state, []Card{other})

	if len(actions) != 1 || actions[0].Card.Project != "saibill" {
		t.Errorf("actions = %+v, want the card from the other project", actions)
	}
}
