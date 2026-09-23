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

// EnsureForge finds or creates the forge's space and the rooms inside it: the
// chat channel, the approvals room, the activity room and the clarifications
// room. Encryption is on from room creation, the operator is invited to all of
// them, and the bridge posts under the forge's own name.
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

	roomID, err := c.ensureRoom(ctx, spaceID, operator, forgeName, config.RoomName)
	if err != nil {
		return bridge.Room{}, err
	}
	approvalsRoomID, err := c.ensureRoom(ctx, spaceID, operator, forgeName, config.ApprovalsRoomName)
	if err != nil {
		return bridge.Room{}, err
	}
	activityRoomID, err := c.ensureRoom(ctx, spaceID, operator, forgeName, config.ActivityRoomName)
	if err != nil {
		return bridge.Room{}, err
	}
	clarificationsRoomID, err := c.ensureRoom(ctx, spaceID, operator, forgeName, config.ClarificationsRoomName)
	if err != nil {
		return bridge.Room{}, err
	}
	return bridge.Room{
		SpaceID:              spaceID,
		RoomID:               roomID,
		ApprovalsRoomID:      approvalsRoomID,
		ActivityRoomID:       activityRoomID,
		ClarificationsRoomID: clarificationsRoomID,
	}, nil
}

// ensureRoom finds or creates one of the forge's rooms inside its space, makes
// sure the operator is in it, and names the bridge after the forge there.
func (c *Client) ensureRoom(ctx context.Context, spaceID, operator, forgeName, roomName string) (string, error) {
	roomID, err := c.findRoom(ctx, spaceID, roomName)
	if err != nil {
		return "", err
	}
	if roomID == "" {
		roomID, err = c.createRoom(ctx, spaceID, operator, roomName)
		if err != nil {
			return "", err
		}
		c.log.Info("created forge room", "forge", forgeName, "room", roomName, "id", roomID)
	}
	if err := c.applyRoom(ctx, roomID, roomName, forgeName, operator); err != nil {
		return "", err
	}
	return roomID, nil
}

// RefreshForge applies the forge's name, and the operator's membership, to the
// rooms the bridge already has: a restart leaves a forge named the way its
// configuration says it is, whether those rooms are new or already there.
func (c *Client) RefreshForge(ctx context.Context, room bridge.Room, forgeName, operator string) error {
	if err := c.ensureRoomNamed(ctx, room.SpaceID, forgeName); err != nil {
		return err
	}
	if err := c.ensureInvited(ctx, room.SpaceID, operator); err != nil {
		return err
	}
	for _, named := range []struct{ id, name string }{
		{room.RoomID, config.RoomName},
		{room.ApprovalsRoomID, config.ApprovalsRoomName},
		{room.ActivityRoomID, config.ActivityRoomName},
		{room.ClarificationsRoomID, config.ClarificationsRoomName},
	} {
		if named.id == "" {
			continue
		}
		if err := c.applyRoom(ctx, named.id, named.name, forgeName, operator); err != nil {
			return err
		}
	}
	return nil
}

// applyRoom brings a room the bridge already has in step with the forge: the
// name the room carries, the operator's membership, and the name the bridge
// posts under there.
func (c *Client) applyRoom(ctx context.Context, roomID, roomName, forgeName, operator string) error {
	if err := c.ensureRoomNamed(ctx, roomID, roomName); err != nil {
		return err
	}
	if err := c.ensureInvited(ctx, roomID, operator); err != nil {
		return err
	}
	return c.ensureDisplayName(ctx, roomID, forgeName)
}

// ensureRoomNamed applies a room's name when it is not what it should be.
func (c *Client) ensureRoomNamed(ctx context.Context, roomID, name string) error {
	current, err := c.roomName(ctx, id.RoomID(roomID))
	if err != nil {
		return err
	}
	if current == name {
		return nil
	}
	if _, err := c.cli.SendStateEvent(ctx, id.RoomID(roomID), event.StateRoomName, "", &event.RoomNameEventContent{Name: name}); err != nil {
		return fmt.Errorf("name room %s %q: %w", roomID, name, err)
	}
	return nil
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

func (c *Client) findRoom(ctx context.Context, spaceID, roomName string) (string, error) {
	state, err := c.cli.State(ctx, id.RoomID(spaceID))
	if err != nil {
		return "", fmt.Errorf("read forge space state: %w", err)
	}
	for childID := range state[event.StateSpaceChild] {
		name, err := c.roomName(ctx, id.RoomID(childID))
		if err != nil {
			continue
		}
		if name == roomName {
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

func (c *Client) createRoom(ctx context.Context, spaceID, operator, roomName string) (string, error) {
	resp, err := c.cli.CreateRoom(ctx, &mautrix.ReqCreateRoom{
		Name:   roomName,
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
