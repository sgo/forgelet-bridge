package steps

import (
	"context"
	"testing"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/internal/dashboard"
	"github.com/unclebob/forgelet-bridge/internal/relay"
)

func TestFakeDashboardCompatibilityStateReachesTheSnapshot(t *testing.T) {
	fake := fixtures.StartFakeDashboard()
	defer fake.Close()
	fake.SetUnsupportedWithFullState(dashboard.State{
		ResolvedApprovals: []dashboard.ResolvedApprovalRequest{{
			ID: "card-a", Project: "project-a", Resolution: relay.ResolutionApproved,
		}},
	})

	snapshot, err := (dashboard.SnapshotSource{
		Root: "/forges/forge-a", ConfiguredURL: fake.URL(), Project: "project-a",
	}).Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := snapshot.ResolvedApprovals["/forges/forge-a/project-a/card-a"]; got != relay.ResolutionApproved {
		t.Fatalf("resolved approval = %q, want approved", got)
	}
}
