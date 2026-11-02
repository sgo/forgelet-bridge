// Package removal clears one configured forge's Matrix rooms and bridge-owned
// bookkeeping without changing the forge itself or the bridge configuration.
package removal

import (
	"context"
	"encoding/json"
	"io"

	"github.com/unclebob/forgelet-bridge/internal/config"
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
	fresh, result, ready := s.reload(inspection.Root)
	if !ready {
		return result
	}
	forge := fresh.Forge
	ensureCleanup(forge)
	for _, roomID := range fresh.Rooms {
		_, result := s.cleanRoom(ctx, fresh.Root, roomID, forge)
		if result.Status != "" {
			return result
		}
	}
	return s.finalize(fresh.Root)
}

func (s Service) reload(root string) (Inspection, Result, bool) {
	if s.Cleaner == nil {
		return Inspection{}, retryable("matrix_unavailable"), false
	}
	loaded, err := s.Load()
	if err != nil {
		return Inspection{}, retryable("state_unavailable"), false
	}
	fresh := Inspect(s.Config, loaded, root)
	return fresh, fresh.Result, fresh.Result.Status == Ready
}

func (s Service) cleanRoom(
	ctx context.Context,
	root, roomID string,
	forge *state.Forge,
) (state.RoomCleanup, Result) {
	progress := forge.Cleanup.Rooms[roomID]
	progress, result := s.cleanStage(ctx, root, roomID, forge, progress, progress.FactsCleared,
		func() error { return s.Cleaner.ClearFacts(ctx, roomID) },
		func(progress *state.RoomCleanup) { progress.FactsCleared = true },
		"relay_cleanup_failed")
	if result.Status != "" {
		return progress, result
	}
	progress, result = s.cleanStage(ctx, root, roomID, forge, progress, progress.OperatorRemoved,
		func() error { return s.Cleaner.RemoveOperator(ctx, roomID, s.Operator) },
		func(progress *state.RoomCleanup) { progress.OperatorRemoved = true },
		"operator_removal_failed")
	if result.Status != "" {
		return progress, result
	}
	return s.cleanStage(ctx, root, roomID, forge, progress, progress.BridgeLeft,
		func() error { return s.Cleaner.LeaveAndForget(ctx, roomID) },
		func(progress *state.RoomCleanup) { progress.BridgeLeft = true },
		"bridge_membership_failed")
}

func (s Service) cleanStage(
	_ context.Context,
	root, roomID string,
	forge *state.Forge,
	progress state.RoomCleanup,
	done bool,
	run func() error,
	mark func(*state.RoomCleanup),
	failureReason string,
) (state.RoomCleanup, Result) {
	if done {
		return progress, Result{}
	}
	if err := run(); err != nil {
		return progress, retryable(failureReason)
	}
	mark(&progress)
	forge.Cleanup.Rooms[roomID] = progress
	if !s.saveForge(root, forge) {
		return progress, retryable("state_unavailable")
	}
	return progress, Result{}
}

func (s Service) finalize(root string) Result {
	loaded, err := s.Load()
	if err != nil {
		return retryable("state_unavailable")
	}
	persisted, ok := loaded.Forges[root]
	if !ok {
		return Result{Status: NothingToRemove}
	}
	purgeRelay(&loaded.Relay, root, persisted)
	delete(loaded.Forges, root)
	if err := loaded.Save(s.StatePath); err != nil {
		return retryable("state_unavailable")
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

func ensureCleanup(forge *state.Forge) {
	if forge.Cleanup == nil {
		forge.Cleanup = &state.Cleanup{Rooms: map[string]state.RoomCleanup{}}
	}
	if forge.Cleanup.Rooms == nil {
		forge.Cleanup.Rooms = map[string]state.RoomCleanup{}
	}
}

func retryable(reason string) Result {
	return Result{Status: Retryable, Reason: reason}
}

func refusedInspection(reason string) Inspection {
	return Inspection{Result: Result{Status: Refused, Reason: reason}}
}
