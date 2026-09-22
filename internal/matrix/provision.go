package matrix

import (
	"context"
	"errors"
	"fmt"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"github.com/unclebob/forgelet-bridge/internal/bridge"
	"github.com/unclebob/forgelet-bridge/internal/config"
)

// EnsureForge finds or creates the forge's space and the chat room inside it.
// Encryption is on from room creation, the operator is invited to both, and
// the bridge posts in the chat room under the forge's own name.
func (c *Client) EnsureForge(ctx context.Context, forgeName, operator string) (bridge.Room, error) {
	spaceID, err := c.findSpace(ctx, forgeName)
	if err != nil {
		return bridge.Room{}, err
	}
	if spaceID == "" {
		spaceID, err = c.createSpace(ctx, forgeName, operator)
		if err != nil {
			return bridge.Room{}, err
		}
		c.log.Info("created forge space", "forge", forgeName, "space", spaceID)
	}
	if err := c.ensureInvited(ctx, spaceID, operator); err != nil {
		return bridge.Room{}, err
	}

	roomID, err := c.findChatRoom(ctx, spaceID)
	if err != nil {
		return bridge.Room{}, err
	}
	if roomID == "" {
		roomID, err = c.createChatRoom(ctx, spaceID, operator)
		if err != nil {
			return bridge.Room{}, err
		}
		c.log.Info("created chat room", "forge", forgeName, "room", roomID)
	}
	if err := c.ensureInvited(ctx, roomID, operator); err != nil {
		return bridge.Room{}, err
	}
	if err := c.ensureDisplayName(ctx, roomID, forgeName); err != nil {
		return bridge.Room{}, err
	}
	return bridge.Room{SpaceID: spaceID, RoomID: roomID}, nil
}

// ensureDisplayName makes the bridge post in a room under the forge's name, so
// the operator can tell one forge's messages from another's on their phone.
func (c *Client) ensureDisplayName(ctx context.Context, roomID, displayName string) error {
	member, err := c.membership(ctx, roomID, c.cli.UserID.String())
	if err != nil {
		return err
	}
	if member == event.MembershipJoin {
		var current event.MemberEventContent
		if err := c.cli.StateEvent(ctx, id.RoomID(roomID), event.StateMember, c.cli.UserID.String(), &current); err == nil &&
			current.Displayname == displayName {
			return nil
		}
	}
	_, err = c.cli.SendStateEvent(ctx, id.RoomID(roomID), event.StateMember, c.cli.UserID.String(), &event.MemberEventContent{
		Membership:  event.MembershipJoin,
		Displayname: displayName,
	})
	if err != nil {
		return fmt.Errorf("name the bridge %s in %s: %w", displayName, roomID, err)
	}
	return nil
}

func (c *Client) findSpace(ctx context.Context, forgeName string) (string, error) {
	joined, err := c.cli.JoinedRooms(ctx)
	if err != nil {
		return "", fmt.Errorf("list joined rooms: %w", err)
	}
	for _, roomID := range joined.JoinedRooms {
		isSpace, err := c.isSpace(ctx, roomID)
		if err != nil {
			return "", err
		}
		if !isSpace {
			continue
		}
		name, err := c.roomName(ctx, roomID)
		if err != nil {
			return "", err
		}
		if name == forgeName {
			return roomID.String(), nil
		}
	}
	return "", nil
}

func (c *Client) findChatRoom(ctx context.Context, spaceID string) (string, error) {
	state, err := c.cli.State(ctx, id.RoomID(spaceID))
	if err != nil {
		return "", fmt.Errorf("read forge space state: %w", err)
	}
	for childID := range state[event.StateSpaceChild] {
		name, err := c.roomName(ctx, id.RoomID(childID))
		if err != nil {
			continue
		}
		if name == config.RoomName {
			return childID, nil
		}
	}
	return "", nil
}

func (c *Client) createSpace(ctx context.Context, forgeName, operator string) (string, error) {
	resp, err := c.cli.CreateRoom(ctx, &mautrix.ReqCreateRoom{
		Name:            forgeName,
		Preset:          "private_chat",
		Invite:          []id.UserID{id.UserID(operator)},
		CreationContent: map[string]interface{}{"type": "m.space"},
	})
	if err != nil {
		return "", fmt.Errorf("create forge space: %w", err)
	}
	return resp.RoomID.String(), nil
}

func (c *Client) createChatRoom(ctx context.Context, spaceID, operator string) (string, error) {
	resp, err := c.cli.CreateRoom(ctx, &mautrix.ReqCreateRoom{
		Name:   config.RoomName,
		Preset: "private_chat",
		Invite: []id.UserID{id.UserID(operator)},
		InitialState: []*event.Event{{
			Type: event.StateEncryption,
			Content: event.Content{Parsed: &event.EncryptionEventContent{
				Algorithm: id.AlgorithmMegolmV1,
			}},
		}},
	})
	if err != nil {
		return "", fmt.Errorf("create chat room: %w", err)
	}

	child := map[string]interface{}{"via": []string{c.serverName}}
	if _, err := c.cli.SendStateEvent(ctx, id.RoomID(spaceID), event.StateSpaceChild, resp.RoomID.String(), child); err != nil {
		return "", fmt.Errorf("put chat room in the forge space: %w", err)
	}
	return resp.RoomID.String(), nil
}

// ensureInvited invites the operator if they are not already in the room.
func (c *Client) ensureInvited(ctx context.Context, roomID, operator string) error {
	member, err := c.membership(ctx, roomID, operator)
	if err != nil {
		return err
	}
	if member == event.MembershipJoin || member == event.MembershipInvite {
		return nil
	}
	if _, err := c.cli.InviteUser(ctx, id.RoomID(roomID), &mautrix.ReqInviteUser{UserID: id.UserID(operator)}); err != nil {
		return fmt.Errorf("invite operator to %s: %w", roomID, err)
	}
	return nil
}

func (c *Client) membership(ctx context.Context, roomID, userID string) (event.Membership, error) {
	var content event.MemberEventContent
	if err := c.cli.StateEvent(ctx, id.RoomID(roomID), event.StateMember, userID, &content); err != nil {
		if isMissing(err) {
			return event.MembershipLeave, nil
		}
		return "", fmt.Errorf("read membership of %s in %s: %w", userID, roomID, err)
	}
	return content.Membership, nil
}

func (c *Client) isSpace(ctx context.Context, roomID id.RoomID) (bool, error) {
	var create event.CreateEventContent
	if err := c.cli.StateEvent(ctx, roomID, event.StateCreate, "", &create); err != nil {
		return false, fmt.Errorf("read room type of %s: %w", roomID, err)
	}
	return create.Type == event.RoomTypeSpace, nil
}

func (c *Client) roomName(ctx context.Context, roomID id.RoomID) (string, error) {
	var name event.RoomNameEventContent
	if err := c.cli.StateEvent(ctx, roomID, event.StateRoomName, "", &name); err != nil {
		if isMissing(err) {
			return "", nil
		}
		return "", fmt.Errorf("read room name of %s: %w", roomID, err)
	}
	return name.Name, nil
}

// isMissing reports whether the homeserver answered that a piece of state does
// not exist.
func isMissing(err error) bool {
	var httpErr mautrix.HTTPError
	return errors.As(err, &httpErr) && httpErr.IsStatus(404)
}
