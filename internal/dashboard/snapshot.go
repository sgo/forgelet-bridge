package dashboard

import (
	"context"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

// SnapshotSource reads all the waiting work one dashboard reports in one
// request. Project names the project when a project dashboard's clarification
// files do not carry one.
type SnapshotSource struct {
	Root          string
	ConfiguredURL string
	Project       string
}

// Identity names the dashboard a snapshot belongs to. The root keeps two
// separately configured forges from sharing a response even when they
// happen to announce the same address.
func (s SnapshotSource) Identity() (string, error) {
	url, err := URL(s.Root, s.ConfiguredURL)
	if err != nil {
		return "", err
	}
	return s.Root + "\x00" + url, nil
}

// Snapshot reads the complete dashboard state once and derives every waiting
// consumer from that response.
func (s SnapshotSource) Snapshot(ctx context.Context) (relay.DashboardSnapshot, error) {
	url, err := URL(s.Root, s.ConfiguredURL)
	if err != nil {
		return relay.DashboardSnapshot{}, err
	}
	api := NewForgeAPI(s.Root, url)
	state, err := api.State(ctx)
	if err != nil {
		return relay.DashboardSnapshot{}, err
	}
	clarifications := api.clarifications(state)
	for index := range clarifications {
		if clarifications[index].Project == "" {
			clarifications[index].Project = s.Project
		}
	}
	return relay.DashboardSnapshot{
		Approvals:         api.approvals(state),
		ResolvedApprovals: api.resolvedApprovals(state),
		Clarifications:    clarifications,
	}, nil
}
