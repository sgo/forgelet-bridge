// Package relay decides what the bridge has to do to keep one forge's chat
// channel and its Matrix room in step, without repeating work after a restart.
package relay

import "strings"

const (
	// AppCompletedCardReportSource marks a completed-card report created in
	// the Forgelet app rather than written by the operator in Matrix.
	AppCompletedCardReportSource = "app.completed-card-report"
	// AppReportPrefix makes an app-created report visibly relayed without
	// pretending that the bridge account is the operator.
	AppReportPrefix = "From the Forgelet app:\n\n"
)

// Request is one chat request as the forge's dashboard holds it.
type Request struct {
	ID       string
	Source   string
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
	// Waiting maps a state event the bridge wrote to the facts it carried,
	// keyed by the room, the event type and the state key, so a restart writes
	// nothing the room already holds and an item that stops waiting is cleared.
	Waiting map[string]string `json:"waiting,omitempty"`
	// Scopes maps the entries whose keys do not name a room to the forge root
	// that owns them, so removing one forge cannot clear another forge's relay
	// bookkeeping.
	Scopes map[string]string `json:"scopes,omitempty"`
	// ProjectScopes names the project that owns an entry whose key does not
	// name its room. Removing one project therefore cannot clear a sibling's
	// relay bookkeeping in the same forge.
	ProjectScopes map[string]string `json:"project_scopes,omitempty"`
}

// Scope sections name the relay bookkeeping maps whose entries are keyed by an
// event or request rather than by a room.
const (
	ScopeThreads        = "threads"
	ScopeReplied        = "replied"
	ScopeRelayed        = "relayed"
	ScopePending        = "pending"
	ScopePendingThreads = "pendingthreads"
)

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
	ensureMap(&s.Threads)
	ensureMap(&s.Replied)
	ensureMap(&s.Relayed)
	ensureMap(&s.Pending)
	ensureMap(&s.PendingThreads)
	ensureMap(&s.Approvals)
	ensureMap(&s.Clarifications)
	ensureMap(&s.Activity)
	ensureMap(&s.Waiting)
	ensureMap(&s.Scopes)
	ensureMap(&s.ProjectScopes)
}

func ensureMap[V any](value *map[string]V) {
	if *value == nil {
		*value = map[string]V{}
	}
}

// Scope records the forge root that owns one relay-state entry.
func (s *State) Scope(section, key, forgeRoot string) {
	s.EnsureMaps()
	s.Scopes[scopeKey(section, key)] = forgeRoot
}

// ScopeProject records both the forge and the project that own one relay-state
// entry. Project removal uses the narrower owner when it is available.
func (s *State) ScopeProject(section, key, forgeRoot, project string) {
	s.Scope(section, key, forgeRoot)
	s.ProjectScopes[scopeKey(section, key)] = project
}

// Unscope forgets the ownership of one relay-state entry after that entry is
// consumed.
func (s *State) Unscope(section, key string) {
	s.EnsureMaps()
	delete(s.Scopes, scopeKey(section, key))
	delete(s.ProjectScopes, scopeKey(section, key))
}

// InScope reports whether one relay-state entry belongs to a forge root.
func (s State) InScope(section, key, forgeRoot string) bool {
	return s.ScopeOwner(section, key) == forgeRoot
}

// ScopeOwner names the forge root that owns one relay-state entry. An empty
// result is state written before scopes were recorded.
func (s State) ScopeOwner(section, key string) string {
	return s.Scopes[scopeKey(section, key)]
}

// ProjectScopeOwner names the project that owns one relay-state entry.
func (s State) ProjectScopeOwner(section, key string) string {
	return s.ProjectScopes[scopeKey(section, key)]
}

// ForgetScoped removes the relay entries scoped to one forge root.
func (s *State) ForgetScoped(forgeRoot string) {
	s.EnsureMaps()
	for scoped, owner := range s.Scopes {
		if owner != forgeRoot {
			continue
		}
		s.deleteScoped(scoped)
	}
}

