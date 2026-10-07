package fixtures

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/cryptohelper"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"github.com/rs/zerolog"
)

// Message is one message a fixture client has seen, after decryption.
type Message struct {
	RoomID     string
	EventID    string
	Sender     string
	Body       string
	ThreadRoot string
	// ReplyTo is the message this one answers, empty when it answers none.
	ReplyTo string
	// MsgType is how the message was sent, which is what says whether clients
	// would notify the operator about it: "m.text" yes, "m.notice" no.
	MsgType   string
	Encrypted bool
}

// DeviceKey is one Matrix device of a user, as another client sees it. The
// identity key is the fingerprint the operator's phone shows for the device.
type DeviceKey struct {
	DeviceID   string
	Ed25519    string
	Curve25519 string
}

// Reaction is one reaction a fixture client has seen, with the message it
// annotates.
type Reaction struct {
	RoomID        string
	EventID       string
	Sender        string
	TargetEventID string
	Key           string
}

// User is a Matrix user the tests drive: the operator standing in for the
// phone, or a stranger who must never reach a forge.
type User struct {
	UserID      string
	Localpart   string
	AccessToken string
	DeviceID    string
	Dir         string

	cli    *mautrix.Client
	helper *cryptohelper.CryptoHelper
	log    *slog.Logger

	mu          sync.Mutex
	messages    []Message
	reactions   []Reaction
	joined      []string
	invites     []string
	everInvited map[string]bool
}

// NewUser registers a fixture user on the homeserver, logs in, and starts
// syncing and decrypting.
func NewUser(ctx context.Context, hs *Synapse, localpart string, dir string) (*User, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	password := "fixture-" + localpart
	userID, token, deviceID, err := hs.Register(ctx, localpart, password)
	if err != nil {
		return nil, err
	}

	cli, err := mautrix.NewClient(hs.URL, id.UserID(userID), token)
	if err != nil {
		return nil, err
	}
	cli.DeviceID = id.DeviceID(deviceID)
	cli.Log = zerologFor(localpart)

	user := &User{
		UserID:      userID,
		Localpart:   localpart,
		AccessToken: token,
		DeviceID:    deviceID,
		Dir:         dir,
		cli:         cli,
		log:         slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
		everInvited: map[string]bool{},
	}
	syncer, ok := cli.Syncer.(*mautrix.DefaultSyncer)
	if !ok {
		return nil, fmt.Errorf("unexpected syncer %T", cli.Syncer)
	}
	syncer.OnEventType(event.EventMessage, user.captureMessage)
	syncer.OnEventType(event.EventReaction, user.captureReaction)
	syncer.OnEventType(event.StateMember, user.captureMembership)

	helper, err := cryptohelper.NewCryptoHelper(cli, pickleKey(userID), filepath.Join(dir, "crypto.db"))
	if err != nil {
		return nil, err
	}
	if err := helper.Init(ctx); err != nil {
		return nil, fmt.Errorf("start crypto for %s: %w", localpart, err)
	}
	helper.DecryptErrorCallback = func(evt *event.Event, err error) {
		user.log.Warn("could not decrypt a room event", "user", userID, "event", evt.ID, "room", evt.RoomID, "error", err)
	}
	user.helper = helper

	// The client's phone stays on for the whole scenario, so its syncer runs
	// on its own context rather than the step that created it.
	go func() {
		if err := cli.SyncWithContext(context.Background()); err != nil {
			user.log.Error("sync stopped", "user", userID, "error", err)
		}
	}()
	return user, nil
}

// Close stops syncing and closes the crypto store.
func (u *User) Close() error {
	u.cli.StopSync()
	if u.helper == nil {
		return nil
	}
	return u.helper.Close()
}

// Client exposes the underlying client for state queries.
func (u *User) Client() *mautrix.Client {
	return u.cli
}

