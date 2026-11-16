package fixtures

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/unclebob/forgelet-bridge/internal/dashboard"
)

func TestFakeDashboardCountsStateRequestsAndBytes(t *testing.T) {
	fake := StartFakeDashboard()
	defer fake.Close()
	fake.SetState(automationState())

	for range 2 {
		response, err := http.Get(fake.URL() + "/api/automation-state")
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		if len(body) == 0 {
			t.Fatal("fake dashboard returned no state")
		}
	}
	counts := fake.Counts()
	if counts.Requests != 2 || counts.StateRequests != 0 || counts.Failed != 0 || counts.Bytes == 0 {
		t.Errorf("counts = %+v, want two successful projection requests with bytes", counts)
	}
}

func TestFakeDashboardCountsFailedStateReads(t *testing.T) {
	fake := StartFakeDashboard()
	defer fake.Close()
	fake.SetFailure(true)

	response, err := http.Get(fake.URL() + "/api/automation-state")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want service unavailable", response.StatusCode)
	}
	counts := fake.Counts()
	if counts.Requests != 1 || counts.Failed != 1 || counts.Bytes != 0 {
		t.Errorf("counts = %+v, want one failed read and no body", counts)
	}
}

func TestFakeDashboardServesUnsupportedCompatibilityState(t *testing.T) {
	fake := StartFakeDashboard()
	defer fake.Close()
	fake.SetUnsupportedWithFullState(dashboard.State{
		ResolvedApprovals: []dashboard.ResolvedApprovalRequest{{
			ID: "card-a", Project: "project-a", Resolution: "approved",
		}},
	})

	response, err := http.Get(fake.URL() + "/api/state")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var state dashboard.State
	if err := json.NewDecoder(response.Body).Decode(&state); err != nil {
		t.Fatal(err)
	}
	if len(state.ResolvedApprovals) != 1 ||
		state.ResolvedApprovals[0].Resolution != "approved" {
		t.Errorf("resolved approvals = %+v, want the compatibility resolution", state.ResolvedApprovals)
	}
}

func automationState() dashboard.AutomationState {
	return dashboard.AutomationState{
		Projection: dashboard.AutomationProjection,
		Identity: &dashboard.AutomationIdentity{
			Kind:         "family",
			Name:         "forge-a",
			AllProjects:  &dashboard.AutomationSection[string]{Items: []string{}},
			OpenProjects: &dashboard.AutomationSection[string]{Items: []string{}},
		},
		Approvals:         &dashboard.AutomationSection[dashboard.AutomationApproval]{Items: []dashboard.AutomationApproval{}},
		ResolvedApprovals: &dashboard.AutomationSection[dashboard.AutomationResolvedApproval]{Items: []dashboard.AutomationResolvedApproval{}},
		Clarifications: &dashboard.AutomationClarifications{
			Legacy: &dashboard.AutomationSection[dashboard.AutomationClarification]{Items: []dashboard.AutomationClarification{}},
			Typed:  &dashboard.AutomationSection[dashboard.AutomationClarification]{Items: []dashboard.AutomationClarification{}},
		},
		Projects: &dashboard.AutomationSection[dashboard.AutomationProject]{Items: []dashboard.AutomationProject{}},
	}
}
