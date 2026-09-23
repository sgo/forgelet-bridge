// Package matrix adapts mautrix-go to what the bridge needs from Matrix: a
// logged-in, end-to-end encrypted client that creates forge spaces and relays
// chat messages.
package matrix

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/cryptohelper"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"github.com/unclebob/forgelet-bridge/internal/config"
	"github.com/unclebob/forgelet-bridge/internal/relay"
)

// DefaultDeviceID names the bridge's Matrix device.
const DefaultDeviceID = "FORGELETBRIDGE"

const eventBuffer = 4096

// Client is a connected bridge device.
type Client struct {
	cli        *mautrix.Client
	helper     *cryptohelper.CryptoHelper
	events     chan relay.RoomEvent
	reactions  chan relay.Reaction
	log        *slog.Logger
	serverName string
	// operator is the one whose phone has to be able to read what the bridge
	// posts, in every room it posts in.
	operator id.UserID
}

// Connect logs the bridge in, opens its crypto store, and starts syncing.
// Reconnecting with the same state directory reuses the same Matrix device.
func Connect(ctx context.Context, cfg config.Config, log *slog.Logger) (*Client, error) {
	if log == nil {
		log = slog.Default()
	}
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		return nil, fmt.Errorf("create state directory: %w", err)
	}

	cli, err := mautrix.NewClient(cfg.HomeserverURL, id.UserID(cfg.UserID), cfg.AccessToken)
	if err != nil {
		return nil, err
	}
	cli.Log = zerolog.New(os.Stderr).Level(zerolog.WarnLevel).With().Str("component", "matrix").Logger()

	client := &Client{
		cli:        cli,
		events:     make(chan relay.RoomEvent, eventBuffer),
		reactions:  make(chan relay.Reaction, eventBuffer),
		log:        log,
		serverName: serverName(cfg.UserID),
		operator:   id.UserID(cfg.Operator),
	}
	syncer, ok := cli.Syncer.(*mautrix.DefaultSyncer)
	if !ok {
		return nil, fmt.Errorf("unexpected syncer %T", cli.Syncer)
	}
	syncer.OnEventType(event.EventMessage, client.captureMessage)
	syncer.OnEventType(event.EventReaction, client.captureReaction)

	helper, err := cryptohelper.NewCryptoHelper(cli, pickleKey(cfg.UserID), filepath.Join(cfg.StateDir, "crypto.db"))
	if err != nil {
		return nil, err
	}
	if cfg.Password != "" {
		helper.LoginAs = &mautrix.ReqLogin{
			Type: mautrix.AuthTypePassword,
			Identifier: mautrix.UserIdentifier{
				Type: mautrix.IdentifierTypeUser,
				User: cfg.UserID,
			},
			Password:                 cfg.Password,
			DeviceID:                 deviceID(cfg),
			InitialDeviceDisplayName: "forgelet-bridge",
			StoreCredentials:         true,
		}
	}
	if err := helper.Init(ctx); err != nil {
		return nil, fmt.Errorf("start matrix crypto: %w", err)
	}
	client.helper = helper

	go func() {
		if err := cli.SyncWithContext(ctx); err != nil && ctx.Err() == nil {
			log.Error("matrix sync stopped", "error", err)
		}
	}()
	log.Info("connected to matrix", "user", cfg.UserID, "device", cli.DeviceID, "homeserver", cfg.HomeserverURL)
	return client, nil
}

// Close stops the crypto store and the syncer.
func (c *Client) Close() error {
	c.cli.StopSync()
	if c.helper == nil {
		return nil
	}
	return c.helper.Close()
}

// DrainEvents returns the chat messages seen since the last drain.
func (c *Client) DrainEvents(_ context.Context) ([]relay.RoomEvent, error) {
	return drain(c.events), nil
}

// DrainReactions returns the reactions seen since the last drain.
func (c *Client) DrainReactions(_ context.Context) ([]relay.Reaction, error) {
	return drain(c.reactions), nil
}

// drain takes everything a channel holds right now.
func drain[T any](from chan T) []T {
	var drained []T
	for {
		select {
		case item := <-from:
			drained = append(drained, item)
		default:
			return drained
		}
	}
}

// DeviceIdentity is the Matrix device the bridge is using: its device id and
// the ed25519 fingerprint the operator's phone shows for it. The fingerprint
// comes from the bridge's own crypto store, so a restart that loses it is
// visible instead of silent.
func (c *Client) DeviceIdentity() (deviceID, fingerprint string) {
	return c.cli.DeviceID.String(), c.helper.Machine().GetAccount().SigningKey().String()
}

