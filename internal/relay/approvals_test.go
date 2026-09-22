package relay

import (
	"reflect"
	"testing"
)

const card = "phone-approvals"

func approval() Approval {
	return Approval{
		Key:       "forgelet-bridge/approval-1",
		Project:   "forgelet-bridge",
		ID:        "approval-1",
		Card:      card,
		Gate:      "coder → refactorer",
		Artifacts: []string{"internal/bridge/bridge.go", "internal/relay/relay.go"},
	}
}

func posted() State {
	return State{Approvals: map[string]ApprovalState{
		"forgelet-bridge/approval-1": {MessageID: "$approval-message"},
	}}
}

func TestPlanApprovalsPostsAnApprovalTheRoomHasNotSeen(t *testing.T) {
	actions := PlanApprovals(operator, State{}, []Approval{approval()}, nil, nil)

	if len(actions) != 1 || actions[0].Kind != PostApproval || actions[0].Key != "forgelet-bridge/approval-1" {
		t.Fatalf("actions = %+v, want one post", actions)
	}
	if actions[0].Approval.Project != "forgelet-bridge" || actions[0].Approval.Card != card {
		t.Errorf("approval = %+v, want the project and card", actions[0].Approval)
	}
}

func TestPlanApprovalsPostsAnApprovalOnlyOnce(t *testing.T) {
	actions := PlanApprovals(operator, posted(), []Approval{approval()}, nil, nil)

	if len(actions) != 0 {
		t.Errorf("actions = %+v, want nothing to post twice", actions)
	}
}

func TestPlanApprovalsCarriesTheOperatorsApprovalBack(t *testing.T) {
	reactions := []Reaction{{Sender: operator, Key: ApproveReaction, TargetEventID: "$approval-message"}}

	actions := PlanApprovals(operator, posted(), []Approval{approval()}, reactions, nil)

	want := []ApprovalAction{{
		Kind:       ResolveApproval,
		Key:        "forgelet-bridge/approval-1",
		Approval:   approval(),
		Resolution: ResolutionApproved,
	}}
	if !reflect.DeepEqual(actions, want) {
		t.Errorf("actions = %+v, want %+v", actions, want)
	}
}

func TestPlanApprovalsIgnoresEveryoneElsesReactions(t *testing.T) {
	reactions := []Reaction{
		{Sender: "@stranger:example.org", Key: ApproveReaction, TargetEventID: "$approval-message"},
		{Sender: operator, Key: "🎉", TargetEventID: "$approval-message"},
	}

	if actions := PlanApprovals(operator, posted(), []Approval{approval()}, reactions, nil); len(actions) != 0 {
		t.Errorf("actions = %+v, want nothing from a reaction that is not the operator's approval", actions)
	}
}

func TestPlanApprovalsIgnoresAReactionForAnotherMessage(t *testing.T) {
	reactions := []Reaction{{Sender: operator, Key: ApproveReaction, TargetEventID: "$something-else"}}

	if actions := PlanApprovals(operator, posted(), []Approval{approval()}, reactions, nil); len(actions) != 0 {
		t.Errorf("actions = %+v, want nothing", actions)
	}
}

func TestPlanApprovalsSendsTheApprovalBackWithTheOperatorsReply(t *testing.T) {
	replies := []RoomEvent{{
		EventID:    "$reply",
		Sender:     operator,
		Body:       "the timesheet total is still wrong",
		ThreadRoot: "$approval-message",
	}}

	actions := PlanApprovals(operator, posted(), []Approval{approval()}, nil, replies)

	want := []ApprovalAction{{
		Kind:       ResolveApproval,
		Key:        "forgelet-bridge/approval-1",
		Approval:   approval(),
		Resolution: ResolutionSentBack,
		Feedback:   "the timesheet total is still wrong",
	}}
	if !reflect.DeepEqual(actions, want) {
		t.Errorf("actions = %+v, want %+v", actions, want)
	}
}