// Send posts an encrypted message into a room.
func (u *User) Send(ctx context.Context, roomID, body string) (string, error) {
	content := &event.MessageEventContent{MsgType: event.MsgText, Body: body}
	encrypted, err := u.helper.Encrypt(ctx, id.RoomID(roomID), event.EventMessage, content)
	if err != nil {
		return "", err
	}
	resp, err := u.cli.SendMessageEvent(ctx, id.RoomID(roomID), event.EventEncrypted, encrypted)
	if err != nil {
		return "", err
	}
	// The phone shows its own message straight away, whether or not the
	// homeserver echoes it back in a readable shape.
	u.mu.Lock()
	u.messages = append(u.messages, Message{
		RoomID:    roomID,
		EventID:   resp.EventID.String(),
		Sender:    u.UserID,
		Body:      body,
		Encrypted: true,
	})
	u.mu.Unlock()
	return resp.EventID.String(), nil
}

// React adds a reaction to an event, the way a phone does when the operator
// taps it.
func (u *User) React(ctx context.Context, roomID, targetEventID, key string) error {
	content := &event.ReactionEventContent{
		RelatesTo: event.RelatesTo{
			Type:    event.RelAnnotation,
			EventID: id.EventID(targetEventID),
			Key:     key,
		},
	}
	_, err := u.cli.SendMessageEvent(ctx, id.RoomID(roomID), event.EventReaction, content)
	return err
}

// SendInThread posts an encrypted reply in a message's thread.
func (u *User) SendInThread(ctx context.Context, roomID, body, threadRoot string) (string, error) {
	content := &event.MessageEventContent{
		MsgType: event.MsgText,
		Body:    body,
		RelatesTo: &event.RelatesTo{
			Type:    event.RelThread,
			EventID: id.EventID(threadRoot),
		},
	}
	encrypted, err := u.helper.Encrypt(ctx, id.RoomID(roomID), event.EventMessage, content)
	if err != nil {
		return "", err
	}
	resp, err := u.cli.SendMessageEvent(ctx, id.RoomID(roomID), event.EventEncrypted, encrypted)
	if err != nil {
		return "", err
	}
	u.mu.Lock()
	u.messages = append(u.messages, Message{
		RoomID:     roomID,
		EventID:    resp.EventID.String(),
		Sender:     u.UserID,
		Body:       body,
		ThreadRoot: threadRoot,
		Encrypted:  true,
	})
	u.mu.Unlock()
	return resp.EventID.String(), nil
}

// SwipeReply sends the reply a phone makes by quoting a message: the phone
// writes the quoted message into the body, each of its lines marked with the
// client's quote marker, and carries the message it quotes without carrying a
// thread.
func (u *User) SwipeReply(ctx context.Context, roomID, targetEventID, words string) (string, error) {
	quoted, ok := u.messageByEvent(roomID, targetEventID)
	if !ok {
		return "", fmt.Errorf("%s cannot quote %s: it never saw that message", u.UserID, targetEventID)
	}
	return u.swipeReplyQuoting(ctx, roomID, targetEventID, quoted.Body, words)
}

// SwipeReplyTo sends the reply a phone makes when it swipes an event it did not
// see as a message. The room's state - the approval or clarification fact the
// phone reads and acts on - is such an event, so the quote the phone writes is
// given rather than looked up, and the reply names that event as what it
// answers.
func (u *User) SwipeReplyTo(ctx context.Context, roomID, targetEventID, quotedBody, words string) (string, error) {
	return u.swipeReplyQuoting(ctx, roomID, targetEventID, quotedBody, words)
}

// swipeReplyQuoting is the reply both swipe forms share: the quoted event with
// the client's quote marker, the operator's own words under it, sent as a reply
// that names the event it quotes and recorded as this client sent it.
func (u *User) swipeReplyQuoting(ctx context.Context, roomID, targetEventID, quotedBody, words string) (string, error) {
	body := quoteBack(quotedBody) + "\n\n" + words
	content := &event.MessageEventContent{
		MsgType:   event.MsgText,
		Body:      body,
		RelatesTo: (&event.RelatesTo{}).SetReplyTo(id.EventID(targetEventID)),
	}
	encrypted, err := u.helper.Encrypt(ctx, id.RoomID(roomID), event.EventMessage, content)
	if err != nil {
		return "", err
	}
	resp, err := u.cli.SendMessageEvent(ctx, id.RoomID(roomID), event.EventEncrypted, encrypted)
	if err != nil {
		return "", err
	}
	u.mu.Lock()
	u.messages = append(u.messages, Message{
		RoomID:    roomID,
		EventID:   resp.EventID.String(),
		Sender:    u.UserID,
		Body:      body,
		ReplyTo:   targetEventID,
		Encrypted: true,
	})
	u.mu.Unlock()
	return resp.EventID.String(), nil
}

