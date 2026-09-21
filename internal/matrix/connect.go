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
	log        *slog.Logger
	serverName string
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
		log:        log,
		serverName: serverName(cfg.UserID),
	}
	syncer, ok := cli.Syncer.(*mautrix.DefaultSyncer)
	if !ok {
		return nil, fmt.Errorf("unexpected syncer %T", cli.Syncer)
	}
	syncer.OnEventType(event.EventMessage, client.captureMessage)

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
	var drained []relay.RoomEvent
	for {
		select {
		case event := <-c.events:
			drained = append(drained, event)
		default:
			return drained, nil
		}
	}
}

// SendText posts an encrypted chat message. A non-empty thread anchor makes it
// a reply in that message's thread.
func (c *Client) SendText(ctx context.Context, roomID, body, threadAnchor string) (string, error) {
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
	select {
	case c.events <- seen:
	default:
		c.log.Error("chat message buffer is full, dropping message", "event", seen.EventID)
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
