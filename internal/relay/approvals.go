package relay

import "strings"

// Approval is one handoff waiting for the operator's decision, as the forge's
// projects hold it.
type Approval struct {
	Key       string
	Project   string
	ID        string
	Card      string
	Gate      string
	Artifacts []string
}

// ApprovalState is what the bridge remembers about one approval.
type ApprovalState struct {
	// RoomID is the room the approval's message was posted in, so a room
	// reports on its own approvals and no other forge's.
	RoomID string `json:"room_id,omitempty"`
	// MessageID is the chat message that carries the approval.
	MessageID string `json:"message_id,omitempty"`
	// Resolution is how the approval was resolved: approved, sent_back, or
	// desktop.
	Resolution string `json:"resolution,omitempty"`
	// ReplyID is the thread reply that reported the resolution.
	ReplyID string `json:"reply_id,omitempty"`
	// ReactionID is the bridge's own reaction on the approval's message, when
	// the resolution was reported that way rather than in the thread.
	ReactionID string `json:"reaction_id,omitempty"`
}

// Room is the room this approval's message is in.
func (s ApprovalState) Room() string { return s.RoomID }

// Resolutions an approval can end up with.
const (
	ResolutionApproved = "approved"
	ResolutionSentBack = "sent_back"
	ResolutionDesktop  = "desktop"
)

// Reaction is one reaction the bridge has seen, with the event it annotates.
type Reaction struct {
	RoomID        string
	EventID       string
	Sender        string
	TargetEventID string
	Key           string
}

// ApproveReaction is the reaction the operator approves with.
const ApproveReaction = "✅"

// CarriedOutReaction is the reaction the bridge marks an approval with once
// the forge confirmed it: it sits on the approval's own message, beside the
// operator's check mark, so the thread stays quiet.
const CarriedOutReaction = "➡"

// SentBackReaction is the reaction the bridge marks a send-back with, so the
// work going back is answered on the approval's own message the way an
// approval is.
const SentBackReaction = "⬅"

// approveReactions are the check marks that mean approval. Reaction pickers
// disagree about the variation selector, and ✔️ and ☑️ are what a thumb reaches
// when it means ✅; the sender check below is what keeps approval narrow, not
// the glyph. A reaction the operator did not send never approves.
var approveReactions = map[string]bool{
	"\u2705":       true, // ✅
	"\u2705\ufe0f": true, // ✅ with the variation selector
	"\u2714":       true, // ✔
	"\u2714\ufe0f": true, // ✔️
	"\u2611":       true, // ☑
	"\u2611\ufe0f": true, // ☑️
}

// Approves reports whether a reaction key is one the operator approves with.
func Approves(key string) bool { return approveReactions[key] }

// ApprovalKind names the work an approval action asks for.
type ApprovalKind string

const (
	// PostApproval posts a pending approval into the approvals room.
	PostApproval ApprovalKind = "post_approval"
	// ResolveApproval acts on the approval in the forge.
	ResolveApproval ApprovalKind = "resolve_approval"
	// ReplyApproval reports in the approval's thread how it was resolved.
	ReplyApproval ApprovalKind = "reply_approval"
	// ReactApproval marks the approval's own message with the bridge's
	// reaction - the carried-out mark for an approval, the send-back's for a
	// send-back - so an outcome carried by a mark leaves no thread reply.
	ReactApproval ApprovalKind = "react_approval"
	// AnswerGestures tells the operator which gestures the room takes, when a
	// message can be read as none of them.
	AnswerGestures ApprovalKind = "answer_gestures"
)

// ApprovalAction is one piece of approvals work for the bridge to carry out.
type ApprovalAction struct {
	Kind       ApprovalKind
	Key        string
	Approval   Approval
	Resolution string
	Feedback   string
	MessageID  string
	Text       string
	Reaction   string
}

// PlanApprovals works out what the approvals room needs from the room's own
// events alone: see PlanApprovalsWithDesk for the approvals a desk resolved.
func PlanApprovals(operator string, st State, pending []Approval, reactions []Reaction, replies []RoomEvent) []ApprovalAction {
	return PlanApprovalsWithDesk(operator, st, pending, nil, reactions, replies)
}

