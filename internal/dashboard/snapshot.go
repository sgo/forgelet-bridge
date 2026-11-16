package dashboard

import (
	"context"
	"errors"
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
		if projectionCompatibilityFallback(err) {
			snapshot, fallbackErr := compatibilitySnapshot(ctx, api, s.Project)
			if fallbackErr != nil {
				return relay.DashboardSnapshot{}, fmt.Errorf(
					"read automation snapshot for %s: %w; compatibility state fallback: %v",
					s.Root, err, fallbackErr)
			}
			return snapshot, nil
		}
		return relay.DashboardSnapshot{}, fmt.Errorf("read automation snapshot for %s: %w", s.Root, err)
	}
	return automationSnapshot(api, state, s.Project)
}

func projectionCompatibilityFallback(err error) bool {
	var unsupported UnsupportedProjectionError
	if errors.As(err, &unsupported) {
		return true
	}
	var legacy LegacyProjectionError
	return errors.As(err, &legacy)
}

func compatibilitySnapshot(ctx context.Context, api *API, project string) (relay.DashboardSnapshot, error) {
	state, err := api.compatibilityState(ctx)
	if err != nil {
		return relay.DashboardSnapshot{}, err
	}
	clarifications := api.clarifications(state)
	for index := range clarifications {
		if clarifications[index].Project == "" {
			clarifications[index].Project = project
		}
	}
	snapshot := relay.DashboardSnapshot{
		Approvals:          api.approvals(state),
		ResolvedApprovals:  api.resolvedApprovals(state),
		Clarifications:     clarifications,
		BoardFallback:      true,
		ProjectionFallback: true,
	}
	return snapshot, nil
}

