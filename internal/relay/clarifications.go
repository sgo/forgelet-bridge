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
	// Answer is the answer the bridge carried back, empty when the
	// clarification was answered somewhere else.
	Answer string `json:"answer,omitempty"`
	// ReplyID is the thread reply that reported the answer.
	ReplyID string `json:"reply_id,omitempty"`
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
}

// PlanClarifications works out what the clarifications room needs: a message
// for every clarification the forge is waiting for, the operator's answer
// carried back to the forge, and a reply in the thread once a clarification is
// answered however it was answered.
//
// A clarification's answer is free text, so a reply is what the room takes
// rather than a gesture: any reply of the operator's in the clarification's
// thread, written there or made by quoting the message, is the answer.
func PlanClarifications(operator string, st State, pending []Clarification, replies []RoomEvent) []ClarificationAction {
	byMessage, byKey := clarificationsByMessage(st, pending)

	actions := answeredByReply(operator, st, byMessage, replies)
	actions = append(actions, unpostedClarifications(st, pending)...)
	return append(actions, answersToReport(st, byKey)...)
}

// clarificationsByMessage indexes the clarifications two ways: by the message
// the room holds for them, and by their key, so a lookup can answer both "which
// clarification is this reply about" and "is this clarification still waiting".
func clarificationsByMessage(st State, pending []Clarification) (map[string]Clarification, map[string]Clarification) {
	byMessage := map[string]Clarification{}
	byKey := map[string]Clarification{}
	for _, clarification := range pending {
		byKey[clarification.Key] = clarification
		if messageID := st.Clarifications[clarification.Key].MessageID; messageID != "" {
			byMessage[messageID] = clarification
		}
	}
	return byMessage, byKey
}

// answeredByReply plans the answers the operator gave in a clarification's
// thread.
func answeredByReply(operator string, st State, byMessage map[string]Clarification, replies []RoomEvent) []ClarificationAction {
	var actions []ClarificationAction
	for _, reply := range replies {
		if reply.Sender != operator {
			continue
		}
		clarification, known := byMessage[repliedTo(reply)]
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

// answersToReport plans the replies for the clarifications that are no longer
// waiting - answered on the desktop, or by the answer just carried back - and
// have not been reported in their thread yet.
func answersToReport(st State, byKey map[string]Clarification) []ClarificationAction {
	var actions []ClarificationAction
	for key, state := range st.Clarifications {
		if !unreportedAnswer(state, key, byKey) {
			continue
		}
		actions = append(actions, ClarificationAction{
			Kind:      ReplyClarification,
			Key:       key,
			MessageID: state.MessageID,
			Text:      ClarificationsReply(state.Answer),
		})
	}
	return actions
}

// unreportedAnswer reports whether a clarification the room has seen still
// needs its answer reported in its thread: it is no longer one the forge waits
// for, and the thread has not been told yet.
func unreportedAnswer(state ClarificationState, key string, byKey map[string]Clarification) bool {
	if state.MessageID == "" || state.ReplyID != "" {
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
// {"version":1,"tested_at":"2026-09-23T14:24:39+02:00","module_hash":"e40b703750e3ba89f99f14443b64989bdd032b99a8c1f5be2227a4151e693777","functions":[{"id":"func/ClarificationState.Room","name":"ClarificationState.Room","line":28,"end_line":28,"hash":"062d0bbb5a7d64a29a3d799f8951afa787ae4f656a21a6e31efd8f0fca4418b5"},{"id":"func/PlanClarifications","name":"PlanClarifications","line":63,"end_line":69,"hash":"154d3af9ddd0c4ca0281557f6b424d2d197132ecaa382d4cc6dd0f6c49cd1b70"},{"id":"func/clarificationsByMessage","name":"clarificationsByMessage","line":74,"end_line":84,"hash":"d5213dfbc6a5b7116b37e8bc457adf1984d939499cc62e7809925348eb0f1ccf"},{"id":"func/answeredByReply","name":"answeredByReply","line":88,"end_line":106,"hash":"ee874cc21fed201a675406db9e838b382e37f8b159713860c9eb87c5a55f5d96"},{"id":"func/awaitingAnswer","name":"awaitingAnswer","line":110,"end_line":113,"hash":"1ed2b04a8b4a62c056ae097d002b06fdf5523817f351f92e76bf30f41065e6d8"},{"id":"func/unpostedClarifications","name":"unpostedClarifications","line":117,"end_line":125,"hash":"fb2a04eaf2c8d3f99cb578c85ec482b605a51f25f8256ef7277b069915e87023"},{"id":"func/answersToReport","name":"answersToReport","line":130,"end_line":144,"hash":"9cd7ad804ac7aabf019121e465df2bb1c597b2b9e14c9054bb0e22d28b934e71"},{"id":"func/unreportedAnswer","name":"unreportedAnswer","line":149,"end_line":155,"hash":"adb8c8996ae5d866b8226f43d49fd1a81eba8391f3754c7eeb5aa752bc2acf02"},{"id":"func/ClarificationsReply","name":"ClarificationsReply","line":159,"end_line":164,"hash":"9ca510908a1c96a3b2a557bd24940ae3d02fa25a18182dd0dbe83a8cf73949d2"}]}
// mutate4go-manifest-end
