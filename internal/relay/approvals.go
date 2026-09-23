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
}

// PlanApprovals works out what the approvals room needs: a message for every
// approval the forge is waiting for, the operator's decision carried back to
// the forge, and a reply in the thread once an approval is resolved however it
// was resolved. Only the operator's own approval reaction, or a reply of theirs
// in the approval's thread, decides anything.
func PlanApprovals(operator string, st State, pending []Approval, reactions []Reaction, replies []RoomEvent) []ApprovalAction {
	byMessage, byKey := approvalsByMessage(st, pending)

	actions := approvedByReaction(operator, st, byMessage, reactions)
	actions = append(actions, textGestures(operator, st, pending, byMessage, replies)...)
	actions = append(actions, unansweredReactions(operator, reactions)...)
	actions = append(actions, unpostedApprovals(st, pending)...)
	return append(actions, resolutionsToReport(st, byKey)...)
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

// resolutionsToReport plans the replies for the approvals that are no longer
// pending - resolved on the desktop, or by the decision just carried back -
// and have not been reported in their thread yet.
func resolutionsToReport(st State, byKey map[string]Approval) []ApprovalAction {
	var actions []ApprovalAction
	for key, state := range st.Approvals {
		if !unreportedResolution(state, key, byKey) {
			continue
		}
		resolution := state.Resolution
		if resolution == "" {
			resolution = ResolutionDesktop
		}
		actions = append(actions, ApprovalAction{
			Kind:       ReplyApproval,
			Key:        key,
			MessageID:  state.MessageID,
			Resolution: resolution,
			Text:       ApprovalsReply(resolution),
		})
	}
	return actions
}

// unreportedResolution reports whether an approval the room has seen still
// needs its resolution reported in its thread: it is no longer one the forge
// waits for, and the thread has not been told yet.
func unreportedResolution(state ApprovalState, key string, byKey map[string]Approval) bool {
	if state.MessageID == "" || state.ReplyID != "" {
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
// {"version":1,"tested_at":"2026-09-23T13:44:28+02:00","module_hash":"e6d7523fe8435894e07bcc8500423efbf901c7e593125c5362c08eddbd72eb92","functions":[{"id":"func/Approves","name":"Approves","line":60,"end_line":60,"hash":"e9a9c788d3529be59309cc6af6bd3b3f8f19d4fa9790fdfc3bc6e597ef64808b"},{"id":"func/PlanApprovals","name":"PlanApprovals","line":93,"end_line":101,"hash":"2d321fa81b8636c660d0615ad8b52171356554b734627180db2cae6c2480b6ee"},{"id":"func/approvalsByMessage","name":"approvalsByMessage","line":106,"end_line":116,"hash":"9c9ccaf39cd05bfdc12bf7d5fafc53e12e872bcbfa9c5bc3ed9ac27f130c4f4c"},{"id":"func/approvedByReaction","name":"approvedByReaction","line":119,"end_line":137,"hash":"b8cb6ef510812b4474786f8b100bc1203aa11871f8f080afcce50cf4f196590d"},{"id":"func/textGestures","name":"textGestures","line":145,"end_line":167,"hash":"4b41513290296b531194d46e75eefef35451706c413a6d2db058048154a1dcd3"},{"id":"func/repliedApproval","name":"repliedApproval","line":172,"end_line":178,"hash":"83c75ab8060928303178a10d9e7cab3d8d4c430dde8424f5271482835f0ac631"},{"id":"func/decisionFor","name":"decisionFor","line":183,"end_line":192,"hash":"d46710175488fee0b70d93ac9583982c6608a5ad479af476c441272e7ba5257e"},{"id":"func/unansweredReactions","name":"unansweredReactions","line":197,"end_line":205,"hash":"24342a0a528b9e7488c3675a19abdf72e7bedcc68b515d86de74dff803330b3c"},{"id":"func/affirmative","name":"affirmative","line":211,"end_line":225,"hash":"09747205691ab6a2d88969c216d979cf77bc1626a2af768c42ab8a93361e22be"},{"id":"func/approvalForText","name":"approvalForText","line":229,"end_line":240,"hash":"4ec4faadd738cba840808b156a1e9c8abb9a94ad5f0ebe9da68542e9f02ce2ca"},{"id":"func/unpostedApprovals","name":"unpostedApprovals","line":244,"end_line":252,"hash":"98ab83a435b4107b8e8437a5cd53e76ed1d11e36779b39bf9eb6505d9a4acd8f"},{"id":"func/resolutionsToReport","name":"resolutionsToReport","line":257,"end_line":276,"hash":"4512351a3e60bb377012307ef1308f49f8bf5df7da6b67532123f962ff2ef1c3"},{"id":"func/unreportedResolution","name":"unreportedResolution","line":281,"end_line":287,"hash":"52a54446563ae25cc3d9e20cf3868fe7c6b1a3078a49abbf9cde593dd407718a"},{"id":"func/ApprovalsReply","name":"ApprovalsReply","line":290,"end_line":299,"hash":"b8a277047b4dd09a8e936509d5d2d5a22fe49d0ce688f000755bf029b619a6f5"},{"id":"func/undecided","name":"undecided","line":301,"end_line":304,"hash":"111471b0a984e4e15086380f5923369b9b7103ddc8290f1a7327544dcfd76fd0"}]}
// mutate4go-manifest-end
