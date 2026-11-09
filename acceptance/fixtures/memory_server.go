package fixtures

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type memoryIdentity struct {
	UserID   id.UserID
	DeviceID id.DeviceID
}

type memoryEvent struct {
	seq int64
	evt *event.Event
}

type memoryRoom struct {
	ID      id.RoomID
	Events  []memoryEvent
	State   map[event.Type]map[string]*event.Event
	Members map[id.UserID]event.Membership
}

type memoryDeviceInbox map[id.UserID]map[id.DeviceID][]memoryEvent

// MemoryHomeserver is a test-only Matrix Client API server. It relays opaque
// encrypted events and enough key/device traffic for real mautrix clients.
type MemoryHomeserver struct {
	server *httptest.Server

	mu          sync.Mutex
	seq         int64
	roomSeq     int
	eventSeq    int
	users       map[string]string
	tokens      map[string]memoryIdentity
	rooms       map[id.RoomID]*memoryRoom
	deviceKeys  map[id.UserID]map[id.DeviceID]mautrix.DeviceKeys
	oneTimeKeys map[id.UserID]map[id.DeviceID]map[id.KeyID]mautrix.OneTimeKey
	inbox       memoryDeviceInbox
	accountData map[id.UserID]map[event.Type]json.RawMessage
	changed     chan struct{}
}

// StartMemoryHomeserver starts an in-process homeserver for one scenario.
func StartMemoryHomeserver(ctx context.Context) (*MemoryHomeserver, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	server := &MemoryHomeserver{
		users:       map[string]string{},
		tokens:      map[string]memoryIdentity{},
		rooms:       map[id.RoomID]*memoryRoom{},
		deviceKeys:  map[id.UserID]map[id.DeviceID]mautrix.DeviceKeys{},
		oneTimeKeys: map[id.UserID]map[id.DeviceID]map[id.KeyID]mautrix.OneTimeKey{},
		inbox:       memoryDeviceInbox{},
		accountData: map[id.UserID]map[event.Type]json.RawMessage{},
		changed:     make(chan struct{}),
	}
	server.server = httptest.NewServer(server.routes())
	return server, nil
}

// BaseURL is the address fixture clients use.
func (s *MemoryHomeserver) BaseURL() string { return s.server.URL }

// Backend names this homeserver implementation.
func (s *MemoryHomeserver) Backend() MatrixBackend { return MatrixBackendMemory }

// Stop shuts down the in-process server.
func (s *MemoryHomeserver) Stop() {
	if s != nil && s.server != nil {
		s.server.CloseClientConnections()
		s.server.Close()
	}
}

func (s *MemoryHomeserver) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /_matrix/client/versions", s.versions)
	mux.HandleFunc("POST /_matrix/client/v3/register", s.register)
	mux.HandleFunc("GET /_matrix/client/v3/login", s.loginFlows)
	mux.HandleFunc("POST /_matrix/client/v3/login", s.login)
	mux.HandleFunc("GET /_matrix/client/v3/sync", s.sync)
	mux.HandleFunc("POST /_matrix/client/v3/user/{user}/filter", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"filter_id": "0"})
	})
	mux.HandleFunc("GET /_matrix/client/v3/joined_rooms", s.joinedRooms)
	mux.HandleFunc("POST /_matrix/client/v3/createRoom", s.createRoom)
	mux.HandleFunc("GET /_matrix/client/v3/rooms/{room}/state", s.state)
	mux.HandleFunc("GET /_matrix/client/v3/rooms/{room}/state/{type}/{key...}", s.stateEvent)
	mux.HandleFunc("PUT /_matrix/client/v3/rooms/{room}/state/{type}/{key...}", s.sendState)
	mux.HandleFunc("PUT /_matrix/client/v3/rooms/{room}/send/{type}/{txn}", s.sendMessage)
	mux.HandleFunc("GET /_matrix/client/v3/rooms/{room}/messages", s.messages)
	mux.HandleFunc("POST /_matrix/client/v3/rooms/{room}/invite", s.invite)
	mux.HandleFunc("POST /_matrix/client/v3/rooms/{room}/join", s.join)
	mux.HandleFunc("POST /_matrix/client/v3/keys/upload", s.uploadKeys)
	mux.HandleFunc("POST /_matrix/client/v3/keys/query", s.queryKeys)
	mux.HandleFunc("POST /_matrix/client/v3/keys/claim", s.claimKeys)
	mux.HandleFunc("PUT /_matrix/client/v3/sendToDevice/{type}/{txn}", s.sendToDevice)
	mux.HandleFunc("POST /_matrix/client/v3/keys/device_signing/upload", s.deviceSigning)
	mux.HandleFunc("POST /_matrix/client/v3/keys/signatures/upload", s.empty)
	mux.HandleFunc("PUT /_matrix/client/v3/user/{user}/account_data/{type}", s.putAccountData)
	mux.HandleFunc("GET /_matrix/client/v3/account/whoami", s.whoami)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(os.Stderr, "memory matrix: unsupported %s %s\n", r.Method, r.URL.RequestURI())
		writeMatrixError(w, http.StatusNotFound, "M_UNRECOGNIZED", "unsupported endpoint")
	})
	return mux
}

