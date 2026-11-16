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
	if sets.legacy, err = automationClarificationSection(root, "legacy", sections.Legacy.Items, project, true, seen); err != nil {
		return sets, err
	}
	if sets.typed, err = automationClarificationSection(root, "typed", sections.Typed.Items, project, false, seen); err != nil {
		return sets, err
	}
	return sets, nil
}

func automationClarificationSection(root, kind string, items []AutomationClarification, fallbackProject string, pendingOnly bool, seen map[string]bool) ([]relay.Clarification, error) {
	var pending []relay.Clarification
	for _, item := range items {
		if pendingOnly && item.Status != "pending" {
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

func automationClarification(root, kind string, item AutomationClarification, fallbackProject string, seen map[string]bool) (relay.Clarification, error) {
	project, id, ok := clarificationIdentity(kind, item, fallbackProject)
	if !ok {
		return relay.Clarification{}, fmt.Errorf("invalid %s clarification in automation projection", kind)
	}
	identity := project + "\x00" + id
	if seen[identity] {
		return relay.Clarification{}, fmt.Errorf("duplicate or colliding clarification in automation projection")
	}
	seen[identity] = true
	clarification := relay.Clarification{
		Key:       forgeKey(root, project, id),
		ID:        id,
		Project:   project,
		Role:      clarificationRole(item),
		Question:  strings.TrimSpace(*item.QuestionExcerpt),
		Lifecycle: item.Status,
		Attention: item.Attention,
	}
	if kind == relay.ClarificationSourceTyped {
		clarification.Key = typedClarificationKey(root, project, id)
		clarification.Source = relay.ClarificationSourceTyped
		clarification.Notify = item.Status == relay.ClarificationOpen && item.Attention == "unseen"
	}
	return clarification, nil
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
	if kind == relay.ClarificationSourceTyped && item.ClarificationID != "" {
		return item.ClarificationID
	}
	return item.ID
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
// {"version":1,"tested_at":"2026-11-16T18:48:42+01:00","module_hash":"eae4842d1419777ede0d0af1906984dcd676650eb0bb396a1b4b8877585526a9","functions":[{"id":"func/Approvals.Identity","name":"Approvals.Identity","line":22,"end_line":24,"hash":"ac25347a37520e83449f41b897b3a6f1e69d76c4444179ad173ced8af74f30b9"},{"id":"func/Approvals.Snapshot","name":"Approvals.Snapshot","line":28,"end_line":30,"hash":"4a1e556c2f7e1f1242133b939c337b5ff866287e1f71e198ff7b0222a823a731"},{"id":"func/Clarifications.Identity","name":"Clarifications.Identity","line":34,"end_line":36,"hash":"cb041835b62b4126171f1814ad9250d89f53d87b98f8d999b4de4bbb5bf09eba"},{"id":"func/Clarifications.Snapshot","name":"Clarifications.Snapshot","line":40,"end_line":42,"hash":"c61862f227b035fd0c6ec6b9d4933ae6f507968ee4647ce763264cf65162ffd1"},{"id":"func/SnapshotSource.Identity","name":"SnapshotSource.Identity","line":47,"end_line":53,"hash":"54aa6ed417e42291276c89fbd0c0e85198c1105568e7363c2fba474f5079a55c"},{"id":"func/SnapshotSource.Snapshot","name":"SnapshotSource.Snapshot","line":57,"end_line":77,"hash":"5b757f34fd35f6fceaf25f31a08698786a33416352691c4ca90ab0c91901d6d4"},{"id":"func/projectionCompatibilityFallback","name":"projectionCompatibilityFallback","line":79,"end_line":86,"hash":"8078a055e6811884ff599e8540afafe317fef734b59fce15eabf9c7b82e2909e"},{"id":"func/compatibilitySnapshot","name":"compatibilitySnapshot","line":88,"end_line":107,"hash":"52aa804694a4a8c70121ec58202ae7552e5cd84bdb36bbd2e83f3bde4a130f26"},{"id":"func/automationSnapshot","name":"automationSnapshot","line":109,"end_line":129,"hash":"5bc949af5bd7c7b331cc210fe8b6c2c65a1c4cce5350549ffabab81b94f41014"},{"id":"func/automationApprovals","name":"automationApprovals","line":131,"end_line":139,"hash":"a8ca4edcf3cf41165691fe00d04c52dad3d43d3cb5a775b12b1a3242a024844b"},{"id":"func/automationResolvedApprovals","name":"automationResolvedApprovals","line":141,"end_line":147,"hash":"5e488ebeecc18e0aefde66c8449c64603830db3d5c784c085cf0551ec4bb193e"},{"id":"func/automationClarifications","name":"automationClarifications","line":154,"end_line":165,"hash":"8392569bb509a4177570a9cb3a9bab495cd86fb00bd9058a315cad0bffd72338"},{"id":"func/automationClarificationSection","name":"automationClarificationSection","line":167,"end_line":180,"hash":"d1012cc20dcc3361ea5f48537d7eaf7ea6442bb85b19a8e6031ec263df60e23b"},{"id":"func/automationClarification","name":"automationClarification","line":182,"end_line":207,"hash":"4c3ce97b29dd595175d17a2c8ae6cbbdea34fb7af4b688289b33eebc42201474"},{"id":"func/clarificationIdentity","name":"clarificationIdentity","line":209,"end_line":216,"hash":"8d9dc2b017815fce18cd4722276f06c56612f1b7eff8e35eb631aee4386d86f4"},{"id":"func/clarificationProject","name":"clarificationProject","line":218,"end_line":223,"hash":"017fe5fb9694256a6c576fcd8e6685fa015a15ebc3ded03915e7736714ac0c71"},{"id":"func/clarificationID","name":"clarificationID","line":225,"end_line":230,"hash":"bade3f472a59542386091c45639811e6a1468fe2f23c6021e243a56a551daeb1"},{"id":"func/typedClarificationKey","name":"typedClarificationKey","line":232,"end_line":234,"hash":"268c4955f00f25d78a65e2b80036b9d18a454ea08efb0d0273d659df27b32cb9"},{"id":"func/clarificationRole","name":"clarificationRole","line":236,"end_line":242,"hash":"e7293dde67f3ed0e93060f9fff9a531817c6ebee68683032012bad8cf69317bf"},{"id":"func/automationBoards","name":"automationBoards","line":244,"end_line":260,"hash":"c124f96f4c30628711a318a3b8ff6e95127a6b24b612a2b3fc1779bad6480c75"}]}
// mutate4go-manifest-end
