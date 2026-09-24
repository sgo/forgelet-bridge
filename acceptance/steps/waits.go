package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/internal/bridge"
	"github.com/unclebob/forgelet-bridge/internal/config"
)

// caughtUp waits for the bridge to finish a tick with nothing left to do,
// counting only ticks it began after this step looked at the status: what a
// step has just written to the forge is only known handled once a tick that
// started afterwards has come round and found nothing left.
func (w *World) caughtUp(ctx context.Context) error {
	path := filepath.Join(w.stateDir, bridge.StatusName)
	before, _ := readStatus(path)
	for {
		status, err := readStatus(path)
		if err == nil && settledSince(status, before.Tick) {
			return nil
		}
		if err := fixtures.Sleep(ctx, 100*time.Millisecond); err != nil {
			return fmt.Errorf("the bridge never caught up: %w", err)
		}
	}
}

// settledSince reports whether a status is one a step can trust: a tick the
// bridge began after the step looked - never the tick it was already running
// then, which may have read the forge before the step wrote to it - that it
// finished with nothing left to do.
func settledSince(status bridge.Status, seen uint64) bool {
	return status.Tick >= seen+2 && status.Idle
}

// readStatus reads the progress the bridge reports, in the shape the bridge
// writes it.
func readStatus(path string) (bridge.Status, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return bridge.Status{}, err
	}
	var parsed bridge.Status
	if err := json.Unmarshal(data, &parsed); err != nil {
		return bridge.Status{}, err
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
				if err != nil || !w.namesARoom(childName, name) {
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
		return err == nil && w.namesARoom(name, roomName)
	})
}

// namesARoom reports whether a room's own name is the one a step named. A step
// that names a channel without its forge - "Chat" - also names the room that
// carries it, because that is the room the scenario means: the channel comes
// first and the forge's name follows it in brackets.
func (w *World) namesARoom(named, roomName string) bool {
	for _, candidate := range w.roomNames(named) {
		if roomName == candidate {
			return true
		}
	}
	return false
}

// roomNames is the names a room step may be naming: the name itself, and - when
// it names a channel without a forge - that channel with the name of every
// forge the scenario declared.
func (w *World) roomNames(named string) []string {
	names := []string{named}
	if strings.Contains(named, " (") {
		return names
	}
	for _, root := range append(append([]string{}, w.configured...), w.forgeRoots...) {
		names = appendUnique(names, config.RoomNameIn(w.forgeDisplayName(root), named))
	}
	return names
}

// forgeDisplayName is what the operator knows one fixture forge by: the name
// the bridge is configured with, or the folder the forge lives in.
func (w *World) forgeDisplayName(root string) string {
	if named := w.forgeNames[root]; named != "" {
		return named
	}
	return filepath.Base(root)
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
		found, err := w.roomsMatchingNow(ctx, match)
		if err != nil {
			return nil, err
		}
		if len(found) > 0 || time.Now().After(deadline) {
			return found, nil
		}
		if err := fixtures.Sleep(ctx, 100*time.Millisecond); err != nil {
			return nil, err
		}
	}
}

// roomsMatchingNow lists the rooms the operator knows about that match right
// now, without waiting for one to appear: a step that checks something is
// absent cannot wait for it.
func (w *World) roomsMatchingNow(ctx context.Context, match func(*fixtures.User, string) bool) ([]string, error) {
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
	return found, nil
}

// spacesNow lists the forge spaces the operator knows about right now.
func (w *World) spacesNow(ctx context.Context) ([]string, error) {
	return w.roomsMatchingNow(ctx, func(operator *fixtures.User, roomID string) bool {
		isSpace, err := operator.IsSpace(ctx, roomID)
		return err == nil && isSpace
	})
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
