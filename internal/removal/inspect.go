package removal

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/unclebob/forgelet-bridge/internal/config"
	"github.com/unclebob/forgelet-bridge/internal/state"
)

// Inspect validates everything that can be checked without touching Matrix.
// It deliberately leaves the state file unmodified.
func Inspect(cfg config.Config, loaded *state.State, root string) Inspection {
	if strings.TrimSpace(root) == "" || hasParentTraversal(root) {
		return refusedInspection("unsafe_root")
	}

	matched := matchingRoots(cfg, root)
	if len(matched) > 1 {
		return refusedInspection("ambiguous_root")
	}
	if !filepath.IsAbs(root) {
		return refusedInspection("unsafe_root")
	}
	if len(matched) == 0 {
		return refusedInspection("unknown_root")
	}
	if loaded == nil {
		return refusedInspection("incomplete_state")
	}
	forge, ok := loaded.Forges[root]
	if !ok {
		return Inspection{Result: Result{Status: NothingToRemove}}
	}
	if !completeForge(forge) {
		return refusedInspection("incomplete_state")
	}

	return Inspection{
		Result: Result{Status: Ready},
		Root:   root,
		Forge:  &forge,
		Rooms:  orderedRooms(forge),
	}
}

func matchingRoots(cfg config.Config, root string) []string {
	var matched []string
	for _, forge := range cfg.Forges {
		if forge.Root == root {
			return []string{root}
		}
		if !filepath.IsAbs(root) && filepath.Base(filepath.Clean(forge.Root)) == root {
			matched = append(matched, forge.Root)
		}
	}
	return matched
}

func hasParentTraversal(root string) bool {
	for _, part := range strings.FieldsFunc(root, func(r rune) bool {
		return r == '/' || r == '\\'
	}) {
		if part == ".." {
			return true
		}
	}
	return false
}

func completeForge(forge state.Forge) bool {
	if forge.SpaceID == "" || forge.RoomID == "" ||
		forge.ApprovalsRoomID == "" || forge.ActivityRoomID == "" ||
		forge.ClarificationsRoomID == "" {
		return false
	}
	for _, project := range forge.Projects {
		if project.RoomID == "" {
			return false
		}
	}
	return true
}

func orderedRooms(forge state.Forge) []string {
	candidates := []string{
		forge.SpaceID,
		forge.RoomID,
		forge.ApprovalsRoomID,
		forge.ActivityRoomID,
		forge.ClarificationsRoomID,
	}
	names := make([]string, 0, len(forge.Projects))
	for name := range forge.Projects {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		candidates = append(candidates, forge.Projects[name].RoomID)
	}

	seen := make(map[string]bool, len(candidates))
	rooms := make([]string, 0, len(candidates))
	for _, roomID := range candidates {
		if roomID == "" || seen[roomID] {
			continue
		}
		seen[roomID] = true
		rooms = append(rooms, roomID)
	}
	return rooms
}
