package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

func TestSnapshotSourceCarriesEveryConsumerFromOneAutomationRead(t *testing.T) {
	state := testAutomationState()
	state.Approvals = testSection([]AutomationApproval{{
		ID: "approval-1", Project: "forgelet-bridge", Card: "snapshot-card", Gate: "coder → refactorer",
	}})
	state.ResolvedApprovals = testSection([]AutomationResolvedApproval{{
		ID: "approval-old", Project: "forgelet-bridge", Card: "old-card", Resolution: relay.ResolutionApproved,
	}})
	legacy := testText
	typed := "typed clarification question"
	state.Clarifications.Legacy = testSection([]AutomationClarification{{
		ID: "clarification-1", Status: "pending", Project: "forgelet-bridge", Role: "coder", QuestionExcerpt: &legacy,
	}})
	state.Clarifications.Typed = testSection([]AutomationClarification{{
		ID: "typed-request", ClarificationID: "clarification-typed", Status: "open",
		Project: "forgelet-bridge", Attention: "unseen", QuestionExcerpt: &typed,
	}})
	state.Projects = testSection([]AutomationProject{{
		Name:     "forgelet-bridge",
		Lanes:    testSection([]string{"master", "coder"}),
		Cards:    testSection([]AutomationCard{{Name: "snapshot-card", Lane: "coder"}}),
		Activity: testSection([]AutomationActivity{}),
	}})
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		if request.Method != http.MethodGet || request.URL.Path != "/api/automation-state" {
			t.Errorf("request = %s %s, want one automation read", request.Method, request.URL.Path)
		}
		writer.Header().Set("X-SwarmForge-Projection", AutomationProjection)
		_ = json.NewEncoder(writer).Encode(state)
	}))
	defer server.Close()

	source := SnapshotSource{Root: "/forges/forge-a", ConfiguredURL: server.URL, Project: "project-a"}
	snapshot, err := source.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if requests != 1 {
		t.Fatalf("automation requests = %d, want one", requests)
	}
	if len(snapshot.Approvals) != 1 || snapshot.Approvals[0].Key != "/forges/forge-a/forgelet-bridge/approval-1" {
		t.Errorf("approvals = %+v, want the pending approval once", snapshot.Approvals)
	}
	if got := snapshot.ResolvedApprovals["/forges/forge-a/forgelet-bridge/approval-old"]; got != relay.ResolutionApproved {
		t.Errorf("resolved approval = %q, want approved", got)
	}
	if len(snapshot.Clarifications) != 1 || snapshot.Clarifications[0].Question != testText {
		t.Errorf("clarifications = %+v, want only the pending legacy clarification", snapshot.Clarifications)
	}
	if len(snapshot.TypedClarifications) != 1 ||
		snapshot.TypedClarifications[0].ID != "clarification-typed" ||
		snapshot.TypedClarifications[0].Question != typed {
		t.Errorf("typed clarifications = %+v, want the safe typed question", snapshot.TypedClarifications)
	}
	if typed := snapshot.TypedClarifications[0]; typed.Source != relay.ClarificationSourceTyped ||
		typed.Lifecycle != relay.ClarificationOpen || typed.Attention != "unseen" || !typed.Notify {
		t.Errorf("typed clarification = %+v, want its notification identity and lifecycle", typed)
	}
	if len(snapshot.Boards) != 1 || len(snapshot.Boards[0].Cards) != 1 ||
		snapshot.Boards[0].Cards[0].Key != "/forges/forge-a/forgelet-bridge/snapshot-card" {
		t.Errorf("boards = %+v, want the projection card", snapshot.Boards)
	}
	if snapshot.Boards[0].Cards[0].Done {
		t.Error("a coder card was marked done")
	}
}

func TestSnapshotPreservesEveryAuthoritativeResolution(t *testing.T) {
	for _, resolution := range []string{
		relay.ResolutionApproved,
		relay.ResolutionSentBack,
		relay.ResolutionAutomaticSpecApproval,
		relay.ResolutionAutoApproved,
	} {
		state := testAutomationState()
		state.ResolvedApprovals = testSection([]AutomationResolvedApproval{{
			ID: "approval-1", Project: "forgelet-bridge", Card: "card", Resolution: resolution,
		}})
		server := automationServer(t, state, AutomationProjection)
		snapshot, err := (SnapshotSource{Root: "/forges/forge-a", ConfiguredURL: server.URL}).Snapshot(context.Background())
		server.Close()
		if err != nil {
			t.Fatalf("%s: Snapshot: %v", resolution, err)
		}
		if got := snapshot.ResolvedApprovals["/forges/forge-a/forgelet-bridge/approval-1"]; got != resolution {
			t.Errorf("resolution = %q, want %q", got, resolution)
		}
	}
}

