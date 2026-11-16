package dashboard

import (
	"errors"
	"strings"
	"testing"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

func TestLegacyProjectionErrorIdentifiesTheResolvedApprovalRolloutGap(t *testing.T) {
	state := testAutomationState()
	state.ResolvedApprovals = testSection([]AutomationResolvedApproval{{
		ID: "old", Project: "project",
	}})
	err := validateAutomationState(state)
	var legacy LegacyProjectionError
	if !errors.As(err, &legacy) {
		t.Fatalf("error = %v, want LegacyProjectionError", err)
	}
	if !strings.Contains(legacy.Error(), "resolved-approval resolutions") {
		t.Errorf("error = %q, want the rollout gap", legacy.Error())
	}
}

func TestAutomationValidationRejectsEveryWrongField(t *testing.T) {
	question := "question?"
	tests := []struct {
		name      string
		change    func(*AutomationState)
		wantError bool
	}{
		{name: "valid", change: func(*AutomationState) {}},
		{name: "discriminator", change: func(state *AutomationState) { state.Projection = "other" }, wantError: true},
		{name: "identity missing", change: func(state *AutomationState) { state.Identity = nil }, wantError: true},
		{name: "all projects missing", change: func(state *AutomationState) { state.Identity.AllProjects = nil }, wantError: true},
		{name: "open projects missing", change: func(state *AutomationState) { state.Identity.OpenProjects = nil }, wantError: true},
		{name: "approvals missing", change: func(state *AutomationState) { state.Approvals = nil }, wantError: true},
		{name: "resolved approvals missing", change: func(state *AutomationState) { state.ResolvedApprovals = nil }, wantError: true},
		{name: "projects missing", change: func(state *AutomationState) { state.Projects = nil }, wantError: true},
		{name: "legacy clarifications missing", change: func(state *AutomationState) { state.Clarifications.Legacy = nil }, wantError: true},
		{name: "typed clarifications missing", change: func(state *AutomationState) { state.Clarifications.Typed = nil }, wantError: true},
		{name: "legacy pending id", change: func(state *AutomationState) {
			state.Clarifications.Legacy = testSection([]AutomationClarification{{
				Status: "pending", Role: "coder", QuestionExcerpt: &question,
			}})
		}, wantError: true},
		{name: "legacy pending role", change: func(state *AutomationState) {
			state.Clarifications.Legacy = testSection([]AutomationClarification{{
				ID: "clar-1", Status: "pending", QuestionExcerpt: &question,
			}})
		}, wantError: true},
		{name: "legacy pending question", change: func(state *AutomationState) {
			state.Clarifications.Legacy = testSection([]AutomationClarification{{
				ID: "clar-1", Status: "pending", Role: "coder",
			}})
		}, wantError: true},
		{name: "resolved legacy malformed is ignored", change: func(state *AutomationState) {
			state.Clarifications.Legacy = testSection([]AutomationClarification{{Status: "done"}})
		}},
		{name: "typed open id", change: func(state *AutomationState) {
			state.Clarifications.Typed = testSection([]AutomationClarification{{
				Status: "open", QuestionExcerpt: &question,
			}})
		}, wantError: true},
		{name: "typed open question", change: func(state *AutomationState) {
			state.Clarifications.Typed = testSection([]AutomationClarification{{
				ClarificationID: "clar-1", Status: "open",
			}})
		}, wantError: true},
		{name: "answered typed malformed is ignored", change: func(state *AutomationState) {
			state.Clarifications.Typed = testSection([]AutomationClarification{{Status: "answered"}})
		}},
		{name: "pending approval id", change: func(state *AutomationState) {
			state.Approvals = testSection([]AutomationApproval{{Project: "p", Card: "card"}})
		}, wantError: true},
		{name: "pending approval project", change: func(state *AutomationState) {
			state.Approvals = testSection([]AutomationApproval{{ID: "id", Card: "card"}})
		}, wantError: true},
		{name: "pending approval card", change: func(state *AutomationState) {
			state.Approvals = testSection([]AutomationApproval{{ID: "id", Project: "p"}})
		}, wantError: true},
		{name: "resolved approval id", change: func(state *AutomationState) {
			state.ResolvedApprovals = testSection([]AutomationResolvedApproval{{
				Project: "p", Resolution: relay.ResolutionApproved,
			}})
		}, wantError: true},
		{name: "resolved approval project", change: func(state *AutomationState) {
			state.ResolvedApprovals = testSection([]AutomationResolvedApproval{{
				ID: "id", Resolution: relay.ResolutionApproved,
			}})
		}, wantError: true},
		{name: "resolved approval ending", change: func(state *AutomationState) {
			state.ResolvedApprovals = testSection([]AutomationResolvedApproval{{
				ID: "id", Project: "p", Resolution: "guessed",
			}})
		}, wantError: true},
		{name: "resolved approval legacy shape", change: func(state *AutomationState) {
			state.ResolvedApprovals = testSection([]AutomationResolvedApproval{{
				ID: "id", Project: "p",
			}})
		}, wantError: true},
		{name: "resolved approval mixed legacy and current shape", change: func(state *AutomationState) {
			state.ResolvedApprovals = testSection([]AutomationResolvedApproval{
				{ID: "old", Project: "p"},
				{ID: "new", Project: "p", Resolution: relay.ResolutionApproved},
			})
		}, wantError: true},
		{name: "project name", change: func(state *AutomationState) {
			state.Projects = testSection([]AutomationProject{validationProject("")})
		}, wantError: true},
		{name: "project lanes", change: func(state *AutomationState) {
			project := validationProject("p")
			project.Lanes = nil
			state.Projects = testSection([]AutomationProject{project})
		}, wantError: true},
		{name: "project cards", change: func(state *AutomationState) {
			project := validationProject("p")
			project.Cards = nil
			state.Projects = testSection([]AutomationProject{project})
		}, wantError: true},
		{name: "project activity", change: func(state *AutomationState) {
			project := validationProject("p")
			project.Activity = nil
			state.Projects = testSection([]AutomationProject{project})
		}, wantError: true},
		{name: "card name", change: func(state *AutomationState) {
			project := validationProject("p")
			project.Cards = testSection([]AutomationCard{{Lane: "coder"}})
			state.Projects = testSection([]AutomationProject{project})
		}, wantError: true},
		{name: "card lane", change: func(state *AutomationState) {
			project := validationProject("p")
			project.Cards = testSection([]AutomationCard{{Name: "card"}})
			state.Projects = testSection([]AutomationProject{project})
		}, wantError: true},
		{name: "section count", change: func(state *AutomationState) {
			state.Approvals = &AutomationSection[AutomationApproval]{Count: 1}
		}, wantError: true},
		{name: "section has more items than count", change: func(state *AutomationState) {
			state.Approvals = &AutomationSection[AutomationApproval]{
				Count: 0, Truncated: true,
				Items: []AutomationApproval{{ID: "id", Project: "p", Card: "card"}},
			}
		}, wantError: true},
		{name: "section truncation", change: func(state *AutomationState) {
			state.Approvals = &AutomationSection[AutomationApproval]{
				Count: 2, Items: []AutomationApproval{{ID: "id", Project: "p", Card: "card"}},
			}
		}, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := testAutomationState()
			test.change(&state)
			err := validateAutomationState(state)
			if (err != nil) != test.wantError {
				t.Errorf("validateAutomationState = %v, wantError %t", err, test.wantError)
			}
		})
	}
}

