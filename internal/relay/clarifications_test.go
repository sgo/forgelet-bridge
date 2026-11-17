package relay

import (
	"reflect"
	"testing"
)

func clarification() Clarification {
	return Clarification{
		Key:       "forgelet-bridge/clar-1",
		ID:        "clar-1",
		Project:   "forgelet-bridge",
		Role:      "coder",
		Question:  "which lane should the refund card start in?",
		Source:    ClarificationSourceTyped,
		Lifecycle: ClarificationOpen,
		Attention: "unseen",
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

func TestPlanClarificationsDoesNotPostASeenTypedClarification(t *testing.T) {
	seen := clarification()
	seen.Attention = "seen"

	if actions := PlanClarifications(operator, State{}, []Clarification{seen}, nil); len(actions) != 0 {
		t.Errorf("actions = %+v, want no new notice for a seen typed clarification", actions)
	}
}

func TestPlanClarificationsDoesNotReportATypedDesktopEnding(t *testing.T) {
	typed := clarification()
	state := State{Clarifications: map[string]ClarificationState{
		typed.Key: {MessageID: "$message", Source: ClarificationSourceTyped},
	}}

	if actions := PlanClarifications(operator, state, nil, nil); len(actions) != 0 {
		t.Errorf("actions = %+v, want the terminal typed row cleared without a Matrix notice", actions)
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

func TestPlanClarificationsCarriesAReplyThatNamesTheStateEventBackAsTheAnswer(t *testing.T) {
	state := State{Clarifications: map[string]ClarificationState{
		"forgelet-bridge/clar-1": {MessageID: "$message", StateEventID: "$clarification-state"},
	}}
	replies := []RoomEvent{{
		EventID: "$reply",
		Sender:  operator,
		Body:    "> Clarification for forgelet-bridge from coder\n> Question: should the invoice card retry on its own?\n\nyes",
		ReplyTo: "$clarification-state",
	}}

	actions := PlanClarifications(operator, state, []Clarification{clarification()}, replies)

	want := []ClarificationAction{{
		Kind: AnswerClarification, Key: "forgelet-bridge/clar-1", Clarification: clarification(),
		Answer: "yes",
	}}
	if !reflect.DeepEqual(actions, want) {
		t.Errorf("actions = %+v, want the reply that names the state event carried back %+v", actions, want)
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

func TestPlanClarificationsMarksAnAnswerTheOperatorGaveOnThePhone(t *testing.T) {
	state := State{Clarifications: map[string]ClarificationState{
		"forgelet-bridge/clar-1": {MessageID: "$message", Answer: "yes"},
	}}

	actions := PlanClarifications(operator, state, []Clarification{clarification()}, nil)

	want := []ClarificationAction{{
		Kind: ReactClarification, Key: "forgelet-bridge/clar-1", MessageID: "$message", Reaction: CarriedOutReaction,
	}}
	if !reflect.DeepEqual(actions, want) {
		t.Errorf("actions = %+v, want the answer marked on the clarification's own message %+v", actions, want)
	}
}

func TestPlanClarificationsMarksAnAnswerOnlyOnce(t *testing.T) {
	state := State{Clarifications: map[string]ClarificationState{
		"forgelet-bridge/clar-1": {MessageID: "$message", Answer: "yes", ReactionID: "$reaction"},
	}}

	if actions := PlanClarifications(operator, state, []Clarification{clarification()}, nil); len(actions) != 0 {
		t.Errorf("actions = %+v, want no second mark", actions)
	}
}

func TestPlanClarificationsLeavesATypedDesktopAnswerToReconciliation(t *testing.T) {
	state := State{Clarifications: map[string]ClarificationState{
		"forgelet-bridge/clar-1": {MessageID: "$message"},
	}}

	actions := PlanClarifications(operator, state, nil, nil)

	if len(actions) != 0 {
		t.Errorf("actions = %+v, want no Matrix work for a typed desktop answer", actions)
	}
}

func TestPlanClarificationsAnswersOnceAndReportsOnce(t *testing.T) {
	state := State{Clarifications: map[string]ClarificationState{
		"forgelet-bridge/clar-1": {MessageID: "$message", Answer: "yes", ReactionID: "$bridge-reaction"},
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
		"forgelet-bridge/clar-1": {MessageID: "$message", Answer: "yes", ReactionID: "$bridge-reaction"},
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
