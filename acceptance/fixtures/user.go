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
	Encrypted  bool
}

// DeviceKey is one Matrix device of a user, as another client sees it. The
// identity key is the fingerprint the operator's phone shows for the device.
type DeviceKey struct {
	DeviceID   string
	Ed25519    string
	Curve25519 string
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
		Encrypted: evt.Mautrix.WasEncrypted,
	}
	if rel := content.RelatesTo; rel != nil && rel.Type == event.RelThread {
		message.ThreadRoot = rel.EventID.String()
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