func TestSnapshotNamesTheProjectWhenTheProjectionDoesNot(t *testing.T) {
	state := projectAutomationState()
	question := "next?"
	state.Clarifications.Legacy = testSection([]AutomationClarification{{
		ID: "clar-1", Status: "pending", Role: "coder", QuestionExcerpt: &question,
	}})
	server := automationServer(t, state, AutomationProjection)
	defer server.Close()
	snapshot, err := (SnapshotSource{Root: "/forges/project-a", ConfiguredURL: server.URL, Project: "project-a"}).
		Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(snapshot.Clarifications) != 1 || snapshot.Clarifications[0].Project != "project-a" {
		t.Errorf("clarifications = %+v, want the source project", snapshot.Clarifications)
	}
}

func TestSnapshotRejectsMalformedProjections(t *testing.T) {
	tests := []struct {
		name   string
		header string
		change func(*AutomationState)
	}{
		{name: "header", header: "dashboard-state-v1"},
		{name: "identity", header: AutomationProjection, change: func(state *AutomationState) { state.Identity = nil }},
		{name: "resolution", header: AutomationProjection, change: func(state *AutomationState) {
			state.ResolvedApprovals = testSection([]AutomationResolvedApproval{{ID: "one", Project: "p", Resolution: "guessed"}})
		}},
		{name: "clarification excerpt", header: AutomationProjection, change: func(state *AutomationState) {
			state.Clarifications.Legacy = testSection([]AutomationClarification{{ID: "clar-1", Status: "pending", Role: "coder"}})
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := testAutomationState()
			if test.change != nil {
				test.change(&state)
			}
			server := automationServer(t, state, test.header)
			defer server.Close()
			if _, err := (SnapshotSource{Root: "/forges/forge-a", ConfiguredURL: server.URL}).Snapshot(context.Background()); err == nil {
				t.Fatal("Snapshot accepted a malformed projection")
			}
		})
	}
}

func TestSnapshotReportsAnUnsupportedAutomationEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "Not found", http.StatusNotFound)
	}))
	defer server.Close()
	_, err := (SnapshotSource{Root: "/forges/forge-a", ConfiguredURL: server.URL}).Snapshot(context.Background())
	if _, ok := err.(interface{ Unwrap() error }); ok {
		err = err.(interface{ Unwrap() error }).Unwrap()
	}
	if !containsUnsupported(err) {
		t.Fatalf("error = %v, want unsupported automation projection", err)
	}
}