// quoteBack is the message a phone writes into a swipe-reply's body, the way
// Matrix clients quote it.
func quoteBack(body string) string {
	lines := strings.Split(body, "\n")
	for index, line := range lines {
		lines[index] = "> " + line
	}
	return strings.Join(lines, "\n")
}

// messageByEvent is a message this client has seen in a room.
func (u *User) messageByEvent(roomID, eventID string) (Message, bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	for _, message := range u.messages {
		if message.RoomID == roomID && message.EventID == eventID {
			return message, true
		}
	}
	return Message{}, false
}

// JoinedRoomIDs lists the rooms this user is in.
func (u *User) JoinedRoomIDs(ctx context.Context) ([]string, error) {
	resp, err := u.cli.JoinedRooms(ctx)
	if err != nil {
		return nil, err
	}
	rooms := make([]string, 0, len(resp.JoinedRooms))
	for _, roomID := range resp.JoinedRooms {
		rooms = append(rooms, roomID.String())
	}
	return rooms, nil
}

// Messages returns the messages seen in a room, oldest first.
func (u *User) Messages(roomID string) []Message {
	u.mu.Lock()
	defer u.mu.Unlock()
	var seen []Message
	for _, message := range u.messages {
		if message.RoomID == roomID {
			seen = append(seen, message)
		}
	}
	return seen
}

// Reactions returns the reactions this client has seen on a message, oldest
// first.
func (u *User) Reactions(roomID, targetEventID string) []Reaction {
	u.mu.Lock()
	defer u.mu.Unlock()
	var seen []Reaction
	for _, reaction := range u.reactions {
		if reaction.RoomID == roomID && reaction.TargetEventID == targetEventID {
			seen = append(seen, reaction)
		}
	}
	return seen
}

// Invites returns the rooms this user has been invited to and not yet joined.
func (u *User) Invites() []string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]string(nil), u.invites...)
}

// WaitForMessage waits for a message with the given body in a room.
func (u *User) WaitForMessage(ctx context.Context, roomID, body string, timeout time.Duration) (Message, error) {
	deadline := time.Now().Add(timeout)
	for {
		for _, message := range u.Messages(roomID) {
			if message.Body == body {
				return message, nil
			}
		}
		if time.Now().After(deadline) {
			return Message{}, fmt.Errorf("%s never saw the message %q in %s", u.UserID, body, roomID)
		}
		if err := Sleep(ctx, 100*time.Millisecond); err != nil {
			return Message{}, err
		}
	}
}

// WaitForThreadReply waits for a thread reply under a chat message.
func (u *User) WaitForThreadReply(ctx context.Context, roomID, anchorEventID, body string, timeout time.Duration) (Message, error) {
	deadline := time.Now().Add(timeout)
	for {
		for _, message := range u.Messages(roomID) {
			if message.Body == body && message.ThreadRoot == anchorEventID {
				return message, nil
			}
		}
		if time.Now().After(deadline) {
			return Message{}, fmt.Errorf("%s never saw the thread reply %q under %s", u.UserID, body, anchorEventID)
		}
		if err := Sleep(ctx, 100*time.Millisecond); err != nil {
			return Message{}, err
		}
	}
}

// WaitForInvite waits until this user has been invited to a room.
func (u *User) WaitForInvite(ctx context.Context, roomID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		for _, invited := range u.Invites() {
			if invited == roomID {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s was never invited to %s", u.UserID, roomID)
		}
		if err := Sleep(ctx, 100*time.Millisecond); err != nil {
			return err
		}
	}
}

// JoinInvites accepts the invites this user holds right now. Invites that
// arrive later are accepted by the syncer, the way a phone would.
func (u *User) JoinInvites(ctx context.Context) error {
	for _, roomID := range u.Invites() {
		if _, err := u.cli.JoinRoomByID(ctx, id.RoomID(roomID)); err != nil && !strings.Contains(err.Error(), "already in the room") {
			return err
		}
	}
	return nil
}

