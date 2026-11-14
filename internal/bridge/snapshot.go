package bridge

import (
	"context"
	"fmt"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

// snapshotResult is one dashboard read in a tick. Failures are cached too, so
// every consumer in the tick sees the same failure and none retries it.
type snapshotResult struct {
	snapshot relay.DashboardSnapshot
	err      error
}

// snapshotCache is local to one Tick. Nothing in it can become stale state for
// a later tick.
type snapshotCache map[string]snapshotResult

// snapshotFrom reads a dashboard once for the identity in this tick. A forge
// and a project that address the same dashboard share the result.
func (b *Bridge) snapshotFrom(ctx context.Context, cache snapshotCache, source SnapshotStore) (relay.DashboardSnapshot, error) {
	identity, err := source.Identity()
	if err != nil {
		return relay.DashboardSnapshot{}, fmt.Errorf("identify dashboard snapshot: %w", err)
	}
	if cached, ok := cache[identity]; ok {
		return cached.snapshot, cached.err
	}
	snapshot, err := source.Snapshot(ctx)
	if err != nil {
		err = fmt.Errorf("read dashboard snapshot: %w", err)
	}
	cache[identity] = snapshotResult{snapshot: snapshot, err: err}
	return snapshot, err
}