// SendText posts an encrypted chat message. A non-empty thread anchor makes it
// a reply in that message's thread.
func (c *Client) SendText(ctx context.Context, roomID, body, threadAnchor string) (string, error) {
	// What the bridge posts, the operator has to be able to read - the first
	// message in a room included. The bridge's own view of a room it has just
	// created can lag the invite it sent, so the operator's devices are named
	// here rather than left to that view.
	if err := c.shareWithOperator(ctx, id.RoomID(roomID)); err != nil {
		return "", err
	}
	content := &event.MessageEventContent{
		MsgType: event.MsgText,
		Body:    body,
	}
	if threadAnchor != "" {
		content.RelatesTo = &event.RelatesTo{
			Type:    event.RelThread,
			EventID: id.EventID(threadAnchor),
		}
	}

	encrypted, err := c.helper.Encrypt(ctx, id.RoomID(roomID), event.EventMessage, content)
	if err != nil {
		return "", fmt.Errorf("encrypt chat message: %w", err)
	}
	resp, err := c.cli.SendMessageEvent(ctx, id.RoomID(roomID), event.EventEncrypted, encrypted)
	if err != nil {
		return "", fmt.Errorf("send chat message: %w", err)
	}
	return resp.EventID.String(), nil
}

// shareWithOperator shares the room's group session with the operator's
// devices. It does nothing when the session is already shared, so it is the
// first message in a room that pays for it.
func (c *Client) shareWithOperator(ctx context.Context, roomID id.RoomID) error {
	if c.operator == "" {
		return nil
	}
	if err := c.helper.Machine().ShareGroupSession(ctx, roomID, []id.UserID{c.operator}); err != nil {
		return fmt.Errorf("share the session in %s with %s: %w", roomID, c.operator, err)
	}
	return nil
}

func (c *Client) captureMessage(_ context.Context, evt *event.Event) {
	if evt.RoomID == "" || evt.ID == "" {
		return
	}
	content := evt.Content.AsMessage()
	if content == nil || content.Body == "" {
		return
	}
	seen := relay.RoomEvent{
		RoomID:  evt.RoomID.String(),
		EventID: evt.ID.String(),
		Sender:  evt.Sender.String(),
		Body:    content.Body,
	}
	if rel := content.RelatesTo; rel != nil && rel.Type == event.RelThread {
		seen.ThreadRoot = rel.EventID.String()
	}
	// A phone replies by quoting the message, which carries the message it
	// answers without carrying a thread. A thread reply's own fallback quote is
	// not that, so it is left out.
	if rel := content.RelatesTo; rel != nil {
		seen.ReplyTo = rel.GetNonFallbackReplyTo().String()
	}
	select {
	case c.events <- seen:
	default:
		c.log.Error("chat message buffer is full, dropping message", "event", seen.EventID)
	}
}

// captureReaction records a reaction and the event it annotates, so the bridge
// can tell an approval tap apart from any other reaction.
func (c *Client) captureReaction(_ context.Context, evt *event.Event) {
	content := evt.Content.AsReaction()
	if content == nil || content.RelatesTo.EventID == "" {
		return
	}
	seen := relay.Reaction{
		RoomID:        evt.RoomID.String(),
		EventID:       evt.ID.String(),
		Sender:        evt.Sender.String(),
		TargetEventID: content.RelatesTo.EventID.String(),
		Key:           content.RelatesTo.Key,
	}
	// Say what arrived, so a reaction the bridge cannot read is a fact in the log
	// rather than silence at the operator's end.
	c.log.Info("reaction seen", "sender", seen.Sender, "key", seen.Key, "target", seen.TargetEventID)
	select {
	case c.reactions <- seen:
	default:
		c.log.Error("reaction buffer is full, dropping reaction", "event", seen.EventID)
	}
}

// pickleKey is the passphrase of the bridge's crypto store. It has to be the
// same on every start or the store cannot be read again.
func pickleKey(userID string) []byte {
	sum := sha256.Sum256([]byte("forgelet-bridge pickle key:" + userID))
	return sum[:]
}

func deviceID(cfg config.Config) id.DeviceID {
	if cfg.DeviceID != "" {
		return id.DeviceID(cfg.DeviceID)
	}
	return DefaultDeviceID
}

// serverName is the homeserver name a user id belongs to.
func serverName(userID string) string {
	_, domain, found := strings.Cut(userID, ":")
	if !found {
		return ""
	}
	return domain
}