func (s *MemoryHomeserver) register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Username) == "" {
		writeMatrixError(w, http.StatusBadRequest, "M_INVALID_USERNAME", "username is required")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.users[req.Username]; exists {
		writeMatrixError(w, http.StatusBadRequest, "M_USER_IN_USE", "username is already taken")
		return
	}
	s.users[req.Username] = req.Password
	identity, token := s.newLogin(req.Username)
	writeJSON(w, http.StatusOK, &mautrix.RespRegister{
		AccessToken: token,
		DeviceID:    identity.DeviceID,
		UserID:      identity.UserID,
	})
}

func (s *MemoryHomeserver) loginFlows(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, &mautrix.RespLoginFlows{Flows: []mautrix.LoginFlow{{Type: mautrix.AuthTypePassword}}})
}

func (s *MemoryHomeserver) login(w http.ResponseWriter, r *http.Request) {
	var req mautrix.ReqLogin
	if !decodeJSON(w, r, &req) {
		return
	}
	localpart := localpartFromUser(req.Identifier.User)
	s.mu.Lock()
	defer s.mu.Unlock()
	if password, ok := s.users[localpart]; !ok || password != req.Password {
		writeMatrixError(w, http.StatusForbidden, "M_FORBIDDEN", "invalid username or password")
		return
	}
	identity, token := s.newLogin(localpart)
	writeJSON(w, http.StatusOK, &mautrix.RespLogin{
		AccessToken: token,
		DeviceID:    identity.DeviceID,
		UserID:      identity.UserID,
	})
}

func (s *MemoryHomeserver) newLogin(localpart string) (memoryIdentity, string) {
	s.eventSeq++
	identity := memoryIdentity{
		UserID:   id.UserID("@" + localpart + ":" + ServerName),
		DeviceID: id.DeviceID(fmt.Sprintf("DEVICE%d", s.eventSeq)),
	}
	token := fmt.Sprintf("memory-token-%d", s.eventSeq)
	s.tokens[token] = identity
	return identity, token
}

// Register creates a memory-server user.
func (s *MemoryHomeserver) Register(ctx context.Context, localpart, password string) (userID, accessToken, deviceID string, err error) {
	body, _ := json.Marshal(map[string]any{"username": localpart, "password": password})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.BaseURL()+"/_matrix/client/v3/register", strings.NewReader(string(body)))
	if err != nil {
		return "", "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()
	var login mautrix.RespRegister
	if err := json.NewDecoder(resp.Body).Decode(&login); err != nil {
		return "", "", "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", "", fmt.Errorf("memory register %s: HTTP %d", localpart, resp.StatusCode)
	}
	return login.UserID.String(), login.AccessToken, login.DeviceID.String(), nil
}

func (s *MemoryHomeserver) versions(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"versions":          []string{"v1.1", "v1.2", "v1.3", "v1.4", "v1.5", "v1.6", "v1.7", "v1.8", "v1.9", "v1.10", "v1.11"},
		"unstable_features": map[string]bool{},
	})
}

