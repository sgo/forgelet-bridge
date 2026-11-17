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
	return (SnapshotSource{Root: c.Root, ConfiguredURL: c.ConfiguredURL}).Identity()
}

// Snapshot makes the clarifications facade serve its project's complete
// dashboard state.
func (c Clarifications) Snapshot(ctx context.Context) (relay.DashboardSnapshot, error) {
	return (SnapshotSource{Root: c.Root, ConfiguredURL: c.ConfiguredURL}).Snapshot(ctx)
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

// Snapshot reads the bounded automation projection once and derives every
// waiting consumer from that response.
func (s SnapshotSource) Snapshot(ctx context.Context) (relay.DashboardSnapshot, error) {
	url, err := URL(s.Root, s.ConfiguredURL)
	if err != nil {
		return relay.DashboardSnapshot{}, fmt.Errorf("find dashboard snapshot for %s: %w", s.Root, err)
	}
	api := NewForgeAPI(s.Root, url)
	state, err := api.AutomationState(ctx)
	if err != nil {
		return relay.DashboardSnapshot{}, fmt.Errorf("read automation snapshot for %s: %w", s.Root, err)
	}
	return automationSnapshot(api, state)
}

func automationSnapshot(api *API, state AutomationState) (relay.DashboardSnapshot, error) {
	clarifications, err := automationClarifications(api.root, state.Clarifications)
	if err != nil {
		return relay.DashboardSnapshot{}, err
	}
	snapshot := relay.DashboardSnapshot{
		Approvals:           automationApprovals(api, state.Approvals.Items),
		ResolvedApprovals:   automationResolvedApprovals(api.root, state.ResolvedApprovals.Items),
		TypedClarifications: clarifications,
		Boards:              automationBoards(api.root, state.Projects.Items),
	}
	for _, project := range state.Projects.Items {
		if project.Lanes.Count == 0 {
			snapshot.Boards = nil
			snapshot.BoardFallback = true
			break
		}
	}
	return snapshot, nil
}

func automationApprovals(api *API, items []AutomationApproval) []relay.Approval {
	approvals := make([]relay.Approval, 0, len(items))
	for _, item := range items {
		approvals = append(approvals, api.approval(ApprovalRequest{
			ID: item.ID, Project: item.Project, Card: item.Card, TaskID: item.TaskID, Gate: item.Gate,
		}))
	}
	return approvals
}

func automationResolvedApprovals(root string, items []AutomationResolvedApproval) map[string]string {
	resolved := make(map[string]string, len(items))
	for _, item := range items {
		resolved[forgeKey(root, item.Project, item.ID)] = item.Resolution
	}
	return resolved
}

func automationClarifications(root string, sections *AutomationClarifications) ([]relay.Clarification, error) {
	var typed []relay.Clarification
	seen := map[string]bool{}
	for _, item := range sections.Typed.Items {
		clarification, err := automationClarification(root, item, seen)
		if err != nil {
			return nil, err
		}
		typed = append(typed, clarification)
	}
	return typed, nil
}

func automationClarification(root string, item AutomationClarification, seen map[string]bool) (relay.Clarification, error) {
	project, id, ok := clarificationIdentity(item)
	if !ok {
		return relay.Clarification{}, fmt.Errorf("invalid typed clarification in automation projection")
	}
	identity := project + "\x00" + id
	if seen[identity] {
		return relay.Clarification{}, fmt.Errorf("duplicate typed clarification in automation projection")
	}
	seen[identity] = true
	clarification := relay.Clarification{
		Key:       typedClarificationKey(root, project, id),
		ID:        id,
		Project:   project,
		Role:      clarificationRole(item),
		Question:  strings.TrimSpace(*item.QuestionExcerpt),
		Source:    relay.ClarificationSourceTyped,
		Lifecycle: item.Status,
		Attention: item.Attention,
	}
	return clarification, nil
}

func clarificationIdentity(item AutomationClarification) (string, string, bool) {
	project := item.Project
	id := item.ClarificationID
	if project == "" || id == "" || item.QuestionExcerpt == nil {
		return "", "", false
	}
	return project, id, true
}

func typedClarificationKey(root, project, id string) string {
	return root + "/" + relay.ClarificationSourceTyped + "/" + project + "/" + id
}

func clarificationRole(item AutomationClarification) string {
	role := item.Role
	if role == "" {
		role = "project-agent"
	}
	return role
}

func automationBoards(root string, projects []AutomationProject) []relay.Board {
	boards := make([]relay.Board, 0, len(projects))
	for _, project := range projects {
		board := relay.Board{Project: project.Name, Lanes: append([]string(nil), project.Lanes.Items...)}
		for _, card := range project.Cards.Items {
			board.Cards = append(board.Cards, relay.Card{
				Key:     forgeKey(root, project.Name, card.Name),
				Project: project.Name,
				Name:    card.Name,
				Lane:    card.Lane,
				Done:    card.Lane == "done",
			})
		}
		boards = append(boards, board)
	}
	return boards
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-11-17T15:20:13+01:00","module_hash":"2e06e8a64f99bfccfb3643ffc9e612a6f55cf641b5cc967b7cf08471e66d6f25","functions":[{"id":"func/Approvals.Identity","name":"Approvals.Identity","line":20,"end_line":22,"hash":"ac25347a37520e83449f41b897b3a6f1e69d76c4444179ad173ced8af74f30b9"},{"id":"func/Approvals.Snapshot","name":"Approvals.Snapshot","line":26,"end_line":28,"hash":"4a1e556c2f7e1f1242133b939c337b5ff866287e1f71e198ff7b0222a823a731"},{"id":"func/Clarifications.Identity","name":"Clarifications.Identity","line":32,"end_line":34,"hash":"69e1a37ab926f8b4bfd1183d3fadb47e4f521be08f9c14d8652329cd749bf9fb"},{"id":"func/Clarifications.Snapshot","name":"Clarifications.Snapshot","line":38,"end_line":40,"hash":"24aefbcf9de474f8e8594e711ce307de1540e7460bfe15fa9a27d2509a24001e"},{"id":"func/SnapshotSource.Identity","name":"SnapshotSource.Identity","line":45,"end_line":51,"hash":"54aa6ed417e42291276c89fbd0c0e85198c1105568e7363c2fba474f5079a55c"},{"id":"func/SnapshotSource.Snapshot","name":"SnapshotSource.Snapshot","line":55,"end_line":66,"hash":"6ec3543b6a08f2110d38ce6fbf93d1fe007e308b363d746efbeb0be327017c76"},{"id":"func/automationSnapshot","name":"automationSnapshot","line":68,"end_line":87,"hash":"88b57163747a973014af5e7b200ed176617f6376c1a061d33cb1059e1b8dfe64"},{"id":"func/automationApprovals","name":"automationApprovals","line":89,"end_line":97,"hash":"a8ca4edcf3cf41165691fe00d04c52dad3d43d3cb5a775b12b1a3242a024844b"},{"id":"func/automationResolvedApprovals","name":"automationResolvedApprovals","line":99,"end_line":105,"hash":"5e488ebeecc18e0aefde66c8449c64603830db3d5c784c085cf0551ec4bb193e"},{"id":"func/automationClarifications","name":"automationClarifications","line":107,"end_line":118,"hash":"8c25cd840b86dd7f7a0403b793afd99f3a9fe9d274af6c503deed54346272479"},{"id":"func/automationClarification","name":"automationClarification","line":120,"end_line":141,"hash":"265be4297cea86e5af368a8f3bd59aeb4051b4abdc4372b62e50b70bbdf6e182"},{"id":"func/clarificationIdentity","name":"clarificationIdentity","line":143,"end_line":150,"hash":"3edeecdf7d85422a8902d7a2e3e0dece59f374cd03eec667c62d0ab9c191dc4e"},{"id":"func/typedClarificationKey","name":"typedClarificationKey","line":152,"end_line":154,"hash":"268c4955f00f25d78a65e2b80036b9d18a454ea08efb0d0273d659df27b32cb9"},{"id":"func/clarificationRole","name":"clarificationRole","line":156,"end_line":162,"hash":"e7293dde67f3ed0e93060f9fff9a531817c6ebee68683032012bad8cf69317bf"},{"id":"func/automationBoards","name":"automationBoards","line":164,"end_line":180,"hash":"c124f96f4c30628711a318a3b8ff6e95127a6b24b612a2b3fc1779bad6480c75"}]}
// mutate4go-manifest-end
