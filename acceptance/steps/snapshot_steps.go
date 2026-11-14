package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/internal/bridge"
	"github.com/unclebob/forgelet-bridge/internal/dashboard"
	"github.com/unclebob/forgelet-bridge/internal/relay"
)

func bridgeChecksEverySecond(_ context.Context, world any, _ []string) error {
	world.(*World).bridgeInterval = time.Second
	return nil
}

func fakeDashboardAvailable(_ context.Context, world any, captures []string) error {
	return startFakeDashboard(world.(*World), captures[1])
}

func fakeDashboardsAvailable(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	for _, name := range captures[1:] {
		if err := startFakeDashboard(w, name); err != nil {
			return err
		}
	}
	return nil
}

func startFakeDashboard(w *World, name string) error {
	if _, ok := w.fakeDashboards[name]; ok {
		return nil
	}
	store, err := w.forge(context.Background(), name)
	if err != nil {
		return err
	}
	fake := fixtures.StartFakeDashboard()
	w.fakeDashboards[name] = fake
	w.dashboardURLs[store.Root()] = fake.URL()
	return nil
}

func fakeFirstServesPendingApproval(_ context.Context, world any, captures []string) error {
	fake, err := fakeDashboard(world.(*World), captures[1])
	if err != nil {
		return err
	}
	fake.SetFailure(false)
	fake.SetState(fixturesStateWithApproval(captures[2], captures[3]))
	return nil
}

func fakeNextServesState(_ context.Context, world any, captures []string) error {
	fake, err := fakeDashboard(world.(*World), captures[1])
	if err != nil {
		return err
	}
	fake.SetFailure(false)
	switch captures[2] {
	case "no waiting items":
		fake.SetState(dashboard.State{})
	case "a pending approval, a resolved approval, and a pending clarification":
		fake.SetState(fixturesFullState())
	default:
		return fmt.Errorf("the fake dashboard does not know the state %q", captures[2])
	}
	return nil
}

func fakeFailsStateReads(_ context.Context, world any, captures []string) error {
	fake, err := fakeDashboard(world.(*World), captures[1])
	if err != nil {
		return err
	}
	fake.SetFailure(true)
	return nil
}

func fakeProjectServesClarification(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	dashboardName, forgeName := captures[1], captures[2]
	project := strings.TrimPrefix(dashboardName, "project-")
	if project == dashboardName || project == "" {
		return fmt.Errorf("the fake project dashboard name %q does not name a project", dashboardName)
	}
	forge, err := w.declaredForge(forgeName)
	if err != nil {
		return err
	}
	root := filepath.Join(forge.Root(), "projects", project)
	if err := os.MkdirAll(filepath.Join(root, ".swarmforge"), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, ".swarmforge", "single-agent"), nil, 0o644); err != nil {
		return err
	}
	fake := fixtures.StartFakeDashboard()
	fake.SetState(dashboard.State{Clarifications: []dashboard.ClarificationRequest{{
		ID:     "clarification-" + project,
		Status: "pending",
		Role:   captures[3],
		Body:   captures[4],
	}}})
	if err := os.WriteFile(filepath.Join(root, dashboard.URLFile), []byte(fake.URL()+"\n"), 0o644); err != nil {
		fake.Close()
		return err
	}
	w.fakeDashboards[dashboardName] = fake
	w.dashboardURLs[root] = fake.URL()
	return addOpenProject(forge.Root(), project)
}

func fakeForgesNextServeApprovals(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	for index, name := range []string{"forge-a", "forge-b"} {
		fake, err := fakeDashboard(w, name)
		if err != nil {
			return err
		}
		card := "card-" + captures[index+1]
		fake.UpdateState(func(state dashboard.State) dashboard.State {
			state.Approvals = fixturesStateWithApproval(card, captures[index+1]).Approvals
			return state
		})
	}
	return nil
}

func fakeForgesNextServeClarifications(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	for index, name := range []string{"forge-a", "forge-b"} {
		fake, err := fakeDashboard(w, name)
		if err != nil {
			return err
		}
		project := captures[index+1]
		fake.UpdateState(func(state dashboard.State) dashboard.State {
			state.Clarifications = []dashboard.ClarificationRequest{{
				ID:      "clarification-" + project,
				Status:  "pending",
				Project: project,
				Role:    "coder",
				Body:    "which stage is next?",
			}}
			return state
		})
	}
	return nil
}

