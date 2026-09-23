package relay

import (
	"reflect"
	"testing"
)

func TestPlanSendsOnlyTheOperatorOwnWordsWhenTheMessageQuotesOne(t *testing.T) {
	events := []RoomEvent{{
		EventID: "$operator-message",
		Sender:  operator,
		Body:    "> is the build green?\n\nyes, the build is green",
		ReplyTo: "$forge-message",
	}}

	actions := Plan(operator, State{}, nil, events)

	want := []Action{{Kind: CreateForgeRequest, Body: "yes, the build is green", SourceEventID: "$operator-message"}}
	if !reflect.DeepEqual(actions, want) {
		t.Errorf("actions = %+v, want the operator's own words %+v", actions, want)
	}
}

func TestPlanThreadsTheAnswerUnderTheMessageTheOperatorQuoted(t *testing.T) {
	events := []RoomEvent{
		{EventID: "$forge-message", Sender: "@bridge:example.org", Body: "is the build green?"},
		{
			EventID: "$operator-message",
			Sender:  operator,
			Body:    "> is the build green?\n\nyes, the build is green",
			ReplyTo: "$forge-message",
		},
	}

	actions := Plan(operator, State{}, nil, events)

	want := []Action{{
		Kind:          CreateForgeRequest,
		Body:          "yes, the build is green",
		SourceEventID: "$operator-message",
		SourceThread:  "$forge-message",
	}}
	if !reflect.DeepEqual(actions, want) {
		t.Errorf("actions = %+v, want the answer threaded under the quoted message %+v", actions, want)
	}
}

func TestPlanThreadsTheAnswerUnderTheThreadTheOperatorQuotedIn(t *testing.T) {
	events := []RoomEvent{
		{EventID: "$forge-message", Sender: "@bridge:example.org", Body: "is the build green?", ThreadRoot: "$root"},
		{
			EventID: "$operator-message",
			Sender:  operator,
			Body:    "> is the build green?\n\nyes, the build is green",
			ReplyTo: "$forge-message",
		},
	}

	actions := Plan(operator, State{}, nil, events)

	if len(actions) != 1 || actions[0].SourceThread != "$root" {
		t.Errorf("actions = %+v, want the answer in the thread the quoted message sits in", actions)
	}
}

func TestPlanSaysNothingForAQuoteTheOperatorDidNotAnswer(t *testing.T) {
	events := []RoomEvent{{
		EventID: "$operator-message",
		Sender:  operator,
		Body:    "> is the build green?\n",
		ReplyTo: "$forge-message",
	}}

	if actions := Plan(operator, State{}, nil, events); len(actions) != 0 {
		t.Errorf("actions = %+v, want none for a quote with no words", actions)
	}
}
