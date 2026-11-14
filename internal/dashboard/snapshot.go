package dashboard

import (
	"context"
	"fmt"
	"strings"

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

// Identity makes the approvals facade the forge's dashboard-snapshot source.
func (a Approvals) Identity() (string, error) {
	return (SnapshotSource{Root: a.Root, ConfiguredURL: a.ConfiguredURL}).Identity()
}

// Snapshot makes the approvals facade serve the forge's complete dashboard
// state, including clarifications.
func (a Approvals) Snapshot(ctx context.Context) (relay.DashboardSnapshot, error) {
	return (SnapshotSource{Root: a.Root, ConfiguredURL: a.ConfiguredURL}).Snapshot(ctx)
}

// Identity makes the clarifications facade a project dashboard-snapshot
// source.
func (c Clarifications) Identity() (string, error) {
	return (SnapshotSource{Root: c.Root, ConfiguredURL: c.ConfiguredURL, Project: c.Project}).Identity()
}

// Snapshot makes the clarifications facade serve its project's complete
// dashboard state.
func (c Clarifications) Snapshot(ctx context.Context) (relay.DashboardSnapshot, error) {
	return (SnapshotSource{Root: c.Root, ConfiguredURL: c.ConfiguredURL, Project: c.Project}).Snapshot(ctx)
}

// Identity names the dashboard a snapshot belongs to. The root keeps two
// separately configured forges from sharing a response even when they
// happen to announce the same address.
func (s SnapshotSource) Identity() (string, error) {
	url, err := URL(s.Root, s.ConfiguredURL)
	if err != nil {
		return "", fmt.Errorf("identify dashboard snapshot for %s: %w", s.Root, err)
	}
	return strings.Join([]string{s.Root, url}, "\x00"), nil
}

// Snapshot reads the complete dashboard state once and derives every waiting
// consumer from that response.
func (s SnapshotSource) Snapshot(ctx context.Context) (relay.DashboardSnapshot, error) {
	url, err := URL(s.Root, s.ConfiguredURL)
	if err != nil {
		return relay.DashboardSnapshot{}, fmt.Errorf("find dashboard snapshot for %s: %w", s.Root, err)
	}
	api := NewForgeAPI(s.Root, url)
	state, err := api.State(ctx)
	if err != nil {
		return relay.DashboardSnapshot{}, fmt.Errorf("read dashboard snapshot for %s: %w", s.Root, err)
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

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-11-14T23:44:46+01:00","module_hash":"f3454f62ad109ba4044858ceb260114afbe80a43e2b97297c6f14ddbd57a0b5d","functions":[{"id":"func/Approvals.Identity","name":"Approvals.Identity","line":21,"end_line":23,"hash":"ac25347a37520e83449f41b897b3a6f1e69d76c4444179ad173ced8af74f30b9"},{"id":"func/Approvals.Snapshot","name":"Approvals.Snapshot","line":27,"end_line":29,"hash":"4a1e556c2f7e1f1242133b939c337b5ff866287e1f71e198ff7b0222a823a731"},{"id":"func/Clarifications.Identity","name":"Clarifications.Identity","line":33,"end_line":35,"hash":"cb041835b62b4126171f1814ad9250d89f53d87b98f8d999b4de4bbb5bf09eba"},{"id":"func/Clarifications.Snapshot","name":"Clarifications.Snapshot","line":39,"end_line":41,"hash":"c61862f227b035fd0c6ec6b9d4933ae6f507968ee4647ce763264cf65162ffd1"},{"id":"func/SnapshotSource.Identity","name":"SnapshotSource.Identity","line":46,"end_line":52,"hash":"54aa6ed417e42291276c89fbd0c0e85198c1105568e7363c2fba474f5079a55c"},{"id":"func/SnapshotSource.Snapshot","name":"SnapshotSource.Snapshot","line":56,"end_line":77,"hash":"586e4f1ceafb321483120ca90b82a7ce599cea2020a0cac569ba8aac1435c1e0"}]}
// mutate4go-manifest-end
