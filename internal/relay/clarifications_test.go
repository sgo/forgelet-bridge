package relay

import (
	"reflect"
	"testing"
)

func clarification() Clarification {
	return Clarification{
		Key:      "forgelet-bridge/clar-1",
		ID:       "clar-1",
		Project:  "forgelet-bridge",
		Role:     "coder",
		Question: "which lane should the refund card start in?",
	}
}

func TestPlanClarificationsPostsEveryPendingClarification(t *testing.T) {
	actions := PlanClarifications(operator, State{}, []Clarification{clarification()}, nil)

	want := []ClarificationAction{{Kind: PostClarification, Key: "forgelet-bridge/clar-1", Clarification: clarification()}}
	if !reflect.DeepEqual(actions, want) {
		t.Errorf("actions = %+v, want %+v", actions, want)
	}
}

func TestPlanClarificationsPostsNothingTheRoomAlreadyHas(t *testing.T) {
	state := State{Clarifications: map[string]ClarificationState{"forgelet-bridge/clar-1": {MessageID: "$message"}}}

	if actions := PlanClarifications(operator, state, []Clarification{clarification()}, nil); len(actions) != 0 {
		t.Errorf("actions = %+v, want none for a clarification already in the room", actions)
	}
}

func TestPlanClarificationsCarriesTheReplyBackAsTheAnswer(t *testing.T) {
	state := State{Clarifications: map[string]ClarificationState{"forgelet-bridge/clar-1": {MessageID: "$message"}}}
	replies := []RoomEvent{{EventID: "$reply", Sender: operator, Body: "yes", ThreadRoot: "$message"}}

	actions := PlanClarifications(operator, state, []Clarification{clarification()}, replies)

	want := []ClarificationAction{{
		Kind: AnswerClarification, Key: "forgelet-bridge/clar-1", Clarification: clarification(), Answer: "yes",
	}}
	if !reflect.DeepEqual(actions, want) {
		t.Errorf("actions = %+v, want the reply carried back as the answer %+v", actions, want)
	}
}

func TestPlanClarificationsCarriesAQuotedReplyBackAsTheAnswer(t *testing.T) {
	state := State{Clarifications: map[string]ClarificationState{"forgelet-bridge/clar-1": {MessageID: "$message"}}}
	replies := []RoomEvent{{
		EventID: "$reply",
		Sender:  operator,
		Body:    "> Clarification for forgelet-bridge from coder\n> Question: which lane?\n\nyes",
		ReplyTo: "$message",
	}}

	actions := PlanClarifications(operator, state, []Clarification{clarification()}, replies)

	if len(actions) != 1 || actions[0].Kind != AnswerClarification || actions[0].Answer != "yes" {
		t.Errorf("actions = %+v, want the operator's own words as the answer", actions)
	}
}

func TestPlanClarificationsIgnoresAnAnswerFromAnyoneElse(t *testing.T) {
	state := State{Clarifications: map[string]ClarificationState{"forgelet-bridge/clar-1": {MessageID: "$message"}}}
	replies := []RoomEvent{{EventID: "$reply", Sender: "@stranger:example.org", Body: "yes", ThreadRoot: "$message"}}

	if actions := PlanClarifications(operator, state, []Clarification{clarification()}, replies); len(actions) != 0 {
		t.Errorf("actions = %+v, want none for a stranger's reply", actions)
	}
}

func TestPlanClarificationsIgnoresAPlainMessageInTheRoom(t *testing.T) {
	state := State{Clarifications: map[string]ClarificationState{"forgelet-bridge/clar-1": {MessageID: "$message"}}}
	replies := []RoomEvent{{EventID: "$message", Sender: operator, Body: "yes, skip it"}}

	if actions := PlanClarifications(operator, state, []Clarification{clarification()}, replies); len(actions) != 0 {
		t.Errorf("actions = %+v, want none for a message that answers nothing", actions)
	}
}

func TestPlanClarificationsReportsAnAnswerTheOperatorGaveOnThePhone(t *testing.T) {
	state := State{Clarifications: map[string]ClarificationState{
		"forgelet-bridge/clar-1": {MessageID: "$message", Answer: "yes"},
	}}

	actions := PlanClarifications(operator, state, nil, nil)

	want := []ClarificationAction{{
		Kind: ReplyClarification, Key: "forgelet-bridge/clar-1", MessageID: "$message", Text: "Answered",
	}}
	if !reflect.DeepEqual(actions, want) {
		t.Errorf("actions = %+v, want the answer reported in the thread %+v", actions, want)
	}
}

func TestPlanClarificationsReportsAClarificationTheDesktopAnswered(t *testing.T) {
	state := State{Clarifications: map[string]ClarificationState{
		"forgelet-bridge/clar-1": {MessageID: "$message"},
	}}

	actions := PlanClarifications(operator, state, nil, nil)

	want := []ClarificationAction{{
		Kind: ReplyClarification, Key: "forgelet-bridge/clar-1", MessageID: "$message", Text: "Resolved on the desktop",
	}}
	if !reflect.DeepEqual(actions, want) {
		t.Errorf("actions = %+v, want the desktop's answer reported %+v", actions, want)
	}
}

func TestPlanClarificationsAnswersOnceAndReportsOnce(t *testing.T) {
	state := State{Clarifications: map[string]ClarificationState{
		"forgelet-bridge/clar-1": {MessageID: "$message", Answer: "yes", ReplyID: "$bridge-reply"},
	}}

	if actions := PlanClarifications(operator, state, []Clarification{clarification()}, nil); len(actions) != 0 {
		t.Errorf("actions = %+v, want nothing left to do", actions)
	}
}

func TestPlanClarificationsDoesNotReportAnAnswerTwice(t *testing.T) {
	// The clarification was answered and the answer reported; the forge no
	// longer waits for it. A later tick must leave the thread alone rather than
	// report the same answer again.
	state := State{Clarifications: map[string]ClarificationState{
		"forgelet-bridge/clar-1": {MessageID: "$message", Answer: "yes", ReplyID: "$bridge-reply"},
	}}

	if actions := PlanClarifications(operator, state, nil, nil); len(actions) != 0 {
		t.Errorf("actions = %+v, want no second report for an answer already reported", actions)
	}
}

func TestPlanClarificationsAnswersOnlyOnce(t *testing.T) {
	state := State{Clarifications: map[string]ClarificationState{
		"forgelet-bridge/clar-1": {MessageID: "$message", Answer: "yes"},
	}}
	replies := []RoomEvent{{EventID: "$reply", Sender: operator, Body: "and also no", ThreadRoot: "$message"}}

	actions := PlanClarifications(operator, state, []Clarification{clarification()}, replies)

	for _, action := range actions {
		if action.Kind == AnswerClarification {
			t.Errorf("actions = %+v, want no second answer for the same clarification", actions)
		}
	}
}
