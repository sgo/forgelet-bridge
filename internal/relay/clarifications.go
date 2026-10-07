package relay

// Clarification is one question a forge's agent is blocked on, as the forge's
// dashboard holds it.
type Clarification struct {
	Key      string
	ID       string
	Project  string
	Role     string
	Question string
}

// ClarificationState is what the bridge remembers about one clarification.
type ClarificationState struct {
	// RoomID is the room the clarification's message was posted in, so a room
	// reports on its own clarifications and no other forge's.
	RoomID string `json:"room_id,omitempty"`
	// MessageID is the chat message that carries the clarification.
	MessageID string `json:"message_id,omitempty"`
	// StateEventID is the room's state event that carries the question - the
	// project, the blocked role and the question itself. It is the only thing a
	// phone can read, because a phone reads the room's state, so the id is
	// remembered when the event is written: a reply that names it answers this
	// clarification, the same way one that names the message does.
	StateEventID string `json:"state_event_id,omitempty"`
	// Answer is the answer the bridge carried back, empty when the
	// clarification was answered somewhere else.
	Answer string `json:"answer,omitempty"`
	// ReplyID is the thread reply that reported the answer.
	ReplyID string `json:"reply_id,omitempty"`
	// ReactionID is the bridge's own reaction on the clarification's message,
	// when the answer was reported that way rather than in the thread.
	ReactionID string `json:"reaction_id,omitempty"`
}

// Room is the room this clarification's message is in.
func (s ClarificationState) Room() string { return s.RoomID }

// ClarificationKind names the work a clarification action asks for.
type ClarificationKind string

const (
	// PostClarification posts a pending clarification into the room.
	PostClarification ClarificationKind = "post_clarification"
	// AnswerClarification carries the operator's answer back to the forge,
	// which is what wakes the blocked role.
	AnswerClarification ClarificationKind = "answer_clarification"
	// ReplyClarification reports in the clarification's thread how it was
	// answered.
	ReplyClarification ClarificationKind = "reply_clarification"
	// ReactClarification marks the clarification's own message with the
	// bridge's reaction, so an answer the operator gave here carries no thread
	// reply.
	ReactClarification ClarificationKind = "react_clarification"
)

// ClarificationAction is one piece of clarifications work for the bridge to
// carry out.
type ClarificationAction struct {
	Kind          ClarificationKind
	Key           string
	Clarification Clarification
	Answer        string
	MessageID     string
	Text          string
	Reaction      string
}

// PlanClarifications works out what the clarifications room needs: a message
// for every clarification the forge is waiting for, the operator's answer
// carried back to the forge, and the answer carried to the operator once a
// clarification is answered - marked on the clarification's own message when
// the operator answered here, and replied to in its thread when the answer came
// from the desktop.
//
// A clarification's answer is free text, so a reply is what the room takes
// rather than a gesture: any reply of the operator's in the clarification's
// thread, written there or made by quoting the message, is the answer.
func PlanClarifications(operator string, st State, pending []Clarification, replies []RoomEvent) []ClarificationAction {
	byEvent, byKey := clarificationsByEvent(st, pending)

	actions := answeredByReply(operator, st, byEvent, replies)
	actions = append(actions, unpostedClarifications(st, pending)...)
	return append(actions, answersToReport(st, byKey)...)
}

// clarificationsByEvent indexes the clarifications two ways: by every event the
// room holds for them - the message the operator reads, and the state event
// that carries the question - and by their key, so a lookup can answer both
// "which clarification is this reply about" and "is this clarification still
// waiting". A reply names one of the two, and either names the question.
func clarificationsByEvent(st State, pending []Clarification) (map[string]Clarification, map[string]Clarification) {
	byEvent := map[string]Clarification{}
	byKey := map[string]Clarification{}
	for _, clarification := range pending {
		byKey[clarification.Key] = clarification
		state := st.Clarifications[clarification.Key]
		if state.MessageID != "" {
			byEvent[state.MessageID] = clarification
		}
		if state.StateEventID != "" {
			byEvent[state.StateEventID] = clarification
		}
	}
	return byEvent, byKey
}

// answeredByReply plans the answers the operator gave in a clarification's
// thread.
func answeredByReply(operator string, st State, byEvent map[string]Clarification, replies []RoomEvent) []ClarificationAction {
	var actions []ClarificationAction
	for _, reply := range replies {
		if reply.Sender != operator {
			continue
		}
		clarification, known := byEvent[repliedTo(reply)]
		if !known || !awaitingAnswer(st, clarification.Key) {
			continue
		}
		actions = append(actions, ClarificationAction{
			Kind:          AnswerClarification,
			Key:           clarification.Key,
			Clarification: clarification,
			Answer:        OwnWords(reply.Body),
		})
	}
	return actions
}

// awaitingAnswer reports whether the room has the clarification and neither
// device has answered it yet.
func awaitingAnswer(st State, key string) bool {
	state, known := st.Clarifications[key]
	return known && state.MessageID != "" && state.Answer == ""
}

