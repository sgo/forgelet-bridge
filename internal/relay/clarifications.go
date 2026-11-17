package relay

const (
	// ClarificationSourceTyped marks a clarification from project chat. The
	// empty source is the legacy clarification store.
	ClarificationSourceTyped = "typed"
	// ClarificationOpen is a typed clarification awaiting an answer.
	ClarificationOpen = "open"
	// ClarificationSeen is an open typed clarification the operator has seen.
	ClarificationSeen = "seen"
)

// Clarification is one question a forge's agent is blocked on, as the forge's
// dashboard holds it.
type Clarification struct {
	Key      string
	ID       string
	Project  string
	Role     string
	Question string
	// Source is empty for the legacy store and "typed" for project chat.
	Source string
	// Lifecycle and Attention preserve the authoritative typed row fields.
	Lifecycle string
	Attention string
}

// ClarificationState is what the bridge remembers about one clarification.
type ClarificationState struct {
	ForgeRoot string `json:"forge_root,omitempty"`
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
	// Source keeps typed notification records distinct from legacy records.
	Source string `json:"source,omitempty"`
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

// PlanClarifications works out what the clarifications room needs: one message
// for every open, unseen typed clarification, the operator's answer carried
// through project chat, and one mark when that answer lands.
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
		unseen := clarification.Lifecycle == ClarificationOpen &&
			clarification.Attention == "unseen"
		if unseen && st.Clarifications[clarification.Key].MessageID == "" {
			actions = append(actions, ClarificationAction{Kind: PostClarification, Key: clarification.Key, Clarification: clarification})
		}
	}
	return actions
}

// answersToReport marks an answer the operator gave in Matrix after project
// chat accepted it. A desktop answer needs no Matrix work; its authoritative
// terminal row reconciles the record.
func answersToReport(st State, byKey map[string]Clarification) []ClarificationAction {
	var actions []ClarificationAction
	for _, clarification := range byKey {
		state := st.Clarifications[clarification.Key]
		if state.Answer == "" || state.ReactionID != "" || state.MessageID == "" {
			continue
		}
		actions = append(actions, ClarificationAction{
			Kind:      ReactClarification,
			Key:       clarification.Key,
			MessageID: state.MessageID,
			Reaction:  CarriedOutReaction,
		})
	}
	return actions
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-11-16T18:49:26+01:00","module_hash":"b5020d2b0194e5e54b92730922169028b6c44dc6786ea80c8d575b8ee8364d9d","functions":[{"id":"func/ClarificationState.Room","name":"ClarificationState.Room","line":58,"end_line":58,"hash":"062d0bbb5a7d64a29a3d799f8951afa787ae4f656a21a6e31efd8f0fca4418b5"},{"id":"func/PlanClarifications","name":"PlanClarifications","line":100,"end_line":106,"hash":"b1ceed3ae125074605a8c65b5f01cbe46f4135b8414deef80c2c871b7687d29a"},{"id":"func/clarificationsByEvent","name":"clarificationsByEvent","line":113,"end_line":127,"hash":"ffe20abed8ba7ca16812802ec8909a5555a4dce5c83fcfc92d191420ad117131"},{"id":"func/answeredByReply","name":"answeredByReply","line":131,"end_line":149,"hash":"c5f1d7fdeda1aa434b0c75bf7ba3aad249cfe80a188b949e1b56742b9b8f821a"},{"id":"func/awaitingAnswer","name":"awaitingAnswer","line":153,"end_line":156,"hash":"1ed2b04a8b4a62c056ae097d002b06fdf5523817f351f92e76bf30f41065e6d8"},{"id":"func/unpostedClarifications","name":"unpostedClarifications","line":160,"end_line":169,"hash":"2236765b1327a95072c95d9ddcc5a3eeadaa440eed2ef0682546d5da9baa8d0f"},{"id":"func/answersToReport","name":"answersToReport","line":175,"end_line":184,"hash":"770b8b859c7f72e0cf36ed3c160f8d70db99f70222948a33c16ee42cda8e8549"},{"id":"func/answerReport","name":"answerReport","line":190,"end_line":205,"hash":"1b0484c123d4a99d8f18304ac017f0d5c4e334e464ebea45c55dc2ebaa810228"},{"id":"func/unreportedAnswer","name":"unreportedAnswer","line":210,"end_line":217,"hash":"87383425ef86a292bd716b0c50b61fc6c4990095658737e13ff13fd7c7e14a68"},{"id":"func/ClarificationsReply","name":"ClarificationsReply","line":221,"end_line":226,"hash":"9ca510908a1c96a3b2a557bd24940ae3d02fa25a18182dd0dbe83a8cf73949d2"}]}
// mutate4go-manifest-end
