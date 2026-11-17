package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/internal/dashboard"
	"github.com/unclebob/forgelet-bridge/internal/relay"
	"github.com/unclebob/forgelet-bridge/internal/state"
)

func fakeNextServesClarificationProjection(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	fake, err := fakeDashboard(w, captures[1])
	if err != nil {
		return err
	}
	fake.SetFailure(false)
	state := automationState()
	question := "which stage?"
	typed := dashboard.AutomationClarification{
		ID: "typed-request", ClarificationID: "clarification-projection",
		Status: "open", Attention: "unseen", Project: "project-alpha",
		Role: "coder", QuestionExcerpt: &question,
	}
	switch captures[2] {
	case "typed":
		state.Clarifications.Typed = automationSection([]dashboard.AutomationClarification{typed})
	case "legacy":
		state.Clarifications.Legacy = automationSection([]dashboard.AutomationClarification{{
			ID: "legacy-projection", Status: "pending", Project: "project-legacy",
			Role: "coder", QuestionExcerpt: &question,
		}})
	case "mixed":
		state.Clarifications.Legacy = automationSection([]dashboard.AutomationClarification{{
			ID: "legacy-projection", Status: "pending", Project: "project-legacy",
			Role: "coder", QuestionExcerpt: &question,
		}})
		state.Clarifications.Typed = automationSection([]dashboard.AutomationClarification{typed})
	case "malformed":
		typed.Status = "closed"
		state.Clarifications.Typed = automationSection([]dashboard.AutomationClarification{typed})
	case "unsupported":
		fake.SetUnsupported(true)
		return nil
	default:
		return fmt.Errorf("the fake dashboard does not know the clarification projection %q", captures[2])
	}
	fake.SetState(state)
	return nil
}

func bridgeLastErrorIsBounded(_ context.Context, world any, captures []string) error {
	status, err := readStatus(filepath.Join(world.(*World).stateDir, "status.json"))
	if err != nil {
		return err
	}
	switch captures[1] {
	case "empty":
		if status.LastError != "" {
			return fmt.Errorf("last error = %q, want empty", status.LastError)
		}
	case "bounded legacy clarification error":
		if !strings.Contains(status.LastError, "unsupported legacy clarification work") {
			return fmt.Errorf("last error = %q, want the bounded legacy clarification error", status.LastError)
		}
	case "bounded malformed projection error":
		if !strings.Contains(status.LastError, "invalid typed clarification") {
			return fmt.Errorf("last error = %q, want the bounded malformed projection error", status.LastError)
		}
	case "bounded unsupported projection error":
		if !strings.Contains(status.LastError, "automation projection is unsupported") {
			return fmt.Errorf("last error = %q, want the bounded unsupported projection error", status.LastError)
		}
	default:
		return fmt.Errorf("the bridge last-error check does not know %q", captures[1])
	}
	return nil
}

func bridgeStatusForForgeMatches(_ context.Context, world any, captures []string) error {
	status, err := readStatus(filepath.Join(world.(*World).stateDir, "status.json"))
	if err != nil {
		return err
	}
	var wantReached bool
	switch captures[2] {
	case "reached":
		wantReached = true
	case "did not reach":
	default:
		return fmt.Errorf("the bridge status match %q is not reached or did not reach", captures[2])
	}
	if fixtures.Contains(status.ReachedForges, captures[1]) != wantReached {
		return fmt.Errorf("reached forges = %v, want %s %s", status.ReachedForges,
			captures[1], map[bool]string{true: "reached", false: "not reached"}[wantReached])
	}
	return nil
}

func bridgeStateHasOldAndTypedClarificationMarks(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	forge, err := w.declaredForge("forge-a")
	if err != nil {
		return err
	}
	path := filepath.Join(w.stateDir, "bridge-state.json")
	loaded, err := state.Load(path)
	if err != nil {
		return err
	}
	loaded.Relay.Clarifications["/forges/forge-a/project-legacy/legacy-old"] = relay.ClarificationState{
		ForgeRoot: forge.Root(),
		MessageID: "$legacy-history",
	}
	loaded.Relay.Clarifications["/forges/forge-a/typed/project-alpha/typed-old"] = relay.ClarificationState{
		ForgeRoot: forge.Root(),
		MessageID: "$typed-history",
		Source:    relay.ClarificationSourceTyped,
	}
	return loaded.Save(path)
}