// PlanApprovalsWithDesk is PlanApprovals with what the desk wrote down about
// the approvals it resolved away from the room, keyed by approval: the ending
// the room reports for an approval the bridge did not carry out itself.
//
// It works out what the approvals room needs: a message for every approval the
// forge is waiting for, the operator's decision carried back to the forge, and
// the outcome carried to the operator once an approval is resolved - marked on
// the approval's own message when it was approved or sent back, and replied to
// in its thread when the resolution carries words. An approval's outcome is the
// bridge's own when it made it, the desk's record when the desk made it, and
// nothing when neither wrote one down. Only the operator's own approval
// reaction, or a reply of theirs in the approval's thread, decides anything.
func PlanApprovalsWithDesk(operator string, st State, pending []Approval, desk map[string]string, reactions []Reaction, replies []RoomEvent) []ApprovalAction {
	byMessage, byKey := approvalsByMessage(st, pending)

	actions := approvedByReaction(operator, st, byMessage, reactions)
	actions = append(actions, textGestures(operator, st, pending, byMessage, replies)...)
	actions = append(actions, unansweredReactions(operator, reactions)...)
	actions = append(actions, unpostedApprovals(st, pending)...)
	return append(actions, resolutionsToReport(st, byKey, desk)...)
}

// approvalsByMessage indexes the approvals two ways: by the message the room
// holds for them, and by their key, so a lookup can answer both "which
// approval is this event about" and "is this approval still pending".
func approvalsByMessage(st State, pending []Approval) (map[string]Approval, map[string]Approval) {
	byMessage := map[string]Approval{}
	byKey := map[string]Approval{}
	for _, approval := range pending {
		byKey[approval.Key] = approval
		if messageID := st.Approvals[approval.Key].MessageID; messageID != "" {
			byMessage[messageID] = approval
		}
	}
	return byMessage, byKey
}

// approvedByReaction plans the approvals the operator approved by reacting.
func approvedByReaction(operator string, st State, byMessage map[string]Approval, reactions []Reaction) []ApprovalAction {
	var actions []ApprovalAction
	for _, reaction := range reactions {
		if reaction.Sender != operator || !Approves(reaction.Key) {
			continue
		}
		approval, known := byMessage[reaction.TargetEventID]
		if !known || !undecided(st, approval.Key) {
			continue
		}
		actions = append(actions, ApprovalAction{
			Kind:       ResolveApproval,
			Key:        approval.Key,
			Approval:   approval,
			Resolution: ResolutionApproved,
		})
	}
	return actions
}

// textGestures plans what the operator's text does in the approvals room. A
// reply that only approves approves, the way the check mark does; a reply that
// says more stays the send-back it always was, feedback and all; a message in
// the room that only approves - the word, or the card's name - approves too;
// and anything the room can read as none of those is answered with the gestures
// it does take, so a dead room and a working one cannot look the same.
func textGestures(operator string, st State, pending []Approval, byMessage map[string]Approval, messages []RoomEvent) []ApprovalAction {
	var actions []ApprovalAction
	for _, message := range messages {
		if message.Sender != operator || OwnWords(message.Body) == "" {
			continue
		}
		if approval, replied := repliedApproval(byMessage, message); replied {
			if !undecided(st, approval.Key) {
				continue // already decided: a later reply in its thread says nothing
			}
			actions = append(actions, decisionFor(approval, OwnWords(message.Body)))
			continue
		}
		if key, approval, matched := approvalForText(pending, OwnWords(message.Body)); matched && undecided(st, key) {
			actions = append(actions, ApprovalAction{
				Kind: ResolveApproval, Key: key, Approval: approval, Resolution: ResolutionApproved,
			})
			continue
		}
		actions = append(actions, ApprovalAction{Kind: AnswerGestures})
	}
	return actions
}

// repliedApproval is the approval a message replies under, when the room knows
// that approval's message. A reply is written in the approval's thread, or made
// by quoting the approval message, which is the reply a phone sends.
func repliedApproval(byMessage map[string]Approval, message RoomEvent) (Approval, bool) {
	if repliedTo(message) == "" {
		return Approval{}, false
	}
	approval, known := byMessage[repliedTo(message)]
	return approval, known
}

// decisionFor is what a reply in an approval's thread decides: the plain word
// approves, and anything else is the send-back it always was, with the reply as
// its feedback.
func decisionFor(approval Approval, body string) ApprovalAction {
	if affirmative(body, approval) {
		return ApprovalAction{
			Kind: ResolveApproval, Key: approval.Key, Approval: approval, Resolution: ResolutionApproved,
		}
	}
	return ApprovalAction{
		Kind: ResolveApproval, Key: approval.Key, Approval: approval, Resolution: ResolutionSentBack, Feedback: body,
	}
}

