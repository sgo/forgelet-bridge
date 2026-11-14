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

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-11-14T23:40:44+01:00","module_hash":"e344de0616f40518b32c58755bbeb4e46cbc16505db3e940bd526337ed6cb321","functions":[{"id":"func/Bridge.snapshotFrom","name":"Bridge.snapshotFrom","line":23,"end_line":37,"hash":"71424fa3e14c4788ec798991f510a2e6eebab9b2657466d487ebdaa60d533ad0"}]}
// mutate4go-manifest-end
