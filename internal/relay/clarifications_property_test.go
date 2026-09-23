//go:build property

package relay

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"testing/quick"
)

const clarificationMessageID = "$clarification-message"

const clarificationKey = "forgelet-bridge/clarification-1"

// TestPropertyPlanClarificationsAnswersOnce is the promise the phone's answer
// rests on: a clarification the bridge has already carried an answer back for
// is never answered a second time, however often the same replies are replayed,
// so a restart cannot ask the forge to answer one twice.
func TestPropertyPlanClarificationsAnswersOnce(t *testing.T) {
	property := func(st State, pending []Clarification, replies []RoomEvent) bool {
		st = cloneStateWithClarifications(st)
		pending = append([]Clarification(nil), pending...)

		actions := PlanClarifications(operator, st, pending, replies)
		if answersAlreadyAnswered(st, actions) {
			return false
		}
		pending = carryOutClarifications(&st, pending, actions)

		// Carrying the plan out leaves nothing to answer again, so a restart
		// that drains the same replies hears nothing.
		return !answersAlreadyAnswered(st, PlanClarifications(operator, st, pending, replies))
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 300,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randomClarificationState(rnd))
			values[1] = reflect.ValueOf(randomClarifications(rnd))
			values[2] = reflect.ValueOf(randomClarificationReplies(rnd))
		},
	}); err != nil {
		t.Error(err)
	}
}

// answersAlreadyAnswered reports whether a plan would carry an answer back for
// a clarification the state already holds an answer for.
func answersAlreadyAnswered(st State, actions []ClarificationAction) bool {
	for _, action := range actions {
		if action.Kind == AnswerClarification && st.Clarifications[action.Key].Answer != "" {
			return true
		}
	}
	return false
}

// TestPropertyPlanClarificationsNeedsTheOperatorsOwnReply is the rule the room
// depends on: only a reply of the operator's in the clarification's thread -
// written there, or made by quoting the clarification message - answers it. No
// other message, and no reply from anyone else, ever carries an answer back.
func TestPropertyPlanClarificationsNeedsTheOperatorsOwnReply(t *testing.T) {
	property := func(reply RoomEvent) bool {
		if reply.Sender == operator && repliedTo(reply) == clarificationMessageID {
			return true // this one is the answer, when it says anything
		}
		state := State{Clarifications: map[string]ClarificationState{
			clarificationKey: {MessageID: clarificationMessageID},
		}}
		pending := []Clarification{{Key: clarificationKey, Project: "forgelet-bridge", ID: "clarification-1"}}

		for _, action := range PlanClarifications(operator, state, pending, []RoomEvent{reply}) {
			if action.Kind == AnswerClarification {
				return false
			}
		}
		return true
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 300,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randomClarificationReply(rnd))
		},
	}); err != nil {
		t.Error(err)
	}
}

// TestPropertyOwnWordsKeepsOnlyWhatTheOperatorSaid is the parsing promise the
// answer rests on: whatever the phone quotes back into the body, and however
// many lines it covers, the answer is the words the operator wrote, never the
// question they answered.
func TestPropertyOwnWordsKeepsOnlyWhatTheOperatorSaid(t *testing.T) {
	property := func(quoteLines []string, words string) bool {
		return OwnWords(phoneQuote(quoteLines, words)) == strings.TrimSpace(words)
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 300,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randomQuoteLines(rnd))
			values[1] = reflect.ValueOf(randomText(rnd))
		},
	}); err != nil {
		t.Error(err)
	}
}

// phoneQuote writes a body the way the operator's phone does when it answers by
// quoting: every line of the quoted message marked, a blank line, then the
// words the operator wrote.
func phoneQuote(quoteLines []string, words string) string {
	var body strings.Builder
	for _, line := range quoteLines {
		body.WriteString("> ")
		body.WriteString(line)
		body.WriteString("\n")
	}
	body.WriteString("\n")
	body.WriteString(words)
	return body.String()
}