func approvalsRoomHolds(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomID, err := w.approvalsRoom(ctx)
	if err != nil {
		return err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	want := captures[1]
	return waitFor(ctx, fmt.Sprintf("the approvals room never held %s approval messages", want), func() (bool, error) {
		return fmt.Sprint(approvalMessagesIn(operator, roomID)) == want, nil
	})
}

func approvalCarriesResolution(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomID, messageID, _, err := w.approvalMessage(ctx, captures[1])
	if err != nil {
		return err
	}
	if captures[2] == "no bridge mark" {
		return roomCarriesNoBridgeReaction(ctx, w, roomID, messageID, "the approval")
	}
	return roomCarriesBridgeMark(ctx, w, roomID, messageID, captures[2], "the approval for "+captures[1])
}

func fakeServesAtMostOneStatePerTick(_ context.Context, world any, captures []string) error {
	return requireAtMostOne(world.(*World), []string{captures[1]}, false)
}

func fakeServesAtMostOneFailedStatePerTick(_ context.Context, world any, captures []string) error {
	return requireAtMostOne(world.(*World), []string{captures[1]}, true)
}

func fakeDashboardsServeAtMostOnePerTick(_ context.Context, world any, captures []string) error {
	return requireAtMostOne(world.(*World), namesIn(captures[1]), false)
}

func requireAtMostOne(w *World, names []string, failed bool) error {
	ctx, cancel := stepContext()
	defer cancel()
	beforeStatus, err := readStatus(filepath.Join(w.stateDir, bridge.StatusName))
	if err != nil {
		return err
	}
	before := map[string]fixtures.FakeDashboardCounts{}
	for _, name := range names {
		fake, err := fakeDashboard(w, name)
		if err != nil {
			return err
		}
		before[name] = fake.Counts()
	}
	if err := waitFor(ctx, "the bridge did not complete three more ticks", func() (bool, error) {
		status, err := readStatus(filepath.Join(w.stateDir, bridge.StatusName))
		if err != nil {
			return false, nil
		}
		return status.Tick >= beforeStatus.Tick+3, nil
	}); err != nil {
		return err
	}
	afterStatus, err := readStatus(filepath.Join(w.stateDir, bridge.StatusName))
	if err != nil {
		return err
	}
	ticks := afterStatus.Tick - beforeStatus.Tick
	for _, name := range names {
		fake, err := fakeDashboard(w, name)
		if err != nil {
			return err
		}
		after := fake.Counts()
		requests := after.Requests - before[name].Requests
		failedRequests := after.Failed - before[name].Failed
		if failed {
			requests = failedRequests
		}
		if requests == 0 {
			return fmt.Errorf("the fake dashboard %s served no state request over %d ticks", name, ticks)
		}
		if requests > ticks+1 {
			return fmt.Errorf("the fake dashboard %s served %d state requests over %d ticks", name, requests, ticks)
		}
	}
	return nil
}

func forgeApprovalRoomHoldsOnly(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	roomID, err := w.waitForSpaceChild(ctx, captures[1], "Approvals")
	if err != nil {
		return err
	}
	wanted, other := captures[2], captures[3]
	if err := waitFor(ctx, fmt.Sprintf("the approval room never showed %s", wanted), func() (bool, error) {
		return approvalRoomHas(operator, roomID, wanted), nil
	}); err != nil {
		return err
	}
	if err := fixtures.Sleep(ctx, 250*time.Millisecond); err != nil {
		return err
	}
	if approvalRoomHas(operator, roomID, other) {
		return fmt.Errorf("the approval room for %s also showed %s", captures[1], other)
	}
	return nil
}

func forgeClarificationsRoomHoldsOnly(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	roomID, err := w.waitForSpaceChild(ctx, captures[1], "Clarifications")
	if err != nil {
		return err
	}
	wanted, other := captures[2], captures[3]
	if err := waitFor(ctx, fmt.Sprintf("the clarifications room never showed %s", wanted), func() (bool, error) {
		return clarificationRoomHas(operator, roomID, wanted), nil
	}); err != nil {
		return err
	}
	if clarificationRoomHas(operator, roomID, other) {
		return fmt.Errorf("the clarifications room for %s also showed %s", captures[1], other)
	}
	return nil
}

func projectRoomCarriesClarification(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	project := captures[1]
	forgeName := ""
	for _, candidate := range w.forgeRoots {
		if _, err := os.Stat(filepath.Join(candidate, "projects", project)); err == nil {
			forgeName = filepath.Base(candidate)
			break
		}
	}
	if forgeName == "" {
		return fmt.Errorf("no forge root holds the project %s", project)
	}
	roomID, err := w.waitForSpaceChild(ctx, forgeName, "Project "+project)
	if err != nil {
		return err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	start := "Clarification for " + project + " from "
	return waitFor(ctx, fmt.Sprintf("the project room %s never carried its clarification", project), func() (bool, error) {
		for _, message := range operator.Messages(roomID) {
			if strings.HasPrefix(message.Body, start) {
				return true, nil
			}
		}
		return false, nil
	})
}

func fixturesStateWithApproval(card, project string) dashboard.State {
	return dashboard.State{Approvals: []dashboard.ApprovalRequest{{
		ID:      "approval-" + card,
		Project: project,
		Card:    card,
		TaskID:  "task-" + card,
		Gate:    "coder → refactorer",
		From:    "coder",
		To:      "refactorer",
	}}}
}

func fixturesFullState() dashboard.State {
	return dashboard.State{
		Approvals: []dashboard.ApprovalRequest{{
			ID:      "approval-card-snapshot",
			Project: "forgelet-bridge",
			Card:    "card-snapshot",
			TaskID:  "task-card-snapshot",
			Gate:    "coder → refactorer",
			From:    "coder",
			To:      "refactorer",
		}},
		ResolvedApprovals: []dashboard.ResolvedApprovalRequest{{
			ID:         "approval-card-a",
			Project:    "forgelet-bridge",
			Resolution: relay.ResolutionApproved,
		}},
		Clarifications: []dashboard.ClarificationRequest{{
			ID:      "clarification-snapshot",
			Status:  "pending",
			Project: "forgelet-bridge",
			Role:    "coder",
			Body:    "which stage is next?",
		}},
	}
}

func fakeDashboard(w *World, name string) (*fixtures.FakeDashboard, error) {
	fake, ok := w.fakeDashboards[name]
	if !ok {
		return nil, fmt.Errorf("the fake dashboard %s is not available", name)
	}
	return fake, nil
}

func namesIn(wording string) []string {
	replaced := strings.ReplaceAll(wording, " and ", ", ")
	var names []string
	for _, part := range strings.Split(replaced, ",") {
		if name := strings.TrimSpace(part); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func addOpenProject(forgeRoot, project string) error {
	path := filepath.Join(forgeRoot, ".swarmforge", "open-projects")
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, line := range strings.Split(string(existing), "\n") {
		if strings.TrimSpace(line) == project {
			return nil
		}
	}
	if len(existing) > 0 && existing[len(existing)-1] != '\n' {
		existing = append(existing, '\n')
	}
	return os.WriteFile(path, append(existing, []byte(project+"\n")...), 0o644)
}

func approvalMessagesIn(operator *fixtures.User, roomID string) int {
	found := 0
	for _, message := range operator.Messages(roomID) {
		if strings.HasPrefix(message.Body, "Approval for ") {
			found++
		}
	}
	return found
}

func approvalRoomHas(operator *fixtures.User, roomID, card string) bool {
	start := "Approval for " + card + " in "
	for _, message := range operator.Messages(roomID) {
		if strings.HasPrefix(message.Body, start) {
			return true
		}
	}
	return false
}

func clarificationRoomHas(operator *fixtures.User, roomID, project string) bool {
	start := "Clarification for " + project + " from "
	for _, message := range operator.Messages(roomID) {
		if strings.HasPrefix(message.Body, start) {
			return true
		}
	}
	return false
}
