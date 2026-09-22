//go:build property

package relay

import (
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"
)

// TestPropertyPlanApprovalsDecidesOnce is the promise the phone depends on:
// once the bridge has carried out an approvals plan - the message is posted,
// the decision is carried back to the forge, the thread reply is recorded - the
// same events decide nothing again, however often they are replayed. The room's
// answer to a message it cannot read is not a decision and asks for nothing
// durable, and the events themselves are drained once, so it cannot repeat.
func TestPropertyPlanApprovalsActsOnce(t *testing.T) {
	property := func(st State, pending []Approval, reactions []Reaction, replies []RoomEvent) bool {
		st = cloneState(st)
		pending = append([]Approval(nil), pending...)

		actions := PlanApprovals(operator, st, pending, reactions, replies)
		pending = carryOutApprovals(&st, pending, actions)

		decided := map[string]bool{}
		for key, state := range st.Approvals {
			if state.Resolution != "" {
				decided[key] = true
			}
		}
		for _, action := range PlanApprovals(operator, st, pending, reactions, replies) {
			if action.Kind == ResolveApproval && decided[action.Key] {
				return false
			}
		}
		return true
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 300,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randomState(rnd))
			values[1] = reflect.ValueOf(randomApprovals(rnd))
			values[2] = reflect.ValueOf(randomReactions(rnd))
			values[3] = reflect.ValueOf(randomApprovalReplies(rnd))
		},
	}); err != nil {
		t.Error(err)
	}
}

// TestPropertyPlanApprovalsNeedsTheOperatorsOwnTap is the allowlist rule: no
// reaction from anyone else, and no other reaction key, ever decides an
// approval.
func TestPropertyPlanApprovalsNeedsTheOperatorsOwnTap(t *testing.T) {
	property := func(reaction Reaction) bool {
		if reaction.Sender == operator && reaction.Key == ApproveReaction {
			return true
		}
		state := State{Approvals: map[string]ApprovalState{
			"forgelet-bridge/approval-1": {MessageID: "$approval-message"},
		}}
		reaction.TargetEventID = "$approval-message"
		pending := []Approval{{Key: "forgelet-bridge/approval-1", Project: "forgelet-bridge", ID: "approval-1"}}

		for _, action := range PlanApprovals(operator, state, pending, []Reaction{reaction}, nil) {
			if action.Kind == ResolveApproval {
				return false
			}
		}
		return true
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 300,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randomReaction(rnd))
		},
	}); err != nil {
		t.Error(err)
	}
}

// TestPropertyPlanApprovalsRepliesNeedTheApprovalsThread is the same rule for
// sending a card back: a message of the operator's that is not a thread reply
// under a known approval decides nothing.
func TestPropertyPlanApprovalsRepliesNeedTheApprovalsThread(t *testing.T) {
	property := func(reply RoomEvent) bool {
		if reply.Sender == operator && reply.ThreadRoot == approvalMessageID {
			return true
		}
		state := State{Approvals: map[string]ApprovalState{
			approvalKey: {MessageID: approvalMessageID},
		}}
		pending := []Approval{{Key: approvalKey, Project: "forgelet-bridge", ID: "approval-1"}}

		for _, action := range PlanApprovals(operator, state, pending, nil, []RoomEvent{reply}) {
			if action.Kind == ResolveApproval {
				return false
			}
		}
		return true
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 300,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randomApprovalReply(rnd))
		},
	}); err != nil {
		t.Error(err)
	}
}

const approvalMessageID = "$approval-message"

const approvalKey = "forgelet-bridge/approval-1"

// carryOutApprovals records the effects an approvals plan has on the bridge's
// state and on the forge, the way a tick does, and returns the approvals the
// forge is still waiting for.
func carryOutApprovals(st *State, pending []Approval, actions []ApprovalAction) []Approval {
	st.EnsureMaps()
	for _, action := range actions {
		switch action.Kind {
		case PostApproval:
			state := st.Approvals[action.Key]
			state.MessageID = "$posted-" + action.Key
			st.Approvals[action.Key] = state
		case ResolveApproval:
			state := st.Approvals[action.Key]
			state.Resolution = action.Resolution
			st.Approvals[action.Key] = state
			pending = removeApproval(pending, action.Key)
		case ReplyApproval:
			state := st.Approvals[action.Key]
			state.Resolution = action.Resolution
			state.ReplyID = "$reply-" + action.Key
			st.Approvals[action.Key] = state
		}
	}
	return pending
}

// removeApproval drops an approval the forge no longer waits for.
func removeApproval(pending []Approval, key string) []Approval {
	kept := make([]Approval, 0, len(pending))
	for _, approval := range pending {
		if approval.Key != key {
			kept = append(kept, approval)
		}
	}
	return kept
}

func randomApprovals(rnd *rand.Rand) []Approval {
	keys := []string{"", approvalKey, "forgelet-bridge/approval-2", "saibill/approval-1"}
	var pending []Approval
	for count := rnd.Intn(4); count > 0; count-- {
		key := keys[rnd.Intn(len(keys))]
		pending = append(pending, Approval{
			Key:       key,
			Project:   randomApprovalText(rnd),
			ID:        randomApprovalText(rnd),
			Card:      randomApprovalText(rnd),
			Gate:      randomApprovalText(rnd),
			Artifacts: []string{randomApprovalText(rnd)},
		})
	}
	return pending
}

func randomReactions(rnd *rand.Rand) []Reaction {
	var reactions []Reaction
	for count := rnd.Intn(4); count > 0; count-- {
		reactions = append(reactions, randomReaction(rnd))
	}
	return reactions
}

func randomReaction(rnd *rand.Rand) Reaction {
	senders := []string{operator, "@stranger:example.org", ""}
	keys := []string{ApproveReaction, "🎉", "", "✅ "}
	targets := []string{approvalMessageID, "$other-message", ""}
	return Reaction{
		RoomID:        "!approvals:example.org",
		EventID:       randomApprovalText(rnd),
		Sender:        senders[rnd.Intn(len(senders))],
		TargetEventID: targets[rnd.Intn(len(targets))],
		Key:           keys[rnd.Intn(len(keys))],
	}
}

func randomApprovalReplies(rnd *rand.Rand) []RoomEvent {
	var replies []RoomEvent
	for count := rnd.Intn(4); count > 0; count-- {
		replies = append(replies, randomApprovalReply(rnd))
	}
	return replies
}

func randomApprovalReply(rnd *rand.Rand) RoomEvent {
	senders := []string{operator, "@stranger:example.org", ""}
	threads := []string{approvalMessageID, "$other-message", ""}
	return RoomEvent{
		RoomID:     "!approvals:example.org",
		EventID:    randomApprovalText(rnd),
		Sender:     senders[rnd.Intn(len(senders))],
		Body:       randomApprovalText(rnd),
		ThreadRoot: threads[rnd.Intn(len(threads))],
	}
}

func randomApprovalText(rnd *rand.Rand) string {
	texts := []string{"", " ", "send it back", "the timesheet total is still wrong", "$event-1"}
	return texts[rnd.Intn(len(texts))]
}
