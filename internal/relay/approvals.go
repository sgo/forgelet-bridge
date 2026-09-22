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
	// MessageID is the chat message that carries the approval.
	MessageID string `json:"message_id,omitempty"`
	// Resolution is how the approval was resolved: approved, sent_back, or
	// desktop.
	Resolution string `json:"resolution,omitempty"`
	// ReplyID is the thread reply that reported the resolution.
	ReplyID string `json:"reply_id,omitempty"`
}

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

// ApprovalKind names the work an approval action asks for.
type ApprovalKind string

const (
	// PostApproval posts a pending approval into the approvals room.
	PostApproval ApprovalKind = "post_approval"
	// ResolveApproval acts on the approval in the forge.
	ResolveApproval ApprovalKind = "resolve_approval"
	// ReplyApproval reports in the approval's thread how it was resolved.
	ReplyApproval ApprovalKind = "reply_approval"
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
	actions = append(actions, sentBackByReply(operator, st, byMessage, replies)...)
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
		if reaction.Sender != operator || reaction.Key != ApproveReaction {
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

// sentBackByReply plans the approvals the operator sent back with feedback in
// the approval's thread.
func sentBackByReply(operator string, st State, byMessage map[string]Approval, replies []RoomEvent) []ApprovalAction {
	var actions []ApprovalAction
	for _, reply := range replies {
		if reply.Sender != operator || strings.TrimSpace(reply.Body) == "" || reply.ThreadRoot == "" {
			continue
		}
		approval, known := byMessage[reply.ThreadRoot]
		if !known || !undecided(st, approval.Key) {
			continue
		}
		actions = append(actions, ApprovalAction{
			Kind:       ResolveApproval,
			Key:        approval.Key,
			Approval:   approval,
			Resolution: ResolutionSentBack,
			Feedback:   reply.Body,
		})
	}
	return actions
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
