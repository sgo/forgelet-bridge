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
	StatePath      string
	Cleaner        Cleaner
	ProjectCleaner ProjectCleaner
	Projects       ProjectCatalog
	Operator       string
	Config         config.Config
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

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-11-03T00:56:00+01:00","module_hash":"ac5990dbb0406dbef21f564f6002b52dbd10e9b4344fad52aa712d4069a6e81b","functions":[{"id":"func/Service.Load","name":"Service.Load","line":64,"end_line":66,"hash":"0628de52e64079c7d3e7b05394743574d498f4fe7c46a89e29935ba7cce9559f"},{"id":"func/Service.Remove","name":"Service.Remove","line":70,"end_line":87,"hash":"f05b3188b213ae17e5244fe2b4d94cc4c39ca0809564d684679b8687ab5d45f5"},{"id":"func/Service.reload","name":"Service.reload","line":89,"end_line":99,"hash":"c843de937df539cd6babdad68544e8571ee405af9b5f867e6520e07d656a1f57"},{"id":"func/Service.cleanRoom","name":"Service.cleanRoom","line":101,"end_line":125,"hash":"c6e1b1a99b12eb6a59457d6e326356ee42e9e76399c54965682318afcc5d7382"},{"id":"func/Service.cleanStage","name":"Service.cleanStage","line":127,"end_line":149,"hash":"e5de51c358add363a146b006bf1ad02f9249e0f9723144fa3577946226ee27f4"},{"id":"func/Service.finalize","name":"Service.finalize","line":151,"end_line":166,"hash":"abfb1bbd2290b2f6bbb9211d46eb3f9e617c07fdb1e1f56e4abab9d8713b6657"},{"id":"func/WriteResult","name":"WriteResult","line":169,"end_line":171,"hash":"66cd8f62d78a0ed162c5ab8f34ceb1bb4dcbbd4a768d32919aa9c50d4ee39201"},{"id":"func/Service.saveForge","name":"Service.saveForge","line":173,"end_line":180,"hash":"1964a0447878a92542dac0164fad1710d003fb1b3b03f2bfafb51fad377b689d"},{"id":"func/ensureCleanup","name":"ensureCleanup","line":182,"end_line":189,"hash":"652421076c9f6312a860ccff9e641e9e4d5ecb5791c060323d2dbd9babb8bed0"},{"id":"func/retryable","name":"retryable","line":191,"end_line":193,"hash":"62ad696ddf5514da3089f71fb5b2f10f5750f8ef16e8027707fa3158bd3efe36"},{"id":"func/refusedInspection","name":"refusedInspection","line":195,"end_line":197,"hash":"87a98e5a5015cbcc780f63e3b767b398fc951f0bbe54079a5d14ced04292a867"}]}
// mutate4go-manifest-end
