package steps

import (
	"context"
	"testing"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/internal/dashboard"
	"github.com/unclebob/forgelet-bridge/internal/relay"
)

func TestFakeDashboardLegacyStateCannotReachTheSnapshot(t *testing.T) {
	fake := fixtures.StartFakeDashboard()
	defer fake.Close()
	fake.SetUnsupportedWithFullState(dashboard.State{
		ResolvedApprovals: []dashboard.ResolvedApprovalRequest{{
			ID: "card-a", Project: "project-a", Resolution: relay.ResolutionApproved,
		}},
	})

	snapshot, err := (dashboard.SnapshotSource{
		Root: "/forges/forge-a", ConfiguredURL: fake.URL(),
	}).Snapshot(context.Background())
	if err == nil {
		t.Fatal("legacy compatibility state reached the snapshot")
	}
	if snapshot.ResolvedApprovals != nil || snapshot.TypedClarifications != nil {
		t.Fatalf("snapshot = %+v, want no consumer state from the failed projection", snapshot)
	}
}
