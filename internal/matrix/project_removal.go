package matrix

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type projectFact struct {
	eventType event.Type
	stateKey  string
}

type projectFactMatch func(event.Type, string, *event.Event) bool

// ClearProjectFacts clears one project's facts without touching a sibling
// project or a forge-level room's own facts.
func (c *Client) ClearProjectFacts(ctx context.Context, projectRoomID, activityRoomID, approvalsRoomID, project string) error {
	return errors.Join(
		c.clearProjectRoomFacts(ctx, projectRoomID),
		c.clearActivityProjectFacts(ctx, activityRoomID, project),
		c.clearApprovalProjectFacts(ctx, approvalsRoomID, project),
	)
}

func (c *Client) clearProjectRoomFacts(ctx context.Context, roomID string) error {
	return c.clearMatchingProjectFacts(ctx, roomID, allProjectFacts)
}

func (c *Client) clearActivityProjectFacts(ctx context.Context, roomID, project string) error {
	return c.clearMatchingProjectFacts(ctx, roomID,
		func(eventType event.Type, stateKey string, evt *event.Event) bool {
			return isProjectBoard(project, eventType, stateKey, evt)
		})
}

func (c *Client) clearApprovalProjectFacts(ctx context.Context, roomID, project string) error {
	return c.clearMatchingProjectFacts(ctx, roomID,
		func(eventType event.Type, stateKey string, evt *event.Event) bool {
			return isProjectApproval(project, eventType, stateKey, evt)
		})
}

func (c *Client) clearMatchingProjectFacts(ctx context.Context, roomID string, match projectFactMatch) error {
	state, err := c.cli.State(ctx, id.RoomID(roomID))
	if err != nil {
		return fmt.Errorf("read project facts in %s: %w", roomID, err)
	}
	facts := matchingProjectFacts(state, match)
	return c.clearProjectFacts(ctx, roomID, facts)
}

func matchingProjectFacts(state map[event.Type]map[string]*event.Event, match projectFactMatch) []projectFact {
	var facts []projectFact
	for eventType, byStateKey := range state {
		if !strings.HasPrefix(eventType.Type, forgeFactPrefix) {
			continue
		}
		for stateKey, evt := range byStateKey {
			if match(eventType, stateKey, evt) {
				facts = append(facts, projectFact{eventType: eventType, stateKey: stateKey})
			}
		}
	}
	sort.Slice(facts, func(i, j int) bool {
		if facts[i].eventType.Type != facts[j].eventType.Type {
			return facts[i].eventType.Type < facts[j].eventType.Type
		}
		return facts[i].stateKey < facts[j].stateKey
	})
	return facts
}

func (c *Client) clearProjectFacts(ctx context.Context, roomID string, facts []projectFact) error {
	errs := make([]error, len(facts))
	for index, fact := range facts {
		_, errs[index] = c.cli.SendStateEvent(ctx, id.RoomID(roomID), fact.eventType, fact.stateKey, map[string]any{})
	}
	return errors.Join(errs...)
}

func factProject(evt *event.Event) string {
	if evt == nil {
		return ""
	}
	return decodedProject(evt.Content.Raw)
}

func decodedProject(content map[string]interface{}) string {
	project, ok := content["project"].(string)
	if !ok {
		return ""
	}
	return project
}

func allProjectFacts(event.Type, string, *event.Event) bool {
	return true
}

func isProjectBoard(project string, eventType event.Type, stateKey string, _ *event.Event) bool {
	return eventType.Type == "com.forgelet.board" && stateKey == project
}

func isProjectApproval(project string, eventType event.Type, _ string, evt *event.Event) bool {
	return eventType.Type == "com.forgelet.approval" && factProject(evt) == project
}