// ForgetProjectScoped removes relay entries owned by one project without
// touching entries owned by its forge or a sibling project.
func (s *State) ForgetProjectScoped(forgeRoot, project string) {
	s.EnsureMaps()
	for scoped, owner := range s.ProjectScopes {
		if owner != project || s.Scopes[scoped] != forgeRoot {
			continue
		}
		s.deleteScoped(scoped)
	}
}

func (s *State) deleteScoped(scoped string) {
	section, key, ok := splitScope(scoped)
	if !ok {
		return
	}
	deleters := map[string]func(){
		ScopeThreads:        func() { delete(s.Threads, key) },
		ScopeReplied:        func() { delete(s.Replied, key) },
		ScopeRelayed:        func() { delete(s.Relayed, key) },
		ScopePending:        func() { delete(s.Pending, key) },
		ScopePendingThreads: func() { delete(s.PendingThreads, key) },
	}
	if remove, known := deleters[section]; known {
		remove()
	}
	delete(s.Scopes, scoped)
	delete(s.ProjectScopes, scoped)
}

func scopeKey(section, key string) string {
	return section + "\n" + key
}

func splitScope(scoped string) (string, string, bool) {
	section, key, found := strings.Cut(scoped, "\n")
	return section, key, found && section != "" && key != ""
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
	return quotedThread(st, event.ReplyTo, events)
}

