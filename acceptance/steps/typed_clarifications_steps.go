package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/internal/dashboard"
)

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
