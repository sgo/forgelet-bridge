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

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-11-03T00:51:56+01:00","module_hash":"4789c66dfe08393b25b9c21eedb1412619e3a0be2bfb9c1b516d1128a067ee29","functions":[{"id":"func/Inspect","name":"Inspect","line":14,"end_line":46,"hash":"ca5500a6c4c5a8dbd19c07b96d5f2dfcf8e3bcc804d5f6d622fc3edfcbd4efed"},{"id":"func/matchingRoots","name":"matchingRoots","line":48,"end_line":59,"hash":"ce6e58b50bb605ff73505d12636edec1c478bef3bdf6a9228893ccb7dcb66542"},{"id":"func/hasParentTraversal","name":"hasParentTraversal","line":61,"end_line":70,"hash":"761e062dab7b3a0ddbcc3ef14d641bfe74b070673175f64fec097f3003cb4190"},{"id":"func/completeForge","name":"completeForge","line":72,"end_line":84,"hash":"46fc3935cf87023ef69d09491e7740f6f4c6c7eb9bd74c273a1cab09c4ab886c"},{"id":"func/orderedRooms","name":"orderedRooms","line":86,"end_line":113,"hash":"31c8713cd0ee9ef5e5e579c755ad7ba0369ffb2072990f8701e5d63ecf87e02f"}]}
// mutate4go-manifest-end