// unpostedClarifications plans the messages for the clarifications the room
// has not seen yet.
func unpostedClarifications(st State, pending []Clarification) []ClarificationAction {
	var actions []ClarificationAction
	for _, clarification := range pending {
		if st.Clarifications[clarification.Key].MessageID == "" {
			actions = append(actions, ClarificationAction{Kind: PostClarification, Key: clarification.Key, Clarification: clarification})
		}
	}
	return actions
}

// answersToReport plans the reports for the clarifications that are no longer
// waiting - answered on the desktop, or by the answer just carried back - and
// have not been reported yet: an answer the operator gave here is marked on the
// clarification's own message, and a desktop answer keeps its threaded reply.
func answersToReport(st State, byKey map[string]Clarification) []ClarificationAction {
	var actions []ClarificationAction
	for key, state := range st.Clarifications {
		if !unreportedAnswer(state, key, byKey) {
			continue
		}
		actions = append(actions, answerReport(key, state))
	}
	return actions
}

// answerReport is how one answer reaches the operator: an answer the operator
// gave here is marked with the bridge's reaction on the clarification's own
// message, and a desktop answer is replied to in the thread, because it is a
// word the room never heard.
func answerReport(key string, state ClarificationState) ClarificationAction {
	if state.Answer != "" {
		return ClarificationAction{
			Kind:      ReactClarification,
			Key:       key,
			MessageID: state.MessageID,
			Reaction:  CarriedOutReaction,
		}
	}
	return ClarificationAction{
		Kind:      ReplyClarification,
		Key:       key,
		MessageID: state.MessageID,
		Text:      ClarificationsReply(state.Answer),
	}
}

// unreportedAnswer reports whether a clarification the room has seen still
// needs its answer reported: it is no longer one the forge waits for, and the
// room has not been told yet, by a thread reply or a reaction.
func unreportedAnswer(state ClarificationState, key string, byKey map[string]Clarification) bool {
	if state.MessageID == "" || state.ReplyID != "" || state.ReactionID != "" {
		return false
	}
	_, stillWaiting := byKey[key]
	return !stillWaiting
}

// ClarificationsReply is the text the bridge reports an answer with: the answer
// that was carried back, or the desktop's when it was answered there.
func ClarificationsReply(answer string) string {
	if answer != "" {
		return "Answered"
	}
	return "Resolved on the desktop"
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-10-07T01:14:46+02:00","module_hash":"30f0a4f181f91047923f40b1d2ca494c082ad49ae2ecdf281a36b20dc5ce1a59","functions":[{"id":"func/ClarificationState.Room","name":"ClarificationState.Room","line":37,"end_line":37,"hash":"062d0bbb5a7d64a29a3d799f8951afa787ae4f656a21a6e31efd8f0fca4418b5"},{"id":"func/PlanClarifications","name":"PlanClarifications","line":79,"end_line":85,"hash":"b1ceed3ae125074605a8c65b5f01cbe46f4135b8414deef80c2c871b7687d29a"},{"id":"func/clarificationsByEvent","name":"clarificationsByEvent","line":92,"end_line":106,"hash":"ffe20abed8ba7ca16812802ec8909a5555a4dce5c83fcfc92d191420ad117131"},{"id":"func/answeredByReply","name":"answeredByReply","line":110,"end_line":128,"hash":"c5f1d7fdeda1aa434b0c75bf7ba3aad249cfe80a188b949e1b56742b9b8f821a"},{"id":"func/awaitingAnswer","name":"awaitingAnswer","line":132,"end_line":135,"hash":"1ed2b04a8b4a62c056ae097d002b06fdf5523817f351f92e76bf30f41065e6d8"},{"id":"func/unpostedClarifications","name":"unpostedClarifications","line":139,"end_line":147,"hash":"fb2a04eaf2c8d3f99cb578c85ec482b605a51f25f8256ef7277b069915e87023"},{"id":"func/answersToReport","name":"answersToReport","line":153,"end_line":162,"hash":"770b8b859c7f72e0cf36ed3c160f8d70db99f70222948a33c16ee42cda8e8549"},{"id":"func/answerReport","name":"answerReport","line":168,"end_line":183,"hash":"1b0484c123d4a99d8f18304ac017f0d5c4e334e464ebea45c55dc2ebaa810228"},{"id":"func/unreportedAnswer","name":"unreportedAnswer","line":188,"end_line":194,"hash":"6341e3b4c9efaa68d9c1d9a8016582d26ce84d5a8eeebab404bfa7661da5e32d"},{"id":"func/ClarificationsReply","name":"ClarificationsReply","line":198,"end_line":203,"hash":"9ca510908a1c96a3b2a557bd24940ae3d02fa25a18182dd0dbe83a8cf73949d2"}]}
// mutate4go-manifest-end
