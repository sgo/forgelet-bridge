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
	// ThreadRoot is the message a reply is threaded under, empty when the
	// message starts its own thread.
	ThreadRoot string
	// ReplyTo is the message this one answers, empty when it answers none. A
	// phone replies by quoting the message, which carries this and no thread.
	ReplyTo string
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
	// Pending maps an operator message the forge has taken to its text, until
	// the request the forge queued for it can be paired with it.
	Pending map[string]string `json:"pending,omitempty"`
	// PendingThreads maps an operator message the forge has taken to the thread
	// it was written in, for messages that were themselves replies. A thread
	// cannot start from an event that already carries a relation, so an answer
	// to such a message has to land in the thread the operator wrote in.
	PendingThreads map[string]string `json:"pendingthreads,omitempty"`
	// Approvals maps an approval to what the bridge has done about it.
	Approvals map[string]ApprovalState `json:"approvals,omitempty"`
	// Clarifications maps a clarification to what the bridge has done about
	// it.
	Clarifications map[string]ClarificationState `json:"clarifications,omitempty"`
	// Activity maps a card to the last thing the bridge said about it.
	Activity map[string]CardState `json:"activity,omitempty"`
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
	// SourceThread is the thread that message was written in, empty when the
	// message starts its own thread.
	SourceThread string
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
	if s.Pending == nil {
		s.Pending = map[string]string{}
	}
	if s.PendingThreads == nil {
		s.PendingThreads = map[string]string{}
	}
	if s.Approvals == nil {
		s.Approvals = map[string]ApprovalState{}
	}
	if s.Clarifications == nil {
		s.Clarifications = map[string]ClarificationState{}
	}
	if s.Activity == nil {
		s.Activity = map[string]CardState{}
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
			Body:          OwnWords(event.Body),
			SourceEventID: event.EventID,
			SourceThread:  askedInThread(st, event, events),
		})
	}
	return actions
}

// askedInThread is the thread the operator's answer belongs in. A message they
// wrote in a thread stays there; a message that quotes another one belongs in
// the thread the quoted message sits in, which is where the answer to it
// belongs.
func askedInThread(st State, event RoomEvent, events []RoomEvent) string {
	if event.ThreadRoot != "" {
		return event.ThreadRoot
	}
	if event.ReplyTo == "" {
		return ""
	}
	for _, quoted := range events {
		if quoted.EventID != event.ReplyTo {
			continue
		}
		if quoted.ThreadRoot != "" {
			return quoted.ThreadRoot
		}
		return quoted.EventID
	}
	// The room only carries what was said since the last drain, so a message
	// the bridge posted earlier is known from the bookkeeping instead.
	for _, messageID := range st.Threads {
		if messageID == event.ReplyTo {
			return event.ReplyTo
		}
	}
	return ""
}

// operatorAsked reports whether a room event is an operator message the forge
// has not seen the request for yet.
func operatorAsked(st State, operator string, event RoomEvent) bool {
	if event.Sender != operator || OwnWords(event.Body) == "" {
		return false
	}
	if _, relayed := st.Relayed[event.EventID]; relayed {
		return false
	}
	_, pending := st.Pending[event.EventID]
	return !pending
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

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-23T13:26:59+02:00","module_hash":"0c58e2f431811a07578706568ba3f3c9b324f362c4c6c0cb8594d843a77a4a5d","functions":[{"id":"func/State.EnsureMaps","name":"State.EnsureMaps","line":87,"end_line":112,"hash":"46ba92a69300c001204fee38a1dfbac08c991f7dbcb170cb6100d9d956a6d22d"},{"id":"func/State.Anchor","name":"State.Anchor","line":115,"end_line":118,"hash":"abf97d6be5138314460256860c7bdf0359f0ecd87e8387fd92f8ca81367796ee"},{"id":"func/Plan","name":"Plan","line":122,"end_line":125,"hash":"e8e1aa7566af7251d7b0b643670813af18620a1c90fec9d9e577c2236675b68c"},{"id":"func/operatorRequests","name":"operatorRequests","line":130,"end_line":144,"hash":"18373dd942f49714d460f67728ae9ccac9cc2593998a4c7037ae8fa5a9f2a164"},{"id":"func/askedInThread","name":"askedInThread","line":150,"end_line":174,"hash":"fdb0bff1b4ac1fccbb8b861b21b2a84ef258429ebaee1c394bfab1754fc8048e"},{"id":"func/operatorAsked","name":"operatorAsked","line":178,"end_line":187,"hash":"cd911834c75727b082d685cadf37ce7d90ea75687e72ce42004c9f39b848b7f3"},{"id":"func/forgeRequests","name":"forgeRequests","line":191,"end_line":222,"hash":"e398aed1cf014ef832770002d1a7009f0429769ff6c49454832b2485f3948d2b"},{"id":"func/relayedOrigins","name":"relayedOrigins","line":226,"end_line":232,"hash":"69ffcaa8aacd4a23072cfed3d3a0f2d1464007173f7e1a6d1bcc644bd9c38fb4"}]}
// mutate4go-manifest-end