// unansweredReactions plans the answer for a reaction the room cannot read: the
// operator's own reaction that is not a check mark decides nothing, and says as
// much. Reactions from anyone else stay silent.
func unansweredReactions(operator string, reactions []Reaction) []ApprovalAction {
	var actions []ApprovalAction
	for _, reaction := range reactions {
		if reaction.Sender == operator && !Approves(reaction.Key) {
			actions = append(actions, ApprovalAction{Kind: AnswerGestures})
		}
	}
	return actions
}

// affirmativeWords are the plain answers a phone keyboard sends.
var affirmativeWords = []string{"approve", "approved", "go ahead", "yes", "ok", "okay", "lgtm"}

// affirmative reports whether a message's whole text approves the approval.
func affirmative(text string, approval Approval) bool {
	trimmed := strings.TrimSpace(strings.ToLower(text))
	if trimmed == "" {
		return false
	}
	if strings.ToLower(strings.TrimSpace(approval.Card)) == trimmed {
		return true
	}
	for _, word := range affirmativeWords {
		if trimmed == word {
			return true
		}
	}
	return false
}

// approvalForText finds the approval a plain message approves: the one whose
// card it names, or the only one waiting when the text is just an affirmative.
func approvalForText(pending []Approval, text string) (string, Approval, bool) {
	trimmed := strings.TrimSpace(strings.ToLower(text))
	for _, approval := range pending {
		if strings.ToLower(strings.TrimSpace(approval.Card)) == trimmed {
			return approval.Key, approval, true
		}
	}
	if len(pending) == 1 && affirmative(text, pending[0]) {
		return pending[0].Key, pending[0], true
	}
	return "", Approval{}, false
}

// unpostedApprovals plans the messages for the approvals the room has not seen
// yet.
func unpostedApprovals(st State, pending []Approval) []ApprovalAction {
	var actions []ApprovalAction
	for _, approval := range pending {
		if st.Approvals[approval.Key].MessageID == "" {
			actions = append(actions, ApprovalAction{Kind: PostApproval, Key: approval.Key, Approval: approval})
		}
	}
	return actions
}

// resolutionsToReport plans the reports for the approvals that are no longer
// pending - resolved by the decision just carried back, or away from the room -
// and have not been reported yet: an approval or a send-back is marked on the
// approval's own message, and a resolution with nothing but the desk's word for
// it, or nothing at all, keeps its threaded reply.
func resolutionsToReport(st State, byKey map[string]Approval, desk map[string]string) []ApprovalAction {
	var actions []ApprovalAction
	for key, state := range st.Approvals {
		if !unreportedResolution(state, key, byKey) {
			continue
		}
		actions = append(actions, resolutionReport(key, state, desk[key]))
	}
	return actions
}

// resolutionReport is how one resolution reaches the operator: an approval and
// a send-back are marked with the bridge's reaction on the approval's own
// message, and a resolution the desk wrote nothing down for is replied to in
// the thread, because "resolved on the desktop" is all the room can say.
//
// The ending is the bridge's own resolution when it made it, and the desk's
// record when the desk made it; the bridge's own word is the one that counts.
func resolutionReport(key string, state ApprovalState, deskResolution string) ApprovalAction {
	resolution := state.Resolution
	if resolution == "" {
		resolution = deskResolution
	}
	if resolution == "" {
		resolution = ResolutionDesktop
	}
	if mark, marked := resolutionMark(resolution); marked {
		return ApprovalAction{
			Kind:       ReactApproval,
			Key:        key,
			MessageID:  state.MessageID,
			Resolution: resolution,
			Reaction:   mark,
		}
	}
	return ApprovalAction{
		Kind:       ReplyApproval,
		Key:        key,
		MessageID:  state.MessageID,
		Resolution: resolution,
		Text:       ApprovalsReply(resolution),
	}
}

// resolutionMark is the mark the bridge answers a resolution with when the
// room learns it by reaction: the carried-out mark for an approval, and the
// send-back's mark for a send-back. A desktop resolution has none, and keeps
// its threaded reply.
func resolutionMark(resolution string) (string, bool) {
	switch resolution {
	case ResolutionApproved:
		return CarriedOutReaction, true
	case ResolutionSentBack:
		return SentBackReaction, true
	default:
		return "", false
	}
}

// unreportedResolution reports whether an approval the room has seen still
// needs its resolution reported: it is no longer one the forge waits for, and
// the room has not been told yet, by a thread reply or a reaction.
func unreportedResolution(state ApprovalState, key string, byKey map[string]Approval) bool {
	if state.MessageID == "" || state.ReplyID != "" || state.ReactionID != "" {
		return false
	}
	_, stillPending := byKey[key]
	return !stillPending
}

