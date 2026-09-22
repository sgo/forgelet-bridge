package relay

import (
	"reflect"
	"testing"
)

const operator = "@operator:example.org"

func TestPlanPostsUnansweredForgeRequest(t *testing.T) {
	actions := Plan(operator, State{}, []Request{{ID: "req-1", Body: "is the build green?"}}, nil)

	want := []Action{{Kind: PostRequestMessage, RequestID: "req-1", Body: "is the build green?"}}
	if !reflect.DeepEqual(actions, want) {
		t.Errorf("actions = %+v, want %+v", actions, want)
	}
}

func TestPlanPostsMessageThenReplyForAlreadyAnsweredRequest(t *testing.T) {
	requests := []Request{{ID: "req-1", Body: "is the build green?", Response: "yes, the build is green"}}

	actions := Plan(operator, State{}, requests, nil)

	want := []Action{
		{Kind: PostRequestMessage, RequestID: "req-1", Body: "is the build green?"},
		{Kind: PostRequestReply, RequestID: "req-1", Body: "yes, the build is green"},
	}
	if !reflect.DeepEqual(actions, want) {
		t.Errorf("actions = %+v, want %+v", actions, want)
	}
}

func TestPlanRepliesInThreadOfPostedMessage(t *testing.T) {
	state := State{Threads: map[string]string{"req-1": "$message"}}
	requests := []Request{{ID: "req-1", Body: "is the build green?", Response: "yes, the build is green"}}

	actions := Plan(operator, state, requests, nil)

	want := []Action{{Kind: PostRequestReply, RequestID: "req-1", Body: "yes, the build is green", AnchorEventID: "$message"}}
	if !reflect.DeepEqual(actions, want) {
		t.Errorf("actions = %+v, want %+v", actions, want)
	}
}

func TestPlanRepeatsNothingAfterARestart(t *testing.T) {
	state := State{
		Threads: map[string]string{"req-1": "$message"},
		Replied: map[string]string{"req-1": "$reply"},
		Relayed: map[string]string{"$operator-message": "req-2"},
	}
	requests := []Request{
		{ID: "req-1", Body: "is the build green?", Response: "yes, the build is green"},
		{ID: "req-2", Body: "is the build green?"},
	}
	events := []RoomEvent{{EventID: "$operator-message", Sender: operator, Body: "is the build green?"}}

	if actions := Plan(operator, state, requests, events); len(actions) != 0 {
		t.Errorf("actions = %+v, want none after a restart", actions)
	}
}

func TestPlanTurnsOperatorMessageIntoForgeRequest(t *testing.T) {
	events := []RoomEvent{{EventID: "$operator-message", Sender: operator, Body: "is the build green?"}}

	actions := Plan(operator, State{}, nil, events)

	want := []Action{{Kind: CreateForgeRequest, Body: "is the build green?", SourceEventID: "$operator-message"}}
	if !reflect.DeepEqual(actions, want) {
		t.Errorf("actions = %+v, want %+v", actions, want)
	}
}

func TestPlanIgnoresMessagesFromAnyoneElse(t *testing.T) {
	events := []RoomEvent{{EventID: "$stranger-message", Sender: "@stranger:example.org", Body: "is the build green?"}}

	if actions := Plan(operator, State{}, nil, events); len(actions) != 0 {
		t.Errorf("actions = %+v, want none for a stranger's message", actions)
	}
}

func TestPlanIgnoresEmptyMessages(t *testing.T) {
	events := []RoomEvent{{EventID: "$operator-message", Sender: operator, Body: "   "}}

	if actions := Plan(operator, State{}, nil, events); len(actions) != 0 {
		t.Errorf("actions = %+v, want none for an empty message", actions)
	}
}

func TestPlanRepliesUnderTheOperatorsOwnMessage(t *testing.T) {
	state := State{
		Threads: map[string]string{"req-1": "$operator-message"},
		Relayed: map[string]string{"$operator-message": "req-1"},
	}
	requests := []Request{{ID: "req-1", Body: "is the build green?", Response: "yes, the build is green"}}

	actions := Plan(operator, state, requests, nil)

	want := []Action{{
		Kind:          PostRequestReply,
		RequestID:     "req-1",
		Body:          "yes, the build is green",
		AnchorEventID: "$operator-message",
	}}
	if !reflect.DeepEqual(actions, want) {
		t.Errorf("actions = %+v, want %+v", actions, want)
	}
}

func TestPlanKeepsEachRequestInItsOwnThread(t *testing.T) {
	state := State{
		Threads: map[string]string{"req-1": "$one"},
		Replied: map[string]string{"req-1": "$one-reply"},
	}
	requests := []Request{
		{ID: "req-1", Body: "is the build green?", Response: "yes, the build is green"},
		{ID: "req-2", Body: "please retry the invoice card", Response: "retried, the card is queued"},
	}

	actions := Plan(operator, state, requests, nil)

	want := []Action{
		{Kind: PostRequestMessage, RequestID: "req-2", Body: "please retry the invoice card"},
		{Kind: PostRequestReply, RequestID: "req-2", Body: "retried, the card is queued"},
	}
	if !reflect.DeepEqual(actions, want) {
		t.Errorf("actions = %+v, want %+v", actions, want)
	}
}

func TestPlanPreservesRequestOrder(t *testing.T) {
	requests := []Request{{ID: "req-1", Body: "one"}, {ID: "req-2", Body: "two"}}

	actions := Plan(operator, State{}, requests, nil)

	if len(actions) != 2 || actions[0].RequestID != "req-1" || actions[1].RequestID != "req-2" {
		t.Errorf("actions = %+v, want forge request order", actions)
	}
}

func TestStateAnchorReportsOnlyKnownThreads(t *testing.T) {
	state := State{Threads: map[string]string{"req-1": "$message", "req-2": ""}}

	if anchor, ok := state.Anchor("req-1"); !ok || anchor != "$message" {
		t.Errorf("Anchor(req-1) = %q, %v", anchor, ok)
	}
	if _, ok := state.Anchor("req-2"); ok {
		t.Error("Anchor(req-2) reported an unknown thread as known")
	}
	if _, ok := state.Anchor("req-3"); ok {
		t.Error("Anchor(req-3) reported a missing thread as known")
	}
}

func TestEnsureMapsMakesStateWritable(t *testing.T) {
	var state State
	state.EnsureMaps()
	state.Threads["req-1"] = "$message"

	if state.Threads["req-1"] != "$message" {
		t.Errorf("threads = %+v, want the recorded anchor", state.Threads)
	}
}

func TestEnsureMapsKeepsTheWorkAlreadyRecorded(t *testing.T) {
	state := State{
		Threads: map[string]string{"req-1": "$message"},
		Replied: map[string]string{"req-1": "$reply"},
		Relayed: map[string]string{"$operator-message": "req-1"},
		Approvals: map[string]ApprovalState{
			"forgelet-bridge/approval-1": {MessageID: "$approval", Resolution: ResolutionApproved},
		},
	}

	state.EnsureMaps()

	if state.Threads["req-1"] != "$message" ||
		state.Replied["req-1"] != "$reply" ||
		state.Relayed["$operator-message"] != "req-1" ||
		state.Approvals["forgelet-bridge/approval-1"].MessageID != "$approval" {
		t.Errorf("state = %+v, want the recorded work kept: a restart must not forget it", state)
	}
}