func TestSnapshotUsesBoundedStateForLegacyAutomationProjection(t *testing.T) {
	state := testAutomationState()
	state.ResolvedApprovals = testSection([]AutomationResolvedApproval{{
		ID: "approval-old", Project: "project-a", Card: "old-card",
	}})
	requests := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests[request.URL.Path]++
		switch request.URL.Path {
		case "/api/automation-state":
			writer.Header().Set("X-SwarmForge-Projection", AutomationProjection)
			_ = json.NewEncoder(writer).Encode(state)
		case "/api/state":
			_ = json.NewEncoder(writer).Encode(State{
				Approvals: []ApprovalRequest{{
					ID: "approval-1", Project: "project-a", Card: "new-card", Gate: "coder → refactorer",
				}},
				ResolvedApprovals: []ResolvedApprovalRequest{{
					ID: "approval-old", Project: "project-a", Resolution: relay.ResolutionSentBack,
				}},
				Clarifications: []ClarificationRequest{{
					ID: "clarification-1", Status: "pending", Role: "coder", Body: testText,
				}},
			})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	snapshot, err := (SnapshotSource{
		Root: "/forges/project-a", ConfiguredURL: server.URL, Project: "project-a",
	}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if requests["/api/automation-state"] != 1 || requests["/api/state"] != 1 {
		t.Fatalf("requests = %v, want one legacy projection and one bounded state read", requests)
	}
	if got := snapshot.ResolvedApprovals["/forges/project-a/project-a/approval-old"]; got != relay.ResolutionSentBack {
		t.Errorf("resolved approval = %q, want sent_back", got)
	}
	if len(snapshot.Clarifications) != 1 || snapshot.Clarifications[0].Project != "project-a" {
		t.Errorf("clarifications = %+v, want the source project", snapshot.Clarifications)
	}
	if !snapshot.BoardFallback || !snapshot.ProjectionFallback || snapshot.Boards != nil {
		t.Errorf("boards = %+v, board fallback = %t, projection fallback = %t, want the compatibility path",
			snapshot.Boards, snapshot.BoardFallback, snapshot.ProjectionFallback)
	}
}

func TestSnapshotUsesBoundedStateWhenAutomationIsUnsupported(t *testing.T) {
	requests := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests[request.URL.Path]++
		if request.URL.Path == "/api/automation-state" {
			http.Error(writer, "Not found", http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(writer).Encode(State{
			Approvals: []ApprovalRequest{{ID: "approval-1", Project: "forge-a", Card: "card"}},
		})
	}))
	defer server.Close()

	snapshot, err := (SnapshotSource{Root: "/forges/forge-a", ConfiguredURL: server.URL}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if requests["/api/automation-state"] != 1 || requests["/api/state"] != 1 {
		t.Fatalf("requests = %v, want one unsupported projection and one bounded state read", requests)
	}
	if len(snapshot.Approvals) != 1 {
		t.Errorf("approvals = %+v, want the bounded state approval", snapshot.Approvals)
	}
	if !snapshot.ProjectionFallback {
		t.Error("unsupported automation did not report its compatibility path")
	}
}

func TestSnapshotDoesNotFallbackForAnUnknownResolution(t *testing.T) {
	state := testAutomationState()
	state.ResolvedApprovals = testSection([]AutomationResolvedApproval{{
		ID: "approval-1", Project: "project-a", Resolution: "guessed",
	}})
	stateRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/api/state" {
			stateRequests++
			return
		}
		writer.Header().Set("X-SwarmForge-Projection", AutomationProjection)
		_ = json.NewEncoder(writer).Encode(state)
	}))
	defer server.Close()

	if _, err := (SnapshotSource{Root: "/forges/forge-a", ConfiguredURL: server.URL}).Snapshot(context.Background()); err == nil {
		t.Fatal("Snapshot accepted an unknown resolution")
	}
	if stateRequests != 0 {
		t.Fatalf("state requests = %d, want malformed projection to fail without fallback", stateRequests)
	}
}

func TestAutomationClarificationIdentityFailsClosed(t *testing.T) {
	question := "question?"
	valid := AutomationClarification{
		ID: "clar-1", Status: "pending", Project: "project",
		Role: "coder", QuestionExcerpt: &question,
	}
	project, id, ok := clarificationIdentity("legacy", valid, "")
	if !ok || project != "project" || id != "clar-1" {
		t.Fatalf("identity = %q/%q/%t, want the valid clarification", project, id, ok)
	}

	for name, test := range map[string]AutomationClarification{
		"missing project": {
			ID: "clar-1", QuestionExcerpt: &question,
		},
		"missing id": {
			Project: "project", QuestionExcerpt: &question,
		},
		"missing question": {
			ID: "clar-1", Project: "project",
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, ok := clarificationIdentity("legacy", test, ""); ok {
				t.Error("incomplete clarification identity was accepted")
			}
		})
	}

	if got := clarificationID("typed", AutomationClarification{ID: "message", ClarificationID: "typed"}); got != "typed" {
		t.Errorf("typed id = %q, want clarification id", got)
	}
	if got := clarificationID("typed", AutomationClarification{ID: "message"}); got != "message" {
		t.Errorf("typed fallback id = %q, want message id", got)
	}
	if got := clarificationID("legacy", AutomationClarification{ID: "message", ClarificationID: "other"}); got != "message" {
		t.Errorf("legacy id = %q, want message id", got)
	}
	if got := clarificationRole(AutomationClarification{Role: "coder"}); got != "coder" {
		t.Errorf("role = %q, want coder", got)
	}
	if got := clarificationRole(AutomationClarification{}); got != "project-agent" {
		t.Errorf("role = %q, want project-agent", got)
	}
}