// ApprovalsReply is the text the bridge reports a resolution with.
func ApprovalsReply(resolution string) string {
	switch resolution {
	case ResolutionApproved:
		return "Approved"
	case ResolutionSentBack:
		return "Sent back with feedback"
	default:
		return "Resolved on the desktop"
	}
}

func undecided(st State, key string) bool {
	state, known := st.Approvals[key]
	return known && state.MessageID != "" && state.Resolution == ""
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-10-03T13:55:00+02:00","module_hash":"4f244746c4cdec67223f11b25498f7845bf014a7b2dd54648d703dcc8a96a0eb","functions":[{"id":"func/ApprovalState.Room","name":"ApprovalState.Room","line":34,"end_line":34,"hash":"78a56a4cd17adf780b13f46acd4bd490ab233cfa0411043f39688e34a039aa50"},{"id":"func/Approves","name":"Approves","line":79,"end_line":79,"hash":"e9a9c788d3529be59309cc6af6bd3b3f8f19d4fa9790fdfc3bc6e597ef64808b"},{"id":"func/PlanApprovals","name":"PlanApprovals","line":114,"end_line":116,"hash":"9b10b4f7fd7920f4db55d169e3053707ef4d14f61cb837b4c86f814a6600759f"},{"id":"func/PlanApprovalsWithDesk","name":"PlanApprovalsWithDesk","line":130,"end_line":138,"hash":"2a4989991330379319b678c2f8f886c9e930ac9a4c5f913cf5f39bc5efd9e222"},{"id":"func/approvalsByMessage","name":"approvalsByMessage","line":143,"end_line":153,"hash":"9c9ccaf39cd05bfdc12bf7d5fafc53e12e872bcbfa9c5bc3ed9ac27f130c4f4c"},{"id":"func/approvedByReaction","name":"approvedByReaction","line":156,"end_line":174,"hash":"b8cb6ef510812b4474786f8b100bc1203aa11871f8f080afcce50cf4f196590d"},{"id":"func/textGestures","name":"textGestures","line":182,"end_line":204,"hash":"4b41513290296b531194d46e75eefef35451706c413a6d2db058048154a1dcd3"},{"id":"func/repliedApproval","name":"repliedApproval","line":209,"end_line":215,"hash":"83c75ab8060928303178a10d9e7cab3d8d4c430dde8424f5271482835f0ac631"},{"id":"func/decisionFor","name":"decisionFor","line":220,"end_line":229,"hash":"d46710175488fee0b70d93ac9583982c6608a5ad479af476c441272e7ba5257e"},{"id":"func/unansweredReactions","name":"unansweredReactions","line":234,"end_line":242,"hash":"24342a0a528b9e7488c3675a19abdf72e7bedcc68b515d86de74dff803330b3c"},{"id":"func/affirmative","name":"affirmative","line":248,"end_line":262,"hash":"09747205691ab6a2d88969c216d979cf77bc1626a2af768c42ab8a93361e22be"},{"id":"func/approvalForText","name":"approvalForText","line":266,"end_line":277,"hash":"4ec4faadd738cba840808b156a1e9c8abb9a94ad5f0ebe9da68542e9f02ce2ca"},{"id":"func/unpostedApprovals","name":"unpostedApprovals","line":281,"end_line":289,"hash":"98ab83a435b4107b8e8437a5cd53e76ed1d11e36779b39bf9eb6505d9a4acd8f"},{"id":"func/resolutionsToReport","name":"resolutionsToReport","line":296,"end_line":305,"hash":"bb81910305926815dc55c1baeb3650e8f8a8bcfa9653523c4352e276fc824956"},{"id":"func/resolutionReport","name":"resolutionReport","line":314,"end_line":338,"hash":"2bfeac6637897fcae596df539e264f6e1e39ea7c1adbd7b99bd6844a8fd1a56c"},{"id":"func/resolutionMark","name":"resolutionMark","line":344,"end_line":353,"hash":"74583fffdb2d28a67096e278c4d86d1e75c0f8c9e0cace181d5ce51052a9ce38"},{"id":"func/unreportedResolution","name":"unreportedResolution","line":358,"end_line":364,"hash":"aedd2ffedfeca6212408439db795c4145a35afb3bf887d288bddd6998f4846ee"},{"id":"func/ApprovalsReply","name":"ApprovalsReply","line":367,"end_line":376,"hash":"b8a277047b4dd09a8e936509d5d2d5a22fe49d0ce688f000755bf029b619a6f5"},{"id":"func/undecided","name":"undecided","line":378,"end_line":381,"hash":"111471b0a984e4e15086380f5923369b9b7103ddc8290f1a7327544dcfd76fd0"}]}
// mutate4go-manifest-end
