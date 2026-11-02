// Package removal clears one configured forge's Matrix rooms and bridge-owned
// bookkeeping without changing the forge itself or the bridge configuration.
package removal

import (
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/unclebob/forgelet-bridge/internal/config"
	"github.com/unclebob/forgelet-bridge/internal/relay"
	"github.com/unclebob/forgelet-bridge/internal/state"
)

// Status is the stable outcome of a removal request.
type Status string

const (
	// Ready is the private planning result an adapter gets before it stops the
	// bridge. It is never returned by the public command.
	Ready Status = "ready"
	// Removed means every recorded room and the forge record were removed.
	Removed Status = "removed"
	// NothingToRemove means the configured forge has no persisted forge record.
	NothingToRemove Status = "nothing_to_remove"
	// Retryable means saved room ids and per-room progress allow another try.
	Retryable Status = "retryable"
	// Refused means validation stopped the request before Matrix changed.
	Refused Status = "refused"
)

// Result is the public, sanitized result written by the adapter.
type Result struct {
	Status Status `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// Inspection is a validated removal request. Rooms are in a stable order and
// contain each distinct Matrix room once.
type Inspection struct {
	Result Result
	Root   string
	Forge  *state.Forge
	Rooms  []string
}

// Cleaner is the Matrix boundary needed to remove a room.
type Cleaner interface {
	ClearFacts(ctx context.Context, roomID string) error
	RemoveOperator(ctx context.Context, roomID, operator string) error
	LeaveAndForget(ctx context.Context, roomID string) error
}

// Service carries out one inspected removal and keeps retry progress in the
// bridge state between attempts.
type Service struct {
	StatePath string
	Cleaner   Cleaner
	Operator  string
	Config    config.Config
}

// Inspect validates everything that can be checked without touching Matrix.
// It deliberately leaves the state unmarshaled result untouched.
func Inspect(cfg config.Config, loaded *state.State, root string) Inspection {
	if blank := strings.TrimSpace(root); blank == "" {
		return refusedInspection("unsafe_root")
	}
	if hasParentTraversal(root) {
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

// Load reads the bridge state a removal request must validate before stopping
// the running process or connecting to Matrix.
func (s Service) Load() (*state.State, error) {
	return state.Load(s.StatePath)
}

// Remove carries out an inspection. A refused or already completed request
// never reaches Matrix. A retry skips room stages already saved as complete.
func (s Service) Remove(ctx context.Context, inspection Inspection) Result {
	if inspection.Result.Status != Ready {
		return inspection.Result
	}
	if s.Cleaner == nil {
		return Result{Status: Retryable, Reason: "matrix_unavailable"}
	}

	loaded, err := s.Load()
	if err != nil {
		return Result{Status: Retryable, Reason: "state_unavailable"}
	}
	fresh := Inspect(s.Config, loaded, inspection.Root)
	if fresh.Result.Status != Ready {
		return fresh.Result
	}

	forge := fresh.Forge
	if forge.Cleanup == nil {
		forge.Cleanup = &state.Cleanup{Rooms: map[string]state.RoomCleanup{}}
	}
	if forge.Cleanup.Rooms == nil {
		forge.Cleanup.Rooms = map[string]state.RoomCleanup{}
	}

	for _, roomID := range fresh.Rooms {
		progress := forge.Cleanup.Rooms[roomID]
		if !progress.FactsCleared {
			if err := s.Cleaner.ClearFacts(ctx, roomID); err != nil {
				return Result{Status: Retryable, Reason: "relay_cleanup_failed"}
			}
			progress.FactsCleared = true
			forge.Cleanup.Rooms[roomID] = progress
			if !s.saveForge(fresh.Root, forge) {
				return Result{Status: Retryable, Reason: "state_unavailable"}
			}
		}
		if !progress.OperatorRemoved {
			if err := s.Cleaner.RemoveOperator(ctx, roomID, s.Operator); err != nil {
				return Result{Status: Retryable, Reason: "operator_removal_failed"}
			}
			progress.OperatorRemoved = true
			forge.Cleanup.Rooms[roomID] = progress
			if !s.saveForge(fresh.Root, forge) {
				return Result{Status: Retryable, Reason: "state_unavailable"}
			}
		}
		if !progress.BridgeLeft {
			if err := s.Cleaner.LeaveAndForget(ctx, roomID); err != nil {
				return Result{Status: Retryable, Reason: "bridge_membership_failed"}
			}
			progress.BridgeLeft = true
			forge.Cleanup.Rooms[roomID] = progress
			if !s.saveForge(fresh.Root, forge) {
				return Result{Status: Retryable, Reason: "state_unavailable"}
			}
		}
	}

	loaded, err = s.Load()
	if err != nil {
		return Result{Status: Retryable, Reason: "state_unavailable"}
	}
	persisted, ok := loaded.Forges[fresh.Root]
	if !ok {
		return Result{Status: NothingToRemove}
	}
	purgeRelay(&loaded.Relay, fresh.Root, persisted)
	delete(loaded.Forges, fresh.Root)
	if err := loaded.Save(s.StatePath); err != nil {
		return Result{Status: Retryable, Reason: "state_unavailable"}
	}
	return Result{Status: Removed}
}

// WriteResult writes the one JSON object a public removal command returns.
func WriteResult(writer io.Writer, result Result) error {
	return json.NewEncoder(writer).Encode(result)
}

func (s Service) saveForge(root string, forge *state.Forge) bool {
	loaded, err := s.Load()
	if err != nil {
		return false
	}
	loaded.RecordForge(root, *forge)
	return loaded.Save(s.StatePath) == nil
}

func refusedInspection(reason string) Inspection {
	return Inspection{Result: Result{Status: Refused, Reason: reason}}
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

func purgeRelay(bookkeeping *relay.State, root string, forge state.Forge) {
	bookkeeping.EnsureMaps()
	bookkeeping.ForgetScoped(root)
	rooms := make(map[string]bool)
	for _, roomID := range orderedRooms(forge) {
		rooms[roomID] = true
	}
	for key := range bookkeeping.Waiting {
		roomID, _, _, ok := splitWaiting(key)
		if ok && rooms[roomID] {
			delete(bookkeeping.Waiting, key)
		}
	}
	for key, approval := range bookkeeping.Approvals {
		if approval.ForgeRoot == root || rooms[approval.RoomID] {
			delete(bookkeeping.Approvals, key)
		}
	}
	for key, clarification := range bookkeeping.Clarifications {
		if clarification.ForgeRoot == root || rooms[clarification.RoomID] {
			delete(bookkeeping.Clarifications, key)
		}
	}
	for key, activity := range bookkeeping.Activity {
		if activity.ForgeRoot == root {
			delete(bookkeeping.Activity, key)
		}
	}
}

func splitWaiting(key string) (string, string, string, bool) {
	parts := strings.SplitN(key, "\n", 3)
	if len(parts) != 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}