func TestAutomationSectionRejectsNilAndOversize(t *testing.T) {
	if err := validateAutomationSection[AutomationApproval]("nil", nil, maxAutomationApprovals); err == nil {
		t.Error("nil section was accepted")
	}
	atLimit := &AutomationSection[int]{Count: maxAutomationApprovals, Items: make([]int, maxAutomationApprovals)}
	if err := validateAutomationSection("at limit", atLimit, maxAutomationApprovals); err != nil {
		t.Errorf("section at its limit failed: %v", err)
	}
	tooMany := &AutomationSection[int]{Count: maxAutomationApprovals + 1, Items: make([]int, maxAutomationApprovals+1)}
	if err := validateAutomationSection("too many", tooMany, maxAutomationApprovals); err == nil {
		t.Error("oversize section was accepted")
	}
}

func TestAutomationQuestionIsBoundedAndNonBlank(t *testing.T) {
	blank := " \n "
	atLimit := strings.Repeat("x", maxAutomationQuestion)
	tooLong := strings.Repeat("x", maxAutomationQuestion+1)
	for name, test := range map[string]struct {
		excerpt *string
		want    bool
	}{
		"nil":      {want: false},
		"blank":    {excerpt: &blank, want: false},
		"at limit": {excerpt: &atLimit, want: true},
		"too long": {excerpt: &tooLong, want: false},
	} {
		t.Run(name, func(t *testing.T) {
			if got := validQuestionExcerpt(test.excerpt); got != test.want {
				t.Errorf("validQuestionExcerpt = %t, want %t", got, test.want)
			}
		})
	}
}

func validationProject(name string) AutomationProject {
	return AutomationProject{
		Name:     name,
		Lanes:    testSection([]string{"coder"}),
		Cards:    testSection([]AutomationCard{{Name: "card", Lane: "coder"}}),
		Activity: testSection([]AutomationActivity{}),
	}
}
