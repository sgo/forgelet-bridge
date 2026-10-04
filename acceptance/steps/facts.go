package steps

import (
	"context"
	"fmt"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

// waitingFact is one state event a room carries, as a step read it: the room,
// the event type and key, and the event the room wrote it as.
type waitingFact struct {
	roomID    string
	eventType string
	stateKey  string
	eventID   string
	content   map[string]any
}

// boardFactCarried checks the activity room carries one project's board as a
// com.forgelet.board state event, keyed by the project, carrying the project's
// lanes and the card in the lane the board holds it in.
func boardFactCarried(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomName, project, card, lane := captures[1], captures[2], captures[3], captures[4]
	roomID, err := w.roomNamed(ctx, roomName)
	if err != nil {
		return err
	}
	_, err = waitForFact(ctx, w, roomID, relay.BoardFactType, project, func(content map[string]any) error {
		said, ok := projectIn(content, project)
		if !ok {
			return fmt.Errorf("the board state does not carry the project %s", project)
		}
		if !carriesString(said["lanes"], lane) {
			return fmt.Errorf("the board's lanes %v do not hold the lane %s", said["lanes"], lane)
		}
		if !taskInLane(said["tasks"], card, lane) {
			return fmt.Errorf("the board's tasks %v do not hold the card %s in the lane %s", said["tasks"], card, lane)
		}
		return nil
	})
	return err
}

// approvalFactCarried checks the approvals room carries one approval as a
// com.forgelet.approval state event, keyed by the approval, naming the project
// and the gate.
func approvalFactCarried(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomName, card, project, gate := captures[1], captures[2], captures[3], captures[4]
	roomID, err := w.roomNamed(ctx, roomName)
	if err != nil {
		return err
	}
	id := approvalCardID(card)
	_, err = waitForFact(ctx, w, roomID, relay.ApprovalFactType, id, func(content map[string]any) error {
		return names(content, map[string]string{"id": id, "project": project, "task": card, "gate": gate})
	})
	return err
}

// clarificationFactCarried checks the clarifications room carries one question
// as a com.forgelet.clarification state event, keyed by the question, asking it.
func clarificationFactCarried(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomName, role, project, question := captures[1], captures[2], captures[3], captures[4]
	roomID, err := w.roomNamed(ctx, roomName)
	if err != nil {
		return err
	}
	id := clarificationID(project)
	_, err = waitForFact(ctx, w, roomID, relay.ClarificationFactType, id, func(content map[string]any) error {
		return names(content, map[string]string{"id": id, "project": project, "role": role, "body": question})
	})
	return err
}

// approvalFactCleared checks the approvals room no longer carries facts for one
// approval: its state event is gone, or cleared with empty content.
func approvalFactCleared(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomName, card := captures[1], captures[2]
	roomID, err := w.roomNamed(ctx, roomName)
	if err != nil {
		return err
	}
	return waitForClearedFact(ctx, w, roomID, relay.ApprovalFactType, approvalCardID(card))
}

// clarificationFactCleared checks the clarifications room no longer carries
// facts for one question.
func clarificationFactCleared(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomName, project := captures[1], captures[3]
	roomID, err := w.roomNamed(ctx, roomName)
	if err != nil {
		return err
	}
	return waitForClearedFact(ctx, w, roomID, relay.ClarificationFactType, clarificationID(project))
}

// boardFactCleared checks the activity room no longer carries facts for one
// project's board.
func boardFactCleared(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomName, project := captures[1], captures[2]
	roomID, err := w.roomNamed(ctx, roomName)
	if err != nil {
		return err
	}
	return waitForClearedFact(ctx, w, roomID, relay.BoardFactType, project)
}

// approvalFactUnchanged checks the approval's state event is still the one the
// room already had: a restart wrote nothing over it.
func approvalFactUnchanged(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomName, card := captures[1], captures[2]
	roomID, err := w.roomNamed(ctx, roomName)
	if err != nil {
		return err
	}
	stateKey := approvalCardID(card)
	remembered := w.stateEvents[stateEventKey(roomID, relay.ApprovalFactType, stateKey)]
	if remembered == "" {
		return fmt.Errorf("the scenario never read the approval state event for %s", card)
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	return waitFor(ctx, fmt.Sprintf("the approval state event for %s was rewritten", card), func() (bool, error) {
		eventID, content, present, err := operator.StateEvent(ctx, roomID, relay.ApprovalFactType, stateKey)
		if err != nil || !present || len(content) == 0 {
			return false, nil
		}
		return eventID == remembered, nil
	})
}

// waitForFact waits until the room carries the state event under [stateKey]
// with content [check] accepts, and returns what it read.
func waitForFact(ctx context.Context, w *World, roomID, eventType, stateKey string, check func(content map[string]any) error) (waitingFact, error) {
	operator, err := w.operator(ctx)
	if err != nil {
		return waitingFact{}, err
	}
	var read waitingFact
	err = waitFor(ctx, fmt.Sprintf("the room never carried the %s state %s", eventType, stateKey), func() (bool, error) {
		eventID, content, present, err := operator.StateEvent(ctx, roomID, eventType, stateKey)
		if err != nil || !present || len(content) == 0 {
			return false, nil
		}
		if err := check(content); err != nil {
			return false, nil
		}
		read = waitingFact{roomID: roomID, eventType: eventType, stateKey: stateKey, eventID: eventID, content: content}
		return true, nil
	})
	if err != nil {
		return waitingFact{}, err
	}
	if w.stateEvents == nil {
		w.stateEvents = map[string]string{}
	}
	w.stateEvents[stateEventKey(roomID, eventType, stateKey)] = read.eventID
	return read, nil
}

// waitForClearedFact waits until the room carries no facts under [stateKey]:
// the state event is gone, or its content is empty.
func waitForClearedFact(ctx context.Context, w *World, roomID, eventType, stateKey string) error {
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	return waitFor(ctx, fmt.Sprintf("the room still carries facts for the %s state %s", eventType, stateKey), func() (bool, error) {
		_, content, present, err := operator.StateEvent(ctx, roomID, eventType, stateKey)
		if err != nil {
			return false, nil
		}
		return !present || len(content) == 0, nil
	})
}

// names checks an event's content names every field the given way.
func names(content map[string]any, want map[string]string) error {
	for field, value := range want {
		if got, _ := content[field].(string); got != value {
			return fmt.Errorf("the state names %s %q, want %q", field, got, value)
		}
	}
	return nil
}

// projectIn finds one project in a board state's projects.
func projectIn(content map[string]any, name string) (map[string]any, bool) {
	for _, project := range objectsOf(content["projects"]) {
		if got, _ := project["name"].(string); got == name {
			return project, true
		}
	}
	return nil, false
}

// objectsOf reads a JSON array of objects the way the room said it.
func objectsOf(value any) []map[string]any {
	items, _ := value.([]any)
	var objects []map[string]any
	for _, item := range items {
		if object, ok := item.(map[string]any); ok {
			objects = append(objects, object)
		}
	}
	return objects
}

// carriesString reports whether a JSON array of strings holds a value.
func carriesString(value any, want string) bool {
	items, _ := value.([]any)
	for _, item := range items {
		if got, _ := item.(string); got == want {
			return true
		}
	}
	return false
}

// taskInLane reports whether a board's tasks hold a card in a lane.
func taskInLane(value any, card, lane string) bool {
	for _, task := range objectsOf(value) {
		name, _ := task["name"].(string)
		got, _ := task["lane"].(string)
		if name == card && got == lane {
			return true
		}
	}
	return false
}

// stateEventKey names one state event across the room it is in and its key.
func stateEventKey(roomID, eventType, stateKey string) string {
	return roomID + "|" + eventType + "|" + stateKey
}