// StateEvent returns one of a room's state events: the event's own id and the
// JSON the room holds, which is where a reader finds the facts a face wrote.
// The event is absent when the room carries none under that type and key.
func (u *User) StateEvent(ctx context.Context, roomID, eventType, stateKey string) (string, map[string]any, bool, error) {
	state, err := u.cli.State(ctx, id.RoomID(roomID))
	if err != nil {
		return "", nil, false, err
	}
	evt := state[event.Type{Type: eventType, Class: event.StateEventType}][stateKey]
	if evt == nil {
		return "", nil, false, nil
	}
	return evt.ID.String(), evt.Content.Raw, true, nil
}

// Invite asks a user into a room, the way the operator would add someone.
func (u *User) Invite(ctx context.Context, roomID, userID string) error {
	_, err := u.cli.InviteUser(ctx, id.RoomID(roomID), &mautrix.ReqInviteUser{UserID: id.UserID(userID)})
	return err
}

// SpaceChildren lists the child rooms of a space.
func (u *User) SpaceChildren(ctx context.Context, spaceID string) ([]string, error) {
	state, err := u.cli.State(ctx, id.RoomID(spaceID))
	if err != nil {
		return nil, err
	}
	var children []string
	for childID := range state[event.StateSpaceChild] {
		children = append(children, childID)
	}
	return children, nil
}

// RoomName reads a room's name.
func (u *User) RoomName(ctx context.Context, roomID string) (string, error) {
	var content event.RoomNameEventContent
	if err := u.cli.StateEvent(ctx, id.RoomID(roomID), event.StateRoomName, "", &content); err != nil {
		return "", err
	}
	return content.Name, nil
}

// RoomTypes reports whether a room is a space.
func (u *User) IsSpace(ctx context.Context, roomID string) (bool, error) {
	var content event.CreateEventContent
	if err := u.cli.StateEvent(ctx, id.RoomID(roomID), event.StateCreate, "", &content); err != nil {
		return false, err
	}
	return content.Type == event.RoomTypeSpace, nil
}

// EncryptionAlgorithm reads a room's encryption algorithm, empty when the room
// is unencrypted.
func (u *User) EncryptionAlgorithm(ctx context.Context, roomID string) (string, error) {
	var content event.EncryptionEventContent
	if err := u.cli.StateEvent(ctx, id.RoomID(roomID), event.StateEncryption, "", &content); err != nil {
		return "", err
	}
	return string(content.Algorithm), nil
}

// Membership reads a user's membership in a room.
func (u *User) Membership(ctx context.Context, roomID, userID string) (string, error) {
	var content event.MemberEventContent
	if err := u.cli.StateEvent(ctx, id.RoomID(roomID), event.StateMember, userID, &content); err != nil {
		return "", err
	}
	return string(content.Membership), nil
}

func (u *User) captureMessage(_ context.Context, evt *event.Event) {
	content := evt.Content.AsMessage()
	if content == nil || content.Body == "" {
		return
	}
	message := Message{
		RoomID:    evt.RoomID.String(),
		EventID:   evt.ID.String(),
		Sender:    evt.Sender.String(),
		Body:      content.Body,
		MsgType:   string(content.MsgType),
		Encrypted: evt.Mautrix.WasEncrypted,
	}
	if rel := content.RelatesTo; rel != nil && rel.Type == event.RelThread {
		message.ThreadRoot = rel.EventID.String()
	}
	if rel := content.RelatesTo; rel != nil {
		message.ReplyTo = rel.GetNonFallbackReplyTo().String()
	}

	u.mu.Lock()
	defer u.mu.Unlock()
	for _, seen := range u.messages {
		if seen.EventID == message.EventID {
			return
		}
	}
	u.messages = append(u.messages, message)
}

