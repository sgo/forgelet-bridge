package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
)

// caughtUp waits for the bridge to finish a tick with nothing left to do.
func (w *World) caughtUp(ctx context.Context) error {
	path := filepath.Join(w.stateDir, "status.json")
	before, _ := readStatus(path)
	for {
		status, err := readStatus(path)
		if err == nil && status.Tick > before.Tick && status.Idle {
			return nil
		}
		if err := fixtures.Sleep(ctx, 100*time.Millisecond); err != nil {
			return fmt.Errorf("the bridge never caught up: %w", err)
		}
	}
}

type status struct {
	Tick uint64 `json:"tick"`
	Idle bool   `json:"idle"`
}

func readStatus(path string) (status, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return status{}, err
	}
	var parsed status
	if err := json.Unmarshal(data, &parsed); err != nil {
		return status{}, err
	}
	return parsed, nil
}

// waitForSpaceChild waits for a chat room of a given name inside a forge space
// the operator can see and is in.
func (w *World) waitForSpaceChild(ctx context.Context, spaceName, childName string) (string, error) {
	var found string
	err := waitFor(ctx, fmt.Sprintf("the forge space %s never showed a chat room %s", spaceName, childName), func() (bool, error) {
		spaces, err := w.spaces(ctx, spaceName)
		if err != nil {
			return false, err
		}
		operator, err := w.operator(ctx)
		if err != nil {
			return false, err
		}
		for _, spaceID := range spaces {
			children, err := operator.SpaceChildren(ctx, spaceID)
			if err != nil {
				continue
			}
			for _, child := range children {
				name, err := operator.RoomName(ctx, child)
				if err != nil || name != childName {
					continue
				}
				membership, err := operator.Membership(ctx, child, w.operatorID)
				if err != nil || membership != "join" {
					continue
				}
				found = child
				return true, nil
			}
		}
		return false, nil
	})
	return found, err
}

// chatRoom finds the one chat room of a given name the operator is in.
func (w *World) chatRoom(ctx context.Context, name string) (string, error) {
	rooms, err := w.rooms(ctx, name)
	if err != nil {
		return "", err
	}
	if len(rooms) == 0 {
		return "", fmt.Errorf("the operator never saw a chat room named %s", name)
	}
	if len(rooms) > 1 {
		return "", fmt.Errorf("the operator sees %d chat rooms named %s: %v", len(rooms), name, rooms)
	}
	return rooms[0], nil
}

// rooms lists the rooms the operator has with a given name, whether joined or
// still only invited.
func (w *World) rooms(ctx context.Context, name string) ([]string, error) {
	return w.roomsMatching(ctx, func(operator *fixtures.User, roomID string) bool {
		roomName, err := operator.RoomName(ctx, roomID)
		return err == nil && roomName == name
	})
}

// spaces lists the spaces of a given name the operator knows about. An empty
// name lists every space.
func (w *World) spaces(ctx context.Context, name string) ([]string, error) {
	return w.roomsMatching(ctx, func(operator *fixtures.User, roomID string) bool {
		isSpace, err := operator.IsSpace(ctx, roomID)
		if err != nil || !isSpace {
			return false
		}
		if name == "" {
			return true
		}
		roomName, err := operator.RoomName(ctx, roomID)
		return err == nil && roomName == name
	})
}

// allSpaces lists every space the operator knows about.
func (w *World) allSpaces(ctx context.Context) ([]string, error) {
	return w.spaces(ctx, "")
}

// roomsMatching polls the rooms the operator knows about until match takes at
// least one of them, or the step runs out of time.
func (w *World) roomsMatching(ctx context.Context, match func(*fixtures.User, string) bool) ([]string, error) {
	deadline := time.Now().Add(stepTimeout)
	for {
		operator, err := w.operator(ctx)
		if err != nil {
			return nil, err
		}
		candidates, err := operator.Rooms(ctx)
		if err != nil {
			return nil, err
		}
		var found []string
		for _, roomID := range candidates {
			if match(operator, roomID) {
				found = append(found, roomID)
			}
		}
		if len(found) > 0 || time.Now().After(deadline) {
			return found, nil
		}
		if err := fixtures.Sleep(ctx, 100*time.Millisecond); err != nil {
			return nil, err
		}
	}
}

// waitFor polls until check succeeds or the context runs out.
func waitFor(ctx context.Context, describe string, check func() (bool, error)) error {
	for {
		ok, err := check()
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		if err := fixtures.Sleep(ctx, 100*time.Millisecond); err != nil {
			return fmt.Errorf("%s: %w", describe, err)
		}
	}
}
