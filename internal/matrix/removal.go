package matrix

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// forgeFactPrefix is the namespace every relay fact the bridge writes uses.
// Room names, memberships and ordinary Matrix state are deliberately outside
// this cleanup.
const forgeFactPrefix = "com.forgelet."

// ClearFacts clears every relay fact the bridge currently carries in a room.
// Reading the room's state means facts written by an earlier process are found
// as well as those remembered in this process.
func (c *Client) ClearFacts(ctx context.Context, roomID string) error {
	return c.clearFactsMatching(ctx, roomID, func(event.Type, string, *event.Event) bool {
		return true
	})
}

// ClearProjectFacts clears one project's facts without touching a sibling
// project or a forge-level room's own facts. The project room is exclusively
// the project's, the activity room carries one board fact per project, and the
// approvals room carries the project name in each approval fact.
func (c *Client) ClearProjectFacts(ctx context.Context, projectRoomID, activityRoomID, approvalsRoomID, project string) error {
	if err := c.clearFactsMatching(ctx, projectRoomID, func(event.Type, string, *event.Event) bool {
		return true
	}); err != nil {
		return err
	}
	if err := c.clearFactsMatching(ctx, activityRoomID, func(eventType event.Type, stateKey string, _ *event.Event) bool {
		return eventType.Type == "com.forgelet.board" && stateKey == project
	}); err != nil {
		return err
	}
	return c.clearFactsMatching(ctx, approvalsRoomID, func(eventType event.Type, _ string, evt *event.Event) bool {
		return eventType.Type == "com.forgelet.approval" && factProject(evt) == project
	})
}

func (c *Client) clearFactsMatching(
	ctx context.Context,
	roomID string,
	match func(event.Type, string, *event.Event) bool,
) error {
	state, err := c.cli.State(ctx, id.RoomID(roomID))
	if err != nil {
		return fmt.Errorf("read forge facts in %s: %w", roomID, err)
	}

	type fact struct {
		eventType event.Type
		stateKey  string
	}
	var facts []fact
	for eventType, byStateKey := range state {
		if !strings.HasPrefix(eventType.Type, forgeFactPrefix) {
			continue
		}
		for stateKey, evt := range byStateKey {
			if !match(eventType, stateKey, evt) {
				continue
			}
			facts = append(facts, fact{eventType: eventType, stateKey: stateKey})
		}
	}
	sort.Slice(facts, func(i, j int) bool {
		if facts[i].eventType.Type != facts[j].eventType.Type {
			return facts[i].eventType.Type < facts[j].eventType.Type
		}
		return facts[i].stateKey < facts[j].stateKey
	})
	for _, fact := range facts {
		if _, err := c.cli.SendStateEvent(ctx, id.RoomID(roomID), fact.eventType, fact.stateKey, map[string]any{}); err != nil {
			return fmt.Errorf("clear forge fact %s in %s: %w", fact.eventType.Type, roomID, err)
		}
	}
	return nil
}

func factProject(evt *event.Event) string {
	if evt == nil {
		return ""
	}
	raw, err := json.Marshal(evt.Content.Raw)
	if err != nil {
		return ""
	}
	var content struct {
		Project string `json:"project"`
	}
	if json.Unmarshal(raw, &content) != nil {
		return ""
	}
	return content.Project
}

// RemoveOperator removes the operator's current or pending membership from a
// room. A membership that is already gone needs no Matrix call.
func (c *Client) RemoveOperator(ctx context.Context, roomID, operator string) error {
	membership, err := c.membership(ctx, roomID, operator)
	if err != nil {
		return err
	}
	if membership != event.MembershipJoin && membership != event.MembershipInvite {
		return nil
	}
	if _, err := c.cli.KickUser(ctx, id.RoomID(roomID), &mautrix.ReqKickUser{UserID: id.UserID(operator)}); err != nil {
		return fmt.Errorf("remove operator from %s: %w", roomID, err)
	}
	return nil
}

// LeaveAndForget removes the bridge from a room and forgets it. Leaving first
// is what makes the room disappear from a room list; forgetting drops the
// bridge's local knowledge of that membership on the homeserver.
func (c *Client) LeaveAndForget(ctx context.Context, roomID string) error {
	membership, err := c.membership(ctx, roomID, c.cli.UserID.String())
	if err != nil {
		return err
	}
	if membership == event.MembershipJoin || membership == event.MembershipInvite {
		if _, err := c.cli.LeaveRoom(ctx, id.RoomID(roomID)); err != nil {
			return fmt.Errorf("leave %s: %w", roomID, err)
		}
	}
	if _, err := c.cli.ForgetRoom(ctx, id.RoomID(roomID)); err != nil && !isMissing(err) {
		return fmt.Errorf("forget %s: %w", roomID, err)
	}
	return nil
}