func TestAutomationClarificationsRejectDuplicateAndMalformedTypedRows(t *testing.T) {
	question := "question?"
	sections := &AutomationClarifications{
		Legacy: testSection([]AutomationClarification{{
			ID: "clar-1", Status: "pending", Project: "project",
			Role: "coder", QuestionExcerpt: &question,
		}}),
		Typed: testSection([]AutomationClarification{{
			ID: "request-1", ClarificationID: "clar-1", Status: "open",
			Project: "project", QuestionExcerpt: &question,
		}}),
	}
	if _, err := automationClarifications("/forge", sections, ""); err == nil {
		t.Fatal("duplicate legacy and typed clarification was accepted")
	}

	sections.Legacy = testSection([]AutomationClarification{})
	sections.Typed = testSection([]AutomationClarification{{
		ID: "request-1", ClarificationID: "clar-1", Status: "open",
		QuestionExcerpt: &question,
	}})
	if _, err := automationClarifications("/forge", sections, ""); err == nil {
		t.Fatal("typed clarification with no project was accepted")
	}
}

func TestAutomationBoardsMarksOnlyTheDoneLane(t *testing.T) {
	projects := []AutomationProject{{
		Name:  "project",
		Lanes: testSection([]string{"coder", "done"}),
		Cards: testSection([]AutomationCard{
			{Name: "live", Lane: "coder"},
			{Name: "finished", Lane: "done"},
		}),
		Activity: testSection([]AutomationActivity{}),
	}}
	boards := automationBoards("/forge", projects)
	if boards[0].Cards[0].Done || !boards[0].Cards[1].Done {
		t.Errorf("cards = %+v, want only the done-lane card finished", boards[0].Cards)
	}
}

func TestSnapshotKeepsApprovalsWhenOnlyTheBoardViewFails(t *testing.T) {
	state := testAutomationState()
	state.Approvals = testSection([]AutomationApproval{{
		ID: "approval-1", Project: "project", Card: "card",
	}})
	state.Projects = testSection([]AutomationProject{{
		Name:     "project",
		Lanes:    testSection([]string{}),
		Cards:    testSection([]AutomationCard{}),
		Activity: testSection([]AutomationActivity{}),
	}})
	server := automationServer(t, state, AutomationProjection)
	defer server.Close()
	snapshot, err := (SnapshotSource{Root: "/forge", ConfiguredURL: server.URL}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(snapshot.Approvals) != 1 {
		t.Errorf("approvals = %+v, want the usable approval", snapshot.Approvals)
	}
	if !snapshot.BoardFallback || snapshot.Boards != nil {
		t.Errorf("boards = %+v, fallback = %t, want only a local board fallback", snapshot.Boards, snapshot.BoardFallback)
	}
}

func TestSnapshotSourceReportsAnUnannouncedDashboard(t *testing.T) {
	source := SnapshotSource{Root: t.TempDir()}
	if _, err := source.Identity(); err == nil {
		t.Error("Identity accepted a dashboard that never announced itself")
	}
	if _, err := source.Snapshot(context.Background()); err == nil {
		t.Error("Snapshot accepted a dashboard that never announced itself")
	}
}

func containsUnsupported(err error) bool {
	for err != nil {
		if _, ok := err.(UnsupportedProjectionError); ok {
			return true
		}
		unwrapper, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = unwrapper.Unwrap()
	}
	return false
}

func automationServer(t *testing.T, state AutomationState, projection string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("X-SwarmForge-Projection", projection)
		_ = json.NewEncoder(writer).Encode(state)
	}))
}

func testAutomationState() AutomationState {
	return AutomationState{
		Projection: AutomationProjection,
		Identity: &AutomationIdentity{
			Kind: "family", Name: "forge-a",
			AllProjects:  testSection([]string{"forgelet-bridge"}),
			OpenProjects: testSection([]string{"forgelet-bridge"}),
		},
		Approvals:         testSection([]AutomationApproval{}),
		ResolvedApprovals: testSection([]AutomationResolvedApproval{}),
		Clarifications: &AutomationClarifications{
			Legacy: testSection([]AutomationClarification{}),
			Typed:  testSection([]AutomationClarification{}),
		},
		Projects: testSection([]AutomationProject{}),
	}
}

func projectAutomationState() AutomationState {
	state := testAutomationState()
	state.Identity = &AutomationIdentity{
		Kind: "project", Name: "project-a",
		AllProjects:  testSection([]string{"project-a"}),
		OpenProjects: testSection([]string{"project-a"}),
	}
	return state
}

func testSection[T any](items []T) *AutomationSection[T] {
	return &AutomationSection[T]{Count: len(items), Items: items}
}

var testText = "which lane should this start in?"
