//go:build property

package relay

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"testing/quick"
)

// TestPropertyOnlyAnAffirmativeReplyApproves is the rule the destructive half
// depends on: a reply in an approval's thread either approves, when its whole
// text is an affirmative word or the card's name, or it is the send-back it
// always was - carrying the reply as its feedback - and no other text ever
// approves.
func TestPropertyOnlyAnAffirmativeReplyApproves(t *testing.T) {
	property := func(body string) bool {
		if strings.TrimSpace(body) == "" {
			return true // an empty message decides nothing, which the planner skips
		}
		st := posted()
		approval := approval()
		reply := RoomEvent{
			RoomID:     "!approvals:example.org",
			EventID:    "$reply",
			Sender:     operator,
			Body:       body,
			ThreadRoot: approvalMessageID,
		}

		actions := PlanApprovals(operator, st, []Approval{approval}, nil, []RoomEvent{reply})
		if len(actions) != 1 || actions[0].Kind != ResolveApproval {
			return false
		}
		if affirmative(body, approval) {
			return actions[0].Resolution == ResolutionApproved && actions[0].Feedback == ""
		}
		return actions[0].Resolution == ResolutionSentBack && actions[0].Feedback == body
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 300,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randomMessageText(rnd))
		},
	}); err != nil {
		t.Error(err)
	}
}

// TestPropertyAnUnreadableMessageIsAnswered checks the other half of the room's
// grammar: a message from the operator that approves nothing, names no waiting
// card, and replies under no known approval is answered with the gestures the
// room takes, so a working relay and a dead one cannot look the same.
func TestPropertyAnUnreadableMessageIsAnswered(t *testing.T) {
	property := func(body string) bool {
		if strings.TrimSpace(body) == "" {
			return true
		}
		message := RoomEvent{
			RoomID:  "!approvals:example.org",
			EventID: "$message",
			Sender:  operator,
			Body:    body,
		}
		if affirmative(body, approval()) || strings.EqualFold(strings.TrimSpace(body), approval().Card) {
			return true // this one is a gesture the room reads, above
		}

		actions := PlanApprovals(operator, posted(), []Approval{approval()}, nil, []RoomEvent{message})
		return len(actions) == 1 && actions[0].Kind == AnswerGestures
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 300,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randomMessageText(rnd))
		},
	}); err != nil {
		t.Error(err)
	}
}

// TestPropertyAStrangersGestureStaysSilent checks the allowlist for text as
// well as reactions: neither a message nor a reaction from anyone but the
// operator decides anything, and the room does not answer them either.
func TestPropertyAStrangersGestureStaysSilent(t *testing.T) {
	property := func(body string) bool {
		message := RoomEvent{
			RoomID:  "!approvals:example.org",
			EventID: "$message",
			Sender:  "@stranger:example.org",
			Body:    body,
		}
		reaction := Reaction{
			RoomID:        "!approvals:example.org",
			EventID:       "$reaction",
			Sender:        "@stranger:example.org",
			TargetEventID: approvalMessageID,
			Key:           "🎉",
		}

		for range PlanApprovals(operator, posted(), []Approval{approval()}, []Reaction{reaction}, []RoomEvent{message}) {
			return false
		}
		return true
	}
	if err := quick.Check(property, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// randomMessageText is what a phone sends: the words the room takes, and the
// ones it does not.
func randomMessageText(rnd *rand.Rand) string {
	texts := []string{
		"",
		" ",
		"approve",
		"Approve",
		" approved ",
		"go ahead",
		"yes",
		"lgtm",
		"okay",
		"phone-approvals",
		"Phone-Approvals",
		"the timesheet total is still wrong",
		"approve it, but the total is wrong",
		"🎉",
	}
	return texts[rnd.Intn(len(texts))]
}