func quotedThread(st State, replyTo string, events []RoomEvent) string {
	for _, quoted := range events {
		if quoted.EventID != replyTo {
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
		if messageID == replyTo {
			return replyTo
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
		actions = append(actions, requestActions(st, request, origins)...)
	}
	return actions
}

func requestActions(st State, request Request, origins map[string]string) []Action {
	if strings.TrimSpace(request.Body) == "" {
		return nil
	}
	anchor, posted := requestAnchor(st, request.ID, origins)
	var actions []Action
	if !posted {
		actions = append(actions, Action{Kind: PostRequestMessage, RequestID: request.ID, Body: presentedRequest(request)})
	}
	if request.Response != "" && st.Replied[request.ID] == "" {
		actions = append(actions, Action{
			Kind: PostRequestReply, RequestID: request.ID, Body: request.Response, AnchorEventID: anchor,
		})
	}
	return actions
}

// presentedRequest keeps the dashboard report unchanged while making an item
// whose explicit source is the Forgelet app visibly relayed.
func presentedRequest(request Request) string {
	if request.Source == AppCompletedCardReportSource {
		return AppReportPrefix + request.Body
	}
	return request.Body
}

func requestAnchor(st State, requestID string, origins map[string]string) (string, bool) {
	if anchor, posted := st.Anchor(requestID); posted {
		return anchor, true
	}
	anchor, posted := origins[requestID]
	return anchor, posted
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
// {"version":1,"tested_at":"2026-11-09T11:50:29+01:00","module_hash":"4fc98c4c6d50ef591b749e7d5d24caa17f0e78dcab8d044ff9bd4d106b190608","functions":[{"id":"func/State.EnsureMaps","name":"State.EnsureMaps","line":119,"end_line":131,"hash":"dbfb3691c68bd7883a63b74e862815c60254e186a0852f5019829649abce384c"},{"id":"func/ensureMap","name":"ensureMap","line":133,"end_line":137,"hash":"014008beb4dbd40d9d9efd28e03d7ceefa9693fcfc89935335de6f4453756a2d"},{"id":"func/State.Scope","name":"State.Scope","line":140,"end_line":143,"hash":"cb3e032cbd5357eb5f04905526ce9643233d9fabe459407027dfc38c9022a7c8"},{"id":"func/State.ScopeProject","name":"State.ScopeProject","line":147,"end_line":150,"hash":"bfff10a3d32e00445487b9ed4532abdb15235acda05c5b3ed9f6ed8077a9f262"},{"id":"func/State.Unscope","name":"State.Unscope","line":154,"end_line":158,"hash":"2cebdd996dd7238adee36d76eaec06c18bd492a3e0e80cd79ff696c58c7fc627"},{"id":"func/State.InScope","name":"State.InScope","line":161,"end_line":163,"hash":"07e1e701f85205ef5511687b2055e6b290a202e712e3e70cbae4d5b84aa8856b"},{"id":"func/State.ScopeOwner","name":"State.ScopeOwner","line":167,"end_line":169,"hash":"96eb0f815c9f6b2b99412cf37611bbd9450abf371d2ad54d3f105125ba040aa3"},{"id":"func/State.ProjectScopeOwner","name":"State.ProjectScopeOwner","line":172,"end_line":174,"hash":"dc3b7dfcee84800b4251f8244d89f274e85810c0c0dee567ff5046d0df3f1daf"},{"id":"func/State.ForgetScoped","name":"State.ForgetScoped","line":177,"end_line":185,"hash":"7d6fa14f9f834f939841c68ab7fb37ff69e482d1fdcbc3a6b435958b1f4d5316"},{"id":"func/State.ForgetProjectScoped","name":"State.ForgetProjectScoped","line":189,"end_line":197,"hash":"dcf4b7efd87b8962efe4927ee2b9c414ef03b070570e8bc6e4ce2253c09d8437"},{"id":"func/State.deleteScoped","name":"State.deleteScoped","line":199,"end_line":216,"hash":"41daa578f5cef42266899603ca4ca32d64522101f1af97d4bc6d5a0b53449631"},{"id":"func/scopeKey","name":"scopeKey","line":218,"end_line":220,"hash":"7e453148ff535cdda1042cd0474417267d12eed2c9a2626b53b052f84da377b8"},{"id":"func/splitScope","name":"splitScope","line":222,"end_line":225,"hash":"00c9a7897c3cf4290e296ce726e8eae1835dfdb056ed38df0282de26383f3b69"},{"id":"func/State.Anchor","name":"State.Anchor","line":228,"end_line":231,"hash":"abf97d6be5138314460256860c7bdf0359f0ecd87e8387fd92f8ca81367796ee"},{"id":"func/Plan","name":"Plan","line":235,"end_line":238,"hash":"e8e1aa7566af7251d7b0b643670813af18620a1c90fec9d9e577c2236675b68c"},{"id":"func/operatorRequests","name":"operatorRequests","line":243,"end_line":257,"hash":"18373dd942f49714d460f67728ae9ccac9cc2593998a4c7037ae8fa5a9f2a164"},{"id":"func/askedInThread","name":"askedInThread","line":263,"end_line":271,"hash":"aa22bd0d1bc559e0ef48ddb9df9dede144c32e4b2842b6589f0db99564a5675a"},{"id":"func/quotedThread","name":"quotedThread","line":273,"end_line":291,"hash":"0120757acfb0bb7e3c361453f8e5102ecfa291661ce707567a51c918fe57f933"},{"id":"func/operatorAsked","name":"operatorAsked","line":295,"end_line":304,"hash":"cd911834c75727b082d685cadf37ce7d90ea75687e72ce42004c9f39b848b7f3"},{"id":"func/forgeRequests","name":"forgeRequests","line":308,"end_line":315,"hash":"185dee07734e8508194a9c5c9cdf9689012df3d3f60ee7554baf86a05e986175"},{"id":"func/requestActions","name":"requestActions","line":317,"end_line":332,"hash":"e84a74467c555cc6a72e24cee9c8ea4ade55198a226452b07966419da8ee2fe1"},{"id":"func/presentedRequest","name":"presentedRequest","line":336,"end_line":341,"hash":"e29fa658b251ba7bb9979db65f010f3d8e964a59190b3525db9fc03a2c39d643"},{"id":"func/requestAnchor","name":"requestAnchor","line":343,"end_line":349,"hash":"2f665320db8fcf0d2bf78a2b04599316a450475e042be0495f6e923b0ea1e77a"},{"id":"func/relayedOrigins","name":"relayedOrigins","line":353,"end_line":359,"hash":"69ffcaa8aacd4a23072cfed3d3a0f2d1464007173f7e1a6d1bcc644bd9c38fb4"}]}
// mutate4go-manifest-end
