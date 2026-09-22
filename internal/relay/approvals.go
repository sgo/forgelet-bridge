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
	var actions []ApprovalAction

	byMessage := map[string]Approval{}
	byKey := map[string]Approval{}
	for _, approval := range pending {
		byKey[approval.Key] = approval
		if messageID := st.Approvals[approval.Key].MessageID; messageID != "" {
			byMessage[messageID] = approval
		}
	}

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

	for _, approval := range pending {
		if st.Approvals[approval.Key].MessageID == "" {
			actions = append(actions, ApprovalAction{Kind: PostApproval, Key: approval.Key, Approval: approval})
		}
	}

	for key, state := range st.Approvals {
		if state.MessageID == "" || state.ReplyID != "" || byKey[key].Key != "" {
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