func (s *MemoryHomeserver) whoami(w http.ResponseWriter, r *http.Request) {
	identity, ok := s.authenticate(r)
	if !ok {
		writeMatrixError(w, http.StatusUnauthorized, "M_UNKNOWN_TOKEN", "unknown token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"user_id": identity.UserID.String(), "device_id": identity.DeviceID.String()})
}

func (s *MemoryHomeserver) createRoom(w http.ResponseWriter, r *http.Request) {
	identity, ok := s.authenticate(r)
	if !ok {
		writeMatrixError(w, http.StatusUnauthorized, "M_UNKNOWN_TOKEN", "unknown token")
		return
	}
	var req mautrix.ReqCreateRoom
	if !decodeJSON(w, r, &req) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.roomSeq++
	roomID := id.RoomID(fmt.Sprintf("!memory%d:%s", s.roomSeq, ServerName))
	room := &memoryRoom{
		ID:      roomID,
		State:   map[event.Type]map[string]*event.Event{},
		Members: map[id.UserID]event.Membership{identity.UserID: event.MembershipJoin},
	}
	s.rooms[roomID] = room
	s.appendState(room, identity.UserID, event.StateCreate, "", map[string]any{
		"creator":      identity.UserID.String(),
		"room_version": "10",
		"type":         req.CreationContent["type"],
	})
	s.appendState(room, identity.UserID, event.StateMember, identity.UserID.String(), map[string]any{
		"membership": "join",
	})
	if req.Name != "" {
		s.appendState(room, identity.UserID, event.StateRoomName, "", map[string]any{"name": req.Name})
	}
	for _, invited := range req.Invite {
		room.Members[invited] = event.MembershipInvite
		s.appendState(room, identity.UserID, event.StateMember, invited.String(), map[string]any{"membership": "invite"})
	}
	for _, evt := range req.InitialState {
		s.appendState(room, identity.UserID, evt.Type, evt.GetStateKey(), evt.Content.Raw)
	}
	s.signal()
	writeJSON(w, http.StatusOK, &mautrix.RespCreateRoom{RoomID: roomID})
}

func (s *MemoryHomeserver) joinedRooms(w http.ResponseWriter, r *http.Request) {
	identity, ok := s.authenticate(r)
	if !ok {
		writeMatrixError(w, http.StatusUnauthorized, "M_UNKNOWN_TOKEN", "unknown token")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rooms := make([]id.RoomID, 0)
	for roomID, room := range s.rooms {
		if room.Members[identity.UserID] == event.MembershipJoin {
			rooms = append(rooms, roomID)
		}
	}
	writeJSON(w, http.StatusOK, &mautrix.RespJoinedRooms{JoinedRooms: rooms})
}

func (s *MemoryHomeserver) state(w http.ResponseWriter, r *http.Request) {
	identity, ok := s.authenticate(r)
	if !ok {
		writeMatrixError(w, http.StatusUnauthorized, "M_UNKNOWN_TOKEN", "unknown token")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	room, ok := s.accessibleRoom(w, r.PathValue("room"), identity)
	if !ok {
		return
	}
	state := make([]*event.Event, 0)
	for _, byKey := range room.State {
		for _, evt := range byKey {
			state = append(state, evt)
		}
	}
	writeJSON(w, http.StatusOK, state)
}

func (s *MemoryHomeserver) stateEvent(w http.ResponseWriter, r *http.Request) {
	identity, ok := s.authenticate(r)
	if !ok {
		writeMatrixError(w, http.StatusUnauthorized, "M_UNKNOWN_TOKEN", "unknown token")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	room, ok := s.accessibleRoom(w, r.PathValue("room"), identity)
	if !ok {
		return
	}
	evt := room.State[event.Type{Type: r.PathValue("type"), Class: event.StateEventType}][r.PathValue("key")]
	if evt == nil {
		writeMatrixError(w, http.StatusNotFound, "M_NOT_FOUND", "state event not found")
		return
	}
	writeJSON(w, http.StatusOK, evt.Content.Raw)
}

func (s *MemoryHomeserver) sendState(w http.ResponseWriter, r *http.Request) {
	identity, ok := s.authenticate(r)
	if !ok {
		writeMatrixError(w, http.StatusUnauthorized, "M_UNKNOWN_TOKEN", "unknown token")
		return
	}
	var raw map[string]interface{}
	if !decodeJSON(w, r, &raw) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	room, ok := s.accessibleRoom(w, r.PathValue("room"), identity)
	if !ok {
		return
	}
	evt := s.appendState(room, identity.UserID,
		event.Type{Type: r.PathValue("type"), Class: event.StateEventType},
		r.PathValue("key"), raw)
	s.signal()
	writeJSON(w, http.StatusOK, &mautrix.RespSendEvent{EventID: evt.ID})
}

func (s *MemoryHomeserver) sendMessage(w http.ResponseWriter, r *http.Request) {
	identity, ok := s.authenticate(r)
	if !ok {
		writeMatrixError(w, http.StatusUnauthorized, "M_UNKNOWN_TOKEN", "unknown token")
		return
	}
	var raw map[string]interface{}
	if !decodeJSON(w, r, &raw) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	room, ok := s.accessibleRoom(w, r.PathValue("room"), identity)
	if !ok {
		return
	}
	evt := s.appendTimeline(room, identity.UserID,
		event.Type{Type: r.PathValue("type"), Class: event.MessageEventType}, raw)
	s.signal()
	writeJSON(w, http.StatusOK, &mautrix.RespSendEvent{EventID: evt.ID})
}

func (s *MemoryHomeserver) appendState(room *memoryRoom, sender id.UserID, evtType event.Type, key string, content any) *event.Event {
	marshaled, _ := json.Marshal(content)
	var raw map[string]interface{}
	_ = json.Unmarshal(marshaled, &raw)
	evt := s.newEvent(room, sender, evtType, &key, raw)
	if room.State[evtType] == nil {
		room.State[evtType] = map[string]*event.Event{}
	}
	room.State[evtType][key] = evt
	room.Events = append(room.Events, memoryEvent{seq: s.seq, evt: evt})
	return evt
}

func (s *MemoryHomeserver) appendTimeline(room *memoryRoom, sender id.UserID, evtType event.Type, raw map[string]interface{}) *event.Event {
	evt := s.newEvent(room, sender, evtType, nil, raw)
	room.Events = append(room.Events, memoryEvent{seq: s.seq, evt: evt})
	return evt
}

func (s *MemoryHomeserver) newEvent(room *memoryRoom, sender id.UserID, evtType event.Type, stateKey *string, raw map[string]interface{}) *event.Event {
	s.seq++
	s.eventSeq++
	return &event.Event{
		StateKey:  stateKey,
		Sender:    sender,
		Type:      evtType,
		Timestamp: time.Now().UnixMilli(),
		ID:        id.EventID(fmt.Sprintf("$memory%d:%s", s.eventSeq, ServerName)),
		RoomID:    room.ID,
		Content:   event.Content{Raw: raw},
	}
}

func (s *MemoryHomeserver) messages(w http.ResponseWriter, r *http.Request) {
	identity, ok := s.authenticate(r)
	if !ok {
		writeMatrixError(w, http.StatusUnauthorized, "M_UNKNOWN_TOKEN", "unknown token")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	room, ok := s.accessibleRoom(w, r.PathValue("room"), identity)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > len(room.Events) {
		limit = len(room.Events)
	}
	backward := r.URL.Query().Get("dir") == "b"
	events := make([]*event.Event, 0, limit)
	if backward {
		for i := len(room.Events) - 1; i >= 0 && len(events) < limit; i-- {
			events = append(events, room.Events[i].evt)
		}
	} else {
		for i := 0; i < len(room.Events) && len(events) < limit; i++ {
			events = append(events, room.Events[i].evt)
		}
	}
	writeJSON(w, http.StatusOK, &mautrix.RespMessages{Chunk: events})
}

func (s *MemoryHomeserver) invite(w http.ResponseWriter, r *http.Request) {
	identity, ok := s.authenticate(r)
	if !ok {
		writeMatrixError(w, http.StatusUnauthorized, "M_UNKNOWN_TOKEN", "unknown token")
		return
	}
	var req mautrix.ReqInviteUser
	if !decodeJSON(w, r, &req) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	room, ok := s.accessibleRoom(w, r.PathValue("room"), identity)
	if !ok {
		return
	}
	if room.Members[req.UserID] == event.MembershipJoin {
		s.signal()
		writeJSON(w, http.StatusOK, &mautrix.RespInviteUser{})
		return
	}
	room.Members[req.UserID] = event.MembershipInvite
	s.appendState(room, identity.UserID, event.StateMember, req.UserID.String(), map[string]any{"membership": "invite"})
	s.signal()
	writeJSON(w, http.StatusOK, &mautrix.RespInviteUser{})
}

func (s *MemoryHomeserver) join(w http.ResponseWriter, r *http.Request) {
	identity, ok := s.authenticate(r)
	if !ok {
		writeMatrixError(w, http.StatusUnauthorized, "M_UNKNOWN_TOKEN", "unknown token")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	room, ok := s.accessibleRoom(w, r.PathValue("room"), identity)
	if !ok {
		return
	}
	room.Members[identity.UserID] = event.MembershipJoin
	s.appendState(room, identity.UserID, event.StateMember, identity.UserID.String(), map[string]any{"membership": "join"})
	s.signal()
	writeJSON(w, http.StatusOK, &mautrix.RespJoinRoom{RoomID: room.ID})
}

func (s *MemoryHomeserver) sync(w http.ResponseWriter, r *http.Request) {
	identity, ok := s.authenticate(r)
	if !ok {
		writeMatrixError(w, http.StatusUnauthorized, "M_UNKNOWN_TOKEN", "unknown token")
		return
	}
	since, _ := strconv.ParseInt(strings.TrimPrefix(r.URL.Query().Get("since"), "s"), 10, 64)
	var response *mautrix.RespSync
	for {
		s.mu.Lock()
		response = s.syncResponse(identity, since)
		changed := s.changed
		current := s.seq
		s.mu.Unlock()
		if current > since || responseHasData(response) {
			writeJSON(w, http.StatusOK, response)
			return
		}
		timeout := 500 * time.Millisecond
		if raw := r.URL.Query().Get("timeout"); raw != "" {
			if millis, err := strconv.Atoi(raw); err == nil && millis >= 0 {
				timeout = time.Duration(millis) * time.Millisecond
			}
		}
		if timeout <= 0 {
			timeout = 100 * time.Millisecond
		}
		timer := time.NewTimer(timeout)
		select {
		case <-r.Context().Done():
			timer.Stop()
			return
		case <-changed:
		case <-timer.C:
			writeJSON(w, http.StatusOK, response)
			return
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}
}

func (s *MemoryHomeserver) syncResponse(identity memoryIdentity, since int64) *mautrix.RespSync {
	resp := &mautrix.RespSync{
		NextBatch: fmt.Sprintf("s%d", s.seq),
		Rooms: mautrix.RespSyncRooms{
			Join:   map[id.RoomID]*mautrix.SyncJoinedRoom{},
			Invite: map[id.RoomID]*mautrix.SyncInvitedRoom{},
		},
	}
	for roomID, room := range s.rooms {
		membership := room.Members[identity.UserID]
		if membership == event.MembershipInvite {
			resp.Rooms.Invite[roomID] = &mautrix.SyncInvitedRoom{State: mautrix.SyncEventsList{Events: stateEvents(room)}}
			continue
		}
		if membership != event.MembershipJoin {
			continue
		}
		joined := &mautrix.SyncJoinedRoom{}
		if since == 0 {
			joined.State.Events = stateEvents(room)
		}
		for _, item := range room.Events {
			if item.seq > since {
				joined.Timeline.Events = append(joined.Timeline.Events, item.evt)
			}
		}
		if len(joined.State.Events) > 0 || len(joined.Timeline.Events) > 0 {
			resp.Rooms.Join[roomID] = joined
		}
	}
	for _, item := range s.inbox[identity.UserID][identity.DeviceID] {
		if item.seq > since {
			resp.ToDevice.Events = append(resp.ToDevice.Events, item.evt)
		}
	}
	for _, raw := range s.accountData[identity.UserID] {
		var content map[string]interface{}
		_ = json.Unmarshal(raw, &content)
		resp.AccountData.Events = append(resp.AccountData.Events, &event.Event{Content: event.Content{Raw: content}})
	}
	return resp
}

func stateEvents(room *memoryRoom) []*event.Event {
	var events []*event.Event
	for _, byKey := range room.State {
		for _, evt := range byKey {
			events = append(events, evt)
		}
	}
	return events
}

func responseHasData(resp *mautrix.RespSync) bool {
	return len(resp.Rooms.Join) > 0 || len(resp.Rooms.Invite) > 0 ||
		len(resp.ToDevice.Events) > 0 || len(resp.AccountData.Events) > 0
}

func (s *MemoryHomeserver) uploadKeys(w http.ResponseWriter, r *http.Request) {
	identity, ok := s.authenticate(r)
	if !ok {
		writeMatrixError(w, http.StatusUnauthorized, "M_UNKNOWN_TOKEN", "unknown token")
		return
	}
	var req mautrix.ReqUploadKeys
	if !decodeJSON(w, r, &req) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deviceKeys[identity.UserID] == nil {
		s.deviceKeys[identity.UserID] = map[id.DeviceID]mautrix.DeviceKeys{}
	}
	if s.oneTimeKeys[identity.UserID] == nil {
		s.oneTimeKeys[identity.UserID] = map[id.DeviceID]map[id.KeyID]mautrix.OneTimeKey{}
	}
	if req.DeviceKeys != nil {
		s.deviceKeys[identity.UserID][identity.DeviceID] = *req.DeviceKeys
	}
	keys := s.oneTimeKeys[identity.UserID][identity.DeviceID]
	if keys == nil {
		keys = map[id.KeyID]mautrix.OneTimeKey{}
		s.oneTimeKeys[identity.UserID][identity.DeviceID] = keys
	}
	for keyID, key := range req.OneTimeKeys {
		keys[keyID] = key
	}
	s.signal()
	writeJSON(w, http.StatusOK, &mautrix.RespUploadKeys{
		OneTimeKeyCounts: mautrix.OTKCount{SignedCurve25519: len(keys)},
	})
}

func (s *MemoryHomeserver) queryKeys(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticate(r); !ok {
		writeMatrixError(w, http.StatusUnauthorized, "M_UNKNOWN_TOKEN", "unknown token")
		return
	}
	var req mautrix.ReqQueryKeys
	if !decodeJSON(w, r, &req) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	resp := &mautrix.RespQueryKeys{
		DeviceKeys:      map[id.UserID]map[id.DeviceID]mautrix.DeviceKeys{},
		MasterKeys:      map[id.UserID]mautrix.CrossSigningKeys{},
		SelfSigningKeys: map[id.UserID]mautrix.CrossSigningKeys{},
		UserSigningKeys: map[id.UserID]mautrix.CrossSigningKeys{},
	}
	for userID, devices := range req.DeviceKeys {
		resp.DeviceKeys[userID] = map[id.DeviceID]mautrix.DeviceKeys{}
		for deviceID, key := range s.deviceKeys[userID] {
			if len(devices) == 0 || containsDevice(devices, deviceID) {
				resp.DeviceKeys[userID][deviceID] = key
			}
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *MemoryHomeserver) claimKeys(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authenticate(r); !ok {
		writeMatrixError(w, http.StatusUnauthorized, "M_UNKNOWN_TOKEN", "unknown token")
		return
	}
	var req mautrix.ReqClaimKeys
	if !decodeJSON(w, r, &req) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	resp := &mautrix.RespClaimKeys{OneTimeKeys: map[id.UserID]map[id.DeviceID]map[id.KeyID]mautrix.OneTimeKey{}}
	for userID, devices := range req.OneTimeKeys {
		resp.OneTimeKeys[userID] = map[id.DeviceID]map[id.KeyID]mautrix.OneTimeKey{}
		for deviceID := range devices {
			keys := s.oneTimeKeys[userID][deviceID]
			for keyID, key := range keys {
				resp.OneTimeKeys[userID][deviceID] = map[id.KeyID]mautrix.OneTimeKey{keyID: key}
				delete(keys, keyID)
				break
			}
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *MemoryHomeserver) sendToDevice(w http.ResponseWriter, r *http.Request) {
	identity, ok := s.authenticate(r)
	if !ok {
		writeMatrixError(w, http.StatusUnauthorized, "M_UNKNOWN_TOKEN", "unknown token")
		return
	}
	var req mautrix.ReqSendToDevice
	if !decodeJSON(w, r, &req) {
		return
	}
	evtType := event.Type{Type: r.PathValue("type"), Class: event.ToDeviceEventType}
	s.mu.Lock()
	defer s.mu.Unlock()
	for userID, devices := range req.Messages {
		if s.inbox[userID] == nil {
			s.inbox[userID] = map[id.DeviceID][]memoryEvent{}
		}
		for deviceID, content := range devices {
			s.seq++
			evt := &event.Event{
				Sender:  identity.UserID,
				Type:    evtType,
				ID:      id.EventID(fmt.Sprintf("$to-device%d:%s", s.seq, ServerName)),
				Content: event.Content{Raw: content.Raw},
			}
			s.inbox[userID][deviceID] = append(s.inbox[userID][deviceID], memoryEvent{seq: s.seq, evt: evt})
		}
	}
	s.signal()
	writeJSON(w, http.StatusOK, &mautrix.RespSendToDevice{})
}

func (s *MemoryHomeserver) deviceSigning(w http.ResponseWriter, _ *http.Request) {
	s.empty(w, nil)
}

func (s *MemoryHomeserver) putAccountData(w http.ResponseWriter, r *http.Request) {
	userID := id.UserID(r.PathValue("user"))
	evtType := event.Type{Type: r.PathValue("type"), Class: event.AccountDataEventType}
	raw, _ := io.ReadAll(r.Body)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.accountData[userID] == nil {
		s.accountData[userID] = map[event.Type]json.RawMessage{}
	}
	s.accountData[userID][evtType] = append(json.RawMessage(nil), raw...)
	s.signal()
	s.empty(w, nil)
}

func (s *MemoryHomeserver) empty(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{})
}

func (s *MemoryHomeserver) accessibleRoom(w http.ResponseWriter, rawRoom string, identity memoryIdentity) (*memoryRoom, bool) {
	room := s.rooms[id.RoomID(rawRoom)]
	if room == nil {
		writeMatrixError(w, http.StatusNotFound, "M_NOT_FOUND", "unknown room")
		return nil, false
	}
	membership := room.Members[identity.UserID]
	if membership != event.MembershipJoin && membership != event.MembershipInvite {
		writeMatrixError(w, http.StatusForbidden, "M_FORBIDDEN", "not a room member")
		return nil, false
	}
	return room, true
}

func (s *MemoryHomeserver) authenticate(r *http.Request) (memoryIdentity, bool) {
	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	s.mu.Lock()
	defer s.mu.Unlock()
	identity, ok := s.tokens[token]
	return identity, ok
}

func (s *MemoryHomeserver) signal() {
	close(s.changed)
	s.changed = make(chan struct{})
}

func localpartFromUser(value string) string {
	trimmed := strings.TrimPrefix(strings.TrimSpace(value), "@")
	localpart, _, _ := strings.Cut(trimmed, ":")
	return localpart
}

func containsDevice(devices []id.DeviceID, want id.DeviceID) bool {
	for _, device := range devices {
		if device == want {
			return true
		}
	}
	return false
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		writeMatrixError(w, http.StatusBadRequest, "M_BAD_JSON", err.Error())
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeMatrixError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"errcode": code, "error": message})
}
