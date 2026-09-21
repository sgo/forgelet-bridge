// Package relay decides what the bridge has to do to keep one forge's chat
// channel and its Matrix room in step, without repeating work after a restart.
package relay

import "strings"

// Request is one chat request as the forge's dashboard holds it.
type Request struct {
	ID       string
	Body     string
	Response string
}

// RoomEvent is one message the bridge has seen in the chat room. Body is the
// decrypted text.
type RoomEvent struct {
	RoomID  string
	EventID string
	Sender  string
	Body    string
}

// State is the bridge's durable bookkeeping. Every map is keyed by the Matrix
// or dashboard identity of the thing it remembers, so a restart repeats nothing.
type State struct {
	// Threads maps a dashboard request id to the chat message its answer is
	// threaded under: the bridge's own message for a forge request, or the
	// operator's message for a request the operator started.
	Threads map[string]string `json:"threads,omitempty"`
	// Replied maps a dashboard request id to the thread reply already sent.
	Replied map[string]string `json:"replied,omitempty"`
	// Relayed maps an operator message to the dashboard request it became.
	Relayed map[string]string `json:"relayed,omitempty"`
}

// Kind names the work an action asks for.
type Kind string

const (
	// PostRequestMessage posts a forge request into the chat room.
	PostRequestMessage Kind = "post_request_message"
	// PostRequestReply answers a chat message in its thread.
	PostRequestReply Kind = "post_request_reply"
	// CreateForgeRequest hands an operator message to the lieutenant.
	CreateForgeRequest Kind = "create_forge_request"
)

// Action is one piece of work for the bridge to carry out.
type Action struct {
	Kind Kind
	// RequestID is the dashboard request a post or reply belongs to.
	RequestID string
	// Body is the text to post, or the operator's text to hand to the forge.
	Body string
	// AnchorEventID is the chat message a reply is threaded under. It is empty
	// when the same plan posts that message first.
	AnchorEventID string
	// SourceEventID is the operator's chat message a new forge request came from.
	SourceEventID string
}

// EnsureMaps makes a state's maps writable.
func (s *State) EnsureMaps() {
	if s.Threads == nil {
		s.Threads = map[string]string{}
	}
	if s.Replied == nil {
		s.Replied = map[string]string{}
	}
	if s.Relayed == nil {
		s.Relayed = map[string]string{}
	}
}

// Anchor returns the chat message a request's answer belongs under.
func (s State) Anchor(requestID string) (string, bool) {
	anchor, ok := s.Threads[requestID]
	return anchor, ok && anchor != ""
}

// Plan works out the actions that catch the room up with the forge and the
// forge up with the room. Messages from anyone but the operator are ignored.
func Plan(operator string, st State, requests []Request, events []RoomEvent) []Action {
	actions := operatorRequests(operator, st, events)
	return append(actions, forgeRequests(st, requests)...)
}

// operatorRequests plans the chat requests the operator's own messages ask
// for: every message from the operator that has not been handed to the forge
// yet becomes one.
func operatorRequests(operator string, st State, events []RoomEvent) []Action {
	var actions []Action
	for _, event := range events {
		if !operatorAsked(st, operator, event) {
			continue
		}
		actions = append(actions, Action{
			Kind:          CreateForgeRequest,
			Body:          event.Body,
			SourceEventID: event.EventID,
		})
	}
	return actions
}

// operatorAsked reports whether a room event is an operator message the forge
// has not seen the request for yet.
func operatorAsked(st State, operator string, event RoomEvent) bool {
	if event.Sender != operator || strings.TrimSpace(event.Body) == "" {
		return false
	}
	_, relayed := st.Relayed[event.EventID]
	return !relayed
}

// forgeRequests plans the chat messages and thread replies the forge's
// requests ask for.
func forgeRequests(st State, requests []Request) []Action {
	var actions []Action
	origins := relayedOrigins(st)
	for _, request := range requests {
		if strings.TrimSpace(request.Body) == "" {
			continue
		}
		anchor, posted := st.Anchor(request.ID)
		if !posted {
			// A request the operator's own message created is already in the
			// room: that message is what its answer belongs under.
			anchor, posted = origins[request.ID]
		}
		if !posted {
			actions = append(actions, Action{
				Kind:      PostRequestMessage,
				RequestID: request.ID,
				Body:      request.Body,
			})
		}
		if request.Response == "" || st.Replied[request.ID] != "" {
			continue
		}
		actions = append(actions, Action{
			Kind:          PostRequestReply,
			RequestID:     request.ID,
			Body:          request.Response,
			AnchorEventID: anchor,
		})
	}
	return actions
}

// relayedOrigins inverts the operator-message bookkeeping: request id to the
// operator message that asked for it.
func relayedOrigins(st State) map[string]string {
	origins := make(map[string]string, len(st.Relayed))
	for eventID, requestID := range st.Relayed {
		origins[requestID] = eventID
	}
	return origins
}