func bridgeStateHoldsOnlyTheTypedClarificationMark(_ context.Context, world any, captures []string) error {
	loaded, err := state.Load(filepath.Join(world.(*World).stateDir, "bridge-state.json"))
	if err != nil {
		return err
	}
	found := 0
	for key, clarification := range loaded.Relay.Clarifications {
		if !strings.Contains(key, "/"+captures[1]+"/") {
			continue
		}
		if clarification.Source != relay.ClarificationSourceTyped || clarification.MessageID == "" {
			return fmt.Errorf("record %s = %+v, want only a typed notification mark", key, clarification)
		}
		found++
	}
	if found != 1 {
		return fmt.Errorf("typed clarification marks for %s = %d, want one", captures[1], found)
	}
	return nil
}

func fakeNextServesTypedClarification(_ context.Context, world any, captures []string) error {
	fake, err := fakeDashboard(world.(*World), captures[1])
	if err != nil {
		return err
	}
	clarification := typedClarification(captures[2], captures[3], "coder", captures[4], "which stage is next?")
	clarification.Attention = captures[5]
	return setTypedClarifications(fake, []dashboard.AutomationClarification{clarification}, false)
}

func fakeAlsoServesTypedClarification(_ context.Context, world any, captures []string) error {
	fake, err := fakeDashboard(world.(*World), captures[1])
	if err != nil {
		return err
	}
	fake.UpdateState(func(state dashboard.AutomationState) dashboard.AutomationState {
		ensureAutomationClarifications(&state)
		clarification := typedClarification(captures[2], captures[3], "coder", captures[4], "which stage is next?")
		clarification.Attention = captures[5]
		state.Clarifications.Typed.Items = append(state.Clarifications.Typed.Items, clarification)
		state.Clarifications.Typed.Count = len(state.Clarifications.Typed.Items)
		state.Clarifications.Typed.Truncated = false
		return state
	})
	return nil
}

func fakeProjectServesTypedAnswerSource(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	project := captures[1]
	if !strings.HasPrefix(project, "project-") {
		return fmt.Errorf("the fake project dashboard name %q does not name a project", project)
	}
	forge, err := w.declaredForge(captures[2])
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
	fake.SetState(automationState())
	if err := setTypedClarifications(fake, []dashboard.AutomationClarification{
		typedClarification(captures[3], project, "coder", "open", captures[4]),
	}, false); err != nil {
		fake.Close()
		return err
	}
	fake.UpdateState(func(state dashboard.AutomationState) dashboard.AutomationState {
		state.Identity.Kind = "project"
		state.Identity.Name = project
		state.Identity.AllProjects = automationSection([]string{project})
		state.Identity.OpenProjects = automationSection([]string{project})
		state.Projects = automationSection([]dashboard.AutomationProject{{
			Name:     project,
			Lanes:    automationSection([]string{"coder"}),
			Cards:    automationSection([]dashboard.AutomationCard{}),
			Activity: automationSection([]dashboard.AutomationActivity{}),
		}})
		return state
	})
	if err := os.WriteFile(filepath.Join(root, dashboard.URLFile), []byte(fake.URL()+"\n"), 0o644); err != nil {
		fake.Close()
		return err
	}
	w.fakeDashboards[captures[1]] = fake
	w.dashboardURLs[root] = fake.URL()
	return addOpenProject(forge.Root(), project)
}