// carryOutClarifications records the effects a clarifications plan has on the
// bridge's state and on the forge, the way a tick does, and returns the
// clarifications the forge is still waiting for.
func carryOutClarifications(st *State, pending []Clarification, actions []ClarificationAction) []Clarification {
	st.EnsureMaps()
	for _, action := range actions {
		state := st.Clarifications[action.Key]
		switch action.Kind {
		case PostClarification:
			state.MessageID = "$posted-" + action.Key
		case AnswerClarification:
			state.Answer = action.Answer
			pending = removeClarification(pending, action.Key)
		case ReplyClarification:
			state.ReplyID = "$reply-" + action.Key
		}
		st.Clarifications[action.Key] = state
	}
	return pending
}

// removeClarification drops a clarification the forge no longer waits for.
func removeClarification(pending []Clarification, key string) []Clarification {
	kept := make([]Clarification, 0, len(pending))
	for _, clarification := range pending {
		if clarification.Key != key {
			kept = append(kept, clarification)
		}
	}
	return kept
}

// cloneStateWithClarifications is cloneState with the clarification
// bookkeeping copied too, so a property can carry a plan out without touching
// the state it was drawn from.
func cloneStateWithClarifications(st State) State {
	cloned := cloneState(st)
	if st.Clarifications != nil {
		cloned.Clarifications = make(map[string]ClarificationState, len(st.Clarifications))
		for key, state := range st.Clarifications {
			cloned.Clarifications[key] = state
		}
	}
	return cloned
}

func randomClarificationState(rnd *rand.Rand) State {
	st := randomState(rnd)
	st.Clarifications = map[string]ClarificationState{}
	keys := []string{"", clarificationKey, "forgelet-bridge/clarification-2"}
	for count := rnd.Intn(3); count > 0; count-- {
		st.Clarifications[keys[rnd.Intn(len(keys))]] = randomClarificationRecord(rnd)
	}
	return st
}

func randomClarificationRecord(rnd *rand.Rand) ClarificationState {
	messageIDs := []string{clarificationMessageID, "$other-message", ""}
	answers := []string{"", "yes", "", "the refund lane"}
	replyIDs := []string{"", "$reported", ""}
	return ClarificationState{
		MessageID: messageIDs[rnd.Intn(len(messageIDs))],
		Answer:    answers[rnd.Intn(len(answers))],
		ReplyID:   replyIDs[rnd.Intn(len(replyIDs))],
	}
}

func randomClarifications(rnd *rand.Rand) []Clarification {
	keys := []string{"", clarificationKey, "forgelet-bridge/clarification-2", "saibill/clarification-1"}
	var pending []Clarification
	for count := rnd.Intn(4); count > 0; count-- {
		pending = append(pending, Clarification{
			Key:      keys[rnd.Intn(len(keys))],
			ID:       randomClarificationText(rnd),
			Project:  randomClarificationText(rnd),
			Role:     randomClarificationText(rnd),
			Question: randomClarificationText(rnd),
		})
	}
	return pending
}

func randomClarificationReplies(rnd *rand.Rand) []RoomEvent {
	var replies []RoomEvent
	for count := rnd.Intn(4); count > 0; count-- {
		replies = append(replies, randomClarificationReply(rnd))
	}
	return replies
}

func randomClarificationReply(rnd *rand.Rand) RoomEvent {
	senders := []string{operator, "@stranger:example.org", ""}
	threads := []string{clarificationMessageID, "$other-message", ""}
	quoted := []string{clarificationMessageID, "$other-message", ""}
	return RoomEvent{
		RoomID:     "!clarifications:example.org",
		EventID:    randomClarificationText(rnd),
		Sender:     senders[rnd.Intn(len(senders))],
		Body:       randomClarificationText(rnd),
		ThreadRoot: threads[rnd.Intn(len(threads))],
		ReplyTo:    quoted[rnd.Intn(len(quoted))],
	}
}

func randomQuoteLines(rnd *rand.Rand) []string {
	lines := []string{"is the build green?", "Clarification for forgelet-bridge from coder", "Question: which lane?", ""}
	var quote []string
	for count := rnd.Intn(4); count > 0; count-- {
		quote = append(quote, lines[rnd.Intn(len(lines))])
	}
	return quote
}

func randomClarificationText(rnd *rand.Rand) string {
	texts := []string{"", " ", "yes, the refund lane", "the timesheet total is still wrong", "$event-1"}
	return texts[rnd.Intn(len(texts))]
}