func automationSnapshot(api *API, state AutomationState, project string) (relay.DashboardSnapshot, error) {
	clarifications, err := automationClarifications(api.root, state.Clarifications, project)
	if err != nil {
		return relay.DashboardSnapshot{}, err
	}
	snapshot := relay.DashboardSnapshot{
		Approvals:           automationApprovals(api, state.Approvals.Items),
		ResolvedApprovals:   automationResolvedApprovals(api.root, state.ResolvedApprovals.Items),
		Clarifications:      clarifications.legacy,
		TypedClarifications: clarifications.typed,
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

type automationClarificationSets struct {
	legacy []relay.Clarification
	typed  []relay.Clarification
}

func automationClarifications(root string, sections *AutomationClarifications, project string) (automationClarificationSets, error) {
	sets := automationClarificationSets{}
	seen := map[string]bool{}
	var err error
	if sets.legacy, err = automationClarificationSection(root, "legacy", sections.Legacy.Items, project, "", seen); err != nil {
		return sets, err
	}
	if sets.typed, err = automationClarificationSection(root, "typed", sections.Typed.Items, project, "open", seen); err != nil {
		return sets, err
	}
	return sets, nil
}

func automationClarificationSection(root, kind string, items []AutomationClarification, fallbackProject, status string, seen map[string]bool) ([]relay.Clarification, error) {
	var pending []relay.Clarification
	for _, item := range items {
		if !clarificationStatusMatches(status, item.Status) {
			continue
		}
		clarification, err := automationClarification(root, kind, item, fallbackProject, seen)
		if err != nil {
			return nil, err
		}
		pending = append(pending, clarification)
	}
	return pending, nil
}

func clarificationStatusMatches(wanted, status string) bool {
	if wanted == "" {
		return status == "pending"
	}
	return status == wanted
}

func automationClarification(root, kind string, item AutomationClarification, fallbackProject string, seen map[string]bool) (relay.Clarification, error) {
	project, id, ok := clarificationIdentity(kind, item, fallbackProject)
	if !ok {
		return relay.Clarification{}, fmt.Errorf("invalid %s clarification in automation projection", kind)
	}
	key := forgeKey(root, project, id)
	if seen[key] {
		return relay.Clarification{}, fmt.Errorf("duplicate %s clarification in automation projection", kind)
	}
	seen[key] = true
	return relay.Clarification{
		Key:      key,
		ID:       id,
		Project:  project,
		Role:     clarificationRole(item),
		Question: strings.TrimSpace(*item.QuestionExcerpt),
	}, nil
}

func clarificationIdentity(kind string, item AutomationClarification, fallbackProject string) (string, string, bool) {
	project := clarificationProject(item.Project, fallbackProject)
	id := clarificationID(kind, item)
	if project == "" || id == "" || item.QuestionExcerpt == nil {
		return "", "", false
	}
	return project, id, true
}

func clarificationProject(project, fallback string) string {
	if project == "" {
		return fallback
	}
	return project
}

func clarificationID(kind string, item AutomationClarification) string {
	if kind == "typed" && item.ClarificationID != "" {
		return item.ClarificationID
	}
	return item.ID
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
// {"version":1,"tested_at":"2026-11-16T17:34:12+01:00","module_hash":"5c941739ce3bced9e4ce3e19fb039d976dea93a517f542b6e4a62814607853c5","functions":[{"id":"func/Approvals.Identity","name":"Approvals.Identity","line":22,"end_line":24,"hash":"ac25347a37520e83449f41b897b3a6f1e69d76c4444179ad173ced8af74f30b9"},{"id":"func/Approvals.Snapshot","name":"Approvals.Snapshot","line":28,"end_line":30,"hash":"4a1e556c2f7e1f1242133b939c337b5ff866287e1f71e198ff7b0222a823a731"},{"id":"func/Clarifications.Identity","name":"Clarifications.Identity","line":34,"end_line":36,"hash":"cb041835b62b4126171f1814ad9250d89f53d87b98f8d999b4de4bbb5bf09eba"},{"id":"func/Clarifications.Snapshot","name":"Clarifications.Snapshot","line":40,"end_line":42,"hash":"c61862f227b035fd0c6ec6b9d4933ae6f507968ee4647ce763264cf65162ffd1"},{"id":"func/SnapshotSource.Identity","name":"SnapshotSource.Identity","line":47,"end_line":53,"hash":"54aa6ed417e42291276c89fbd0c0e85198c1105568e7363c2fba474f5079a55c"},{"id":"func/SnapshotSource.Snapshot","name":"SnapshotSource.Snapshot","line":57,"end_line":77,"hash":"5b757f34fd35f6fceaf25f31a08698786a33416352691c4ca90ab0c91901d6d4"},{"id":"func/projectionCompatibilityFallback","name":"projectionCompatibilityFallback","line":79,"end_line":86,"hash":"8078a055e6811884ff599e8540afafe317fef734b59fce15eabf9c7b82e2909e"},{"id":"func/compatibilitySnapshot","name":"compatibilitySnapshot","line":88,"end_line":107,"hash":"52aa804694a4a8c70121ec58202ae7552e5cd84bdb36bbd2e83f3bde4a130f26"},{"id":"func/automationSnapshot","name":"automationSnapshot","line":109,"end_line":129,"hash":"5bc949af5bd7c7b331cc210fe8b6c2c65a1c4cce5350549ffabab81b94f41014"},{"id":"func/automationApprovals","name":"automationApprovals","line":131,"end_line":139,"hash":"a8ca4edcf3cf41165691fe00d04c52dad3d43d3cb5a775b12b1a3242a024844b"},{"id":"func/automationResolvedApprovals","name":"automationResolvedApprovals","line":141,"end_line":147,"hash":"5e488ebeecc18e0aefde66c8449c64603830db3d5c784c085cf0551ec4bb193e"},{"id":"func/automationClarifications","name":"automationClarifications","line":154,"end_line":165,"hash":"227b52cefd6cd16f34defdb4ad652af7e5bc2f66ac1da671c44c08585b7af9ec"},{"id":"func/automationClarificationSection","name":"automationClarificationSection","line":167,"end_line":180,"hash":"cfbaabf2449e08b589cbd3a7555d6093f9c427808f04de8dd6b8454c686f6a2e"},{"id":"func/clarificationStatusMatches","name":"clarificationStatusMatches","line":182,"end_line":187,"hash":"e6cf970f83ccaa922453c62cb0a6cb975bfdef31428270a0eeaff38efc625857"},{"id":"func/automationClarification","name":"automationClarification","line":189,"end_line":206,"hash":"d42a0f7d1f5f608f191ce38f7de7fb29ee6b800b661132e97a198ab231a9e885"},{"id":"func/clarificationIdentity","name":"clarificationIdentity","line":208,"end_line":215,"hash":"8d9dc2b017815fce18cd4722276f06c56612f1b7eff8e35eb631aee4386d86f4"},{"id":"func/clarificationProject","name":"clarificationProject","line":217,"end_line":222,"hash":"017fe5fb9694256a6c576fcd8e6685fa015a15ebc3ded03915e7736714ac0c71"},{"id":"func/clarificationID","name":"clarificationID","line":224,"end_line":229,"hash":"3c7115a4183f81d309b022450b1bad5696d3ff412b3c265614d44c909644dd7a"},{"id":"func/clarificationRole","name":"clarificationRole","line":231,"end_line":237,"hash":"e7293dde67f3ed0e93060f9fff9a531817c6ebee68683032012bad8cf69317bf"},{"id":"func/automationBoards","name":"automationBoards","line":239,"end_line":255,"hash":"c124f96f4c30628711a318a3b8ff6e95127a6b24b612a2b3fc1779bad6480c75"}]}
// mutate4go-manifest-end