func fakeServesMalformedTypedClarification(_ context.Context, world any, captures []string) error {
	fake, err := fakeDashboard(world.(*World), captures[1])
	if err != nil {
		return err
	}
	valid := typedClarification("clarification-invalid", "project-alpha", "coder", "open", "which stage?")
	typed := []dashboard.AutomationClarification{valid}
	truncated := false
	switch captures[2] {
	case "invalid lifecycle":
		typed[0].Status = "closed"
	case "invalid attention":
		typed[0].Attention = "unread"
	case "a missing clarification id":
		typed[0].ClarificationID = ""
	case "a blank question excerpt":
		blank := " \t "
		typed[0].QuestionExcerpt = &blank
	case "duplicate ids in one project":
		typed = append(typed, valid)
	case "a legacy collision":
		fake.UpdateState(func(state dashboard.AutomationState) dashboard.AutomationState {
			ensureAutomationClarifications(&state)
			question := "which stage?"
			state.Clarifications.Legacy.Items = []dashboard.AutomationClarification{{
				ID: valid.ClarificationID, Status: "pending", Project: valid.Project,
				Role: "coder", QuestionExcerpt: &question,
			}}
			state.Clarifications.Legacy.Count = 1
			return state
		})
	case "a truncated typed section":
		truncated = true
	default:
		return fmt.Errorf("the fake dashboard does not know the typed clarification problem %q", captures[2])
	}
	return setTypedClarifications(fake, typed, truncated)
}

func fakeNextServesLegacyAndTypedClarification(_ context.Context, world any, captures []string) error {
	fake, err := fakeDashboard(world.(*World), captures[1])
	if err != nil {
		return err
	}
	question := "which stage?"
	fake.UpdateState(func(state dashboard.AutomationState) dashboard.AutomationState {
		ensureAutomationClarifications(&state)
		state.Clarifications.Legacy.Items = []dashboard.AutomationClarification{{
			ID: "legacy-clarification", Status: "pending", Project: "project-legacy",
			Role: "coder", QuestionExcerpt: &question,
		}}
		state.Clarifications.Legacy.Count = 1
		state.Clarifications.Typed.Items = []dashboard.AutomationClarification{
			typedClarification("typed-clarification", "project-alpha", "coder", "open", question),
		}
		state.Clarifications.Typed.Count = 1
		state.Clarifications.Typed.Truncated = false
		return state
	})
	return nil
}

func fakeServesRedactedTypedClarification(_ context.Context, world any, captures []string) error {
	fake, err := fakeDashboard(world.(*World), captures[1])
	if err != nil {
		return err
	}
	return setTypedClarifications(fake, []dashboard.AutomationClarification{
		typedClarification("clarification-safe", captures[2], "coder", "open", "which safe stage?"),
	}, false)
}

func fakeFamiliesServeTheSameTypedClarification(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	for index, name := range []string{"forge-a", "forge-b"} {
		fake, err := fakeDashboard(w, name)
		if err != nil {
			return err
		}
		project := []string{"project-alpha", "project-beta"}[index]
		if err := setTypedClarifications(fake, []dashboard.AutomationClarification{
			typedClarification("clarification-shared", project, "coder", "open", "which stage?"),
		}, false); err != nil {
			return err
		}
	}
	return nil
}

func setTypedClarifications(fake *fixtures.FakeDashboard, typed []dashboard.AutomationClarification, truncated bool) error {
	fake.SetFailure(false)
	fake.UpdateState(func(state dashboard.AutomationState) dashboard.AutomationState {
		ensureAutomationClarifications(&state)
		state.Clarifications.Typed = &dashboard.AutomationSection[dashboard.AutomationClarification]{
			Count: len(typed), Truncated: truncated, Items: typed,
		}
		return state
	})
	return nil
}

func ensureAutomationClarifications(state *dashboard.AutomationState) {
	if state.Clarifications == nil {
		state.Clarifications = &dashboard.AutomationClarifications{}
	}
	if state.Clarifications.Legacy == nil {
		state.Clarifications.Legacy = automationSection([]dashboard.AutomationClarification{})
	}
	if state.Clarifications.Typed == nil {
		state.Clarifications.Typed = automationSection([]dashboard.AutomationClarification{})
	}
}

func typedClarification(id, project, role, lifecycle, question string) dashboard.AutomationClarification {
	return dashboard.AutomationClarification{
		ID: "request-" + id, ClarificationID: id, Project: project, Role: role,
		Status: lifecycle, Attention: "unseen", QuestionExcerpt: &question,
	}
}