func (u *User) captureMembership(_ context.Context, evt *event.Event) {
	if evt.StateKey == nil || *evt.StateKey != u.UserID {
		return
	}
	content := evt.Content.AsMember()
	if content == nil {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	switch content.Membership {
	case event.MembershipInvite:
		u.everInvited[evt.RoomID.String()] = true
		if !Contains(u.invites, evt.RoomID.String()) && !Contains(u.joined, evt.RoomID.String()) {
			u.invites = append(u.invites, evt.RoomID.String())
		}
		roomID := evt.RoomID
		go func() {
			if _, err := u.cli.JoinRoomByID(context.Background(), roomID); err != nil {
				u.log.Error("could not accept invite", "user", u.UserID, "room", roomID, "error", err)
			}
		}()
	case event.MembershipJoin:
		u.invites = remove(u.invites, evt.RoomID.String())
		if !Contains(u.joined, evt.RoomID.String()) {
			u.joined = append(u.joined, evt.RoomID.String())
		}
	}
}

func (u *User) captureReaction(_ context.Context, evt *event.Event) {
	content := evt.Content.AsReaction()
	if content == nil || content.RelatesTo.EventID == "" {
		return
	}
	reaction := Reaction{
		RoomID:        evt.RoomID.String(),
		EventID:       evt.ID.String(),
		Sender:        evt.Sender.String(),
		TargetEventID: content.RelatesTo.EventID.String(),
		Key:           content.RelatesTo.Key,
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	for _, seen := range u.reactions {
		if seen.EventID == reaction.EventID {
			return
		}
	}
	u.reactions = append(u.reactions, reaction)
}

// Rooms lists the rooms this user has: joined rooms first, then open invites.
func (u *User) Rooms(ctx context.Context) ([]string, error) {
	joined, err := u.JoinedRoomIDs(ctx)
	if err != nil {
		return nil, err
	}
	rooms := append([]string(nil), joined...)
	for _, invited := range u.Invites() {
		if !Contains(rooms, invited) {
			rooms = append(rooms, invited)
		}
	}
	return rooms, nil
}

// DisplayName is the name a user shows up under in a room, which is what the
// phone shows as the sender of their messages.
func (u *User) DisplayName(ctx context.Context, roomID, userID string) (string, error) {
	var content event.MemberEventContent
	if err := u.cli.StateEvent(ctx, id.RoomID(roomID), event.StateMember, userID, &content); err != nil {
		return "", err
	}
	return content.Displayname, nil
}

// Devices lists the devices a user has published, seen from this client.
func (u *User) Devices(ctx context.Context, userID string) ([]DeviceKey, error) {
	resp, err := u.cli.QueryKeys(ctx, &mautrix.ReqQueryKeys{
		DeviceKeys: mautrix.DeviceKeysRequest{id.UserID(userID): mautrix.DeviceIDList{}},
	})
	if err != nil {
		return nil, err
	}
	var devices []DeviceKey
	for deviceID, keys := range resp.DeviceKeys[id.UserID(userID)] {
		devices = append(devices, DeviceKey{
			DeviceID:   deviceID.String(),
			Ed25519:    keys.Keys.GetEd25519(deviceID).String(),
			Curve25519: keys.Keys.GetCurve25519(deviceID).String(),
		})
	}
	sort.Slice(devices, func(i, j int) bool { return devices[i].DeviceID < devices[j].DeviceID })
	return devices, nil
}

// EverInvited reports whether this user was ever invited to a room.
func (u *User) EverInvited(roomID string) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.everInvited[roomID]
}

func pickleKey(userID string) []byte {
	sum := sha256.Sum256([]byte("forgelet-bridge acceptance pickle key:" + userID))
	return sum[:]
}

func zerologFor(component string) zerolog.Logger {
	return zerolog.New(os.Stderr).
		Level(zerolog.WarnLevel).
		With().
		Str("component", "acceptance-"+component).
		Logger()
}

// Sleep waits for d, or for the context to end, whichever comes first.
func Sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Contains reports whether a list of room ids holds one room.
func Contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func remove(list []string, value string) []string {
	kept := list[:0]
	for _, item := range list {
		if item != value {
			kept = append(kept, item)
		}
	}
	return kept
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-10-07T01:35:55+02:00","module_hash":"5f99b34a02aae4401964559b3c2785224ff117f7de7e6369626e9a379d642a11","functions":[{"id":"func/NewUser","name":"NewUser","line":79,"end_line":134,"hash":"7ea268338fde46a82eb7eb4de938f925b516f371543afd87ecbcdc27ba4d21e5"},{"id":"func/User.Close","name":"User.Close","line":137,"end_line":143,"hash":"6eb3cb38d1070e9c1e3b0805386c878ed8c7dc275d8312ea06f12fcd91bc0485"},{"id":"func/User.Client","name":"User.Client","line":146,"end_line":148,"hash":"5e4dfa2c17b6c8d4bc253f1c8637400669508b85ac78cc695a094d5e775337c7"},{"id":"func/User.Send","name":"User.Send","line":151,"end_line":173,"hash":"7393edb71cc28fb2aa101490d90885b4d93d84e651031c77f9c560259910ccc9"},{"id":"func/User.React","name":"User.React","line":177,"end_line":187,"hash":"2f3164a5db90ca4c9d79e01d4c5a65b87cdab98adf3a6408260dbde6a045b5e2"},{"id":"func/User.SendInThread","name":"User.SendInThread","line":190,"end_line":218,"hash":"76018dd7fd5c7fe48f1953480769d33b675d8e846758baacc6c3bed6f7ee758c"},{"id":"func/User.SwipeReply","name":"User.SwipeReply","line":224,"end_line":230,"hash":"562e9c1d9d71bf98584b6a8d28d5b3b53afe51e8b4b9506e9c1eb92793b93bfb"},{"id":"func/User.SwipeReplyTo","name":"User.SwipeReplyTo","line":237,"end_line":239,"hash":"b4ce3f4fb4bc0aee8443e1264c239c106160051f5fa6da7a63c6fd186b0e801b"},{"id":"func/User.swipeReplyQuoting","name":"User.swipeReplyQuoting","line":244,"end_line":270,"hash":"bfd79a60e02a7b19e625b37d8e89c541002e43111bdaa48426ac672a25536f7e"},{"id":"func/quoteBack","name":"quoteBack","line":274,"end_line":280,"hash":"4100d65c0e50fb765dd57de5e9f5262dad2936321316a39e957e444d5349704b"},{"id":"func/User.messageByEvent","name":"User.messageByEvent","line":283,"end_line":292,"hash":"0beac9866b59865084d2ec228c5eefd096193bb90e99d47dd1b8cd002a11f27d"},{"id":"func/User.JoinedRoomIDs","name":"User.JoinedRoomIDs","line":295,"end_line":305,"hash":"2511fde6d29aefae34f9c8ca39dbe418bf7115a733e11622a243828789ee3fee"},{"id":"func/User.Messages","name":"User.Messages","line":308,"end_line":318,"hash":"4e4f32a74fff15b0c630c7883035d46c383e92dea107fba411fee1aeb17dfee0"},{"id":"func/User.Reactions","name":"User.Reactions","line":322,"end_line":332,"hash":"76c501e36d4aeaafd2fc6206076f4e8d30a490761fabaaabf5a5f9e8d5bd3332"},{"id":"func/User.Invites","name":"User.Invites","line":335,"end_line":339,"hash":"29939967063452e6fcbed757b45580b5339c32f83c86b585078945fa78b80cd3"},{"id":"func/User.WaitForMessage","name":"User.WaitForMessage","line":342,"end_line":357,"hash":"5007302230f1a1e47e2ed8437d9edf760ed9540c005de5cb6605ea8ed93881c3"},{"id":"func/User.WaitForThreadReply","name":"User.WaitForThreadReply","line":360,"end_line":375,"hash":"85f7ed1fbf9983d38780c7b1a3f55bed0ec72cbd1f28740792b0f89602dc3fb1"},{"id":"func/User.WaitForInvite","name":"User.WaitForInvite","line":378,"end_line":393,"hash":"535732a9ac9dbb8438c8161068fad4a42d71953168ef6a58a092814007b9c8f0"},{"id":"func/User.JoinInvites","name":"User.JoinInvites","line":397,"end_line":404,"hash":"7ce0d2f1822cc2b796f2d20904a77494c2288c449fe0e7bb77d300326b43a075"},{"id":"func/User.StateEvent","name":"User.StateEvent","line":409,"end_line":419,"hash":"0bd9d41e6a1400f4dbb865bef8cd7ada555c41eca844621b0718f1077e39b27f"},{"id":"func/User.Invite","name":"User.Invite","line":422,"end_line":425,"hash":"7391d1fcc83c02b32c5c7683c613f12023c99ed6eeb34420cc9d37befca53b45"},{"id":"func/User.SpaceChildren","name":"User.SpaceChildren","line":428,"end_line":438,"hash":"a87e8b0197e4679139bff77f8983728d4889dde2ce479ffed2125a80a6aff2e8"},{"id":"func/User.RoomName","name":"User.RoomName","line":441,"end_line":447,"hash":"075bd7e2afe865686278772f042abd04e204268107c07e4deafc7cb4ae3b128b"},{"id":"func/User.IsSpace","name":"User.IsSpace","line":450,"end_line":456,"hash":"e9160f7fc294e0025536d686a4f4f672186444cbc53865af88cc7e03e573f82c"},{"id":"func/User.EncryptionAlgorithm","name":"User.EncryptionAlgorithm","line":460,"end_line":466,"hash":"7ce889801629e864824181c5d1434a903624e05df9035ddbc4f7ad389d08cb3d"},{"id":"func/User.Membership","name":"User.Membership","line":469,"end_line":475,"hash":"65f83783dee5be4ee8a71cfb4c580496a8eb38ad72841ec8f585e70070db4ca7"},{"id":"func/User.captureMessage","name":"User.captureMessage","line":477,"end_line":505,"hash":"61a2108a82dc10f6d785ac94e53b572a5b265212493f77937714bb8bd991afda"},{"id":"func/User.captureMembership","name":"User.captureMembership","line":507,"end_line":535,"hash":"3002cd0a2edad2d776dbf405eb87d5f6c5ca524ad41e8a6c83a23ab6c0391604"},{"id":"func/User.captureReaction","name":"User.captureReaction","line":537,"end_line":557,"hash":"f5063c3117747e93ead4e5dfa8c4449eb17f7460d655f4b259b422aa763deb77"},{"id":"func/User.Rooms","name":"User.Rooms","line":560,"end_line":572,"hash":"f38aca72a49b785f45c0bbf719385c7c0013e406e876241798f1966604a5f037"},{"id":"func/User.DisplayName","name":"User.DisplayName","line":576,"end_line":582,"hash":"fd6e672b573665e730255405ea1a7bfcf0b7f7f10ef4d1be58e2b6eb35047c20"},{"id":"func/User.Devices","name":"User.Devices","line":585,"end_line":602,"hash":"1b9758e0750407e1a3afe2c87c56d1f5c2afd13d0a0c323c43295bcab2d90b2c"},{"id":"func/User.EverInvited","name":"User.EverInvited","line":605,"end_line":609,"hash":"71a597f6c7d61d2e12fc4536ec2f6e167b05c831cdf2bf2629d44ef4eeac396d"},{"id":"func/pickleKey","name":"pickleKey","line":611,"end_line":614,"hash":"5637211494e7d43cd4a1bd624793734ed23ed03406d9dc544bb3fddd4f86267a"},{"id":"func/zerologFor","name":"zerologFor","line":616,"end_line":622,"hash":"66a6cf397bd61bd22145b4cebba1115181f2de073151178ba361d5a586961234"},{"id":"func/Sleep","name":"Sleep","line":625,"end_line":634,"hash":"a12f07bb5bcf8de79ddaf54634400059f7618129370bb4cc937bcc41ee1fc2dd"},{"id":"func/Contains","name":"Contains","line":637,"end_line":644,"hash":"6b0b8bdf14ac6244234a3a1fc80bb41e6253d47800575b875e015d0798c0966c"},{"id":"func/remove","name":"remove","line":646,"end_line":654,"hash":"e545834c7ad7f87ee590391433deac3f051f90e905fe7c59f9e4aad3a7606654"}]}
// mutate4go-manifest-end