func TestPlanApprovalsIgnoresAMessageThatIsNotInTheApprovalsThread(t *testing.T) {
	replies := []RoomEvent{{EventID: "$plain", Sender: operator, Body: "when is this due?"}}

	if actions := PlanApprovals(operator, posted(), []Approval{approval()}, nil, replies); len(actions) != 0 {
		t.Errorf("actions = %+v, want a plain message to decide nothing", actions)
	}
}

func TestPlanApprovalsIgnoresABlankReplyInTheApprovalsThread(t *testing.T) {
	replies := []RoomEvent{{EventID: "$blank", Sender: operator, Body: "   ", ThreadRoot: "$approval-message"}}

	if actions := PlanApprovals(operator, posted(), []Approval{approval()}, nil, replies); len(actions) != 0 {
		t.Errorf("actions = %+v, want a blank reply to decide nothing", actions)
	}
}

func TestPlanApprovalsDecidesOnlyOnce(t *testing.T) {
	state := posted()
	state.Approvals["forgelet-bridge/approval-1"] = ApprovalState{
		MessageID:  "$approval-message",
		Resolution: ResolutionApproved,
	}
	reactions := []Reaction{{Sender: operator, Key: ApproveReaction, TargetEventID: "$approval-message"}}
	replies := []RoomEvent{{Sender: operator, Body: "one more thing", ThreadRoot: "$approval-message"}}

	if actions := PlanApprovals(operator, state, []Approval{approval()}, reactions, replies); len(actions) != 0 {
		t.Errorf("actions = %+v, want nothing after the approval is decided", actions)
	}
}

func TestPlanApprovalsReportsTheOperatorsOwnResolution(t *testing.T) {
	state := posted()
	state.Approvals["forgelet-bridge/approval-1"] = ApprovalState{
		MessageID:  "$approval-message",
		Resolution: ResolutionApproved,
	}

	actions := PlanApprovals(operator, state, nil, nil, nil)

	want := []ApprovalAction{{
		Kind:       ReplyApproval,
		Key:        "forgelet-bridge/approval-1",
		MessageID:  "$approval-message",
		Resolution: ResolutionApproved,
		Text:       "Approved",
	}}
	if !reflect.DeepEqual(actions, want) {
		t.Errorf("actions = %+v, want %+v", actions, want)
	}
}

func TestPlanApprovalsReportsAResolutionFromTheDesktop(t *testing.T) {
	actions := PlanApprovals(operator, posted(), nil, nil, nil)

	if len(actions) != 1 || actions[0].Kind != ReplyApproval {
		t.Fatalf("actions = %+v, want one reply", actions)
	}
	if actions[0].Text != "Resolved on the desktop" || actions[0].Resolution != ResolutionDesktop {
		t.Errorf("reply = %+v, want the desktop wording", actions[0])
	}
}

func TestPlanApprovalsReportsEachResolutionOnce(t *testing.T) {
	state := posted()
	state.Approvals["forgelet-bridge/approval-1"] = ApprovalState{
		MessageID:  "$approval-message",
		Resolution: ResolutionSentBack,
		ReplyID:    "$reply",
	}

	if actions := PlanApprovals(operator, state, nil, nil, nil); len(actions) != 0 {
		t.Errorf("actions = %+v, want no second reply", actions)
	}
}

func TestPlanApprovalsKeepsApprovalsApart(t *testing.T) {
	other := Approval{Key: "forgelet-bridge/approval-2", Project: "forgelet-bridge", ID: "approval-2", Card: "another-card"}
	actions := PlanApprovals(operator, posted(), []Approval{approval(), other}, nil, nil)

	if len(actions) != 1 || actions[0].Key != "forgelet-bridge/approval-2" {
		t.Errorf("actions = %+v, want only the approval the room has not seen", actions)
	}
}

func TestApprovalsReply(t *testing.T) {
	cases := map[string]string{
		ResolutionApproved: "Approved",
		ResolutionSentBack: "Sent back with feedback",
		ResolutionDesktop:  "Resolved on the desktop",
		"":                 "Resolved on the desktop",
	}
	for resolution, want := range cases {
		if got := ApprovalsReply(resolution); got != want {
			t.Errorf("ApprovalsReply(%q) = %q, want %q", resolution, got, want)
		}
	}
}
