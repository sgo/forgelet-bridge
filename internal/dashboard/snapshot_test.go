package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
	typed := "typed clarification question"
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

	source := SnapshotSource{Root: "/forges/forge-a", ConfiguredURL: server.URL}
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
	if len(snapshot.TypedClarifications) != 1 ||
		snapshot.TypedClarifications[0].ID != "clarification-typed" ||
		snapshot.TypedClarifications[0].Question != typed {
		t.Errorf("typed clarifications = %+v, want the safe typed question", snapshot.TypedClarifications)
	}
	if typed := snapshot.TypedClarifications[0]; typed.Source != relay.ClarificationSourceTyped ||
		typed.Lifecycle != relay.ClarificationOpen || typed.Attention != "unseen" {
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
		relay.ResolutionSuperseded,
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

func TestTypedNotificationDependsOnBothOpenAndUnseen(t *testing.T) {
	question := "which stage?"
	for _, lifecycle := range []string{"open", "answered", "withdrawn"} {
		for _, attention := range []string{"unseen", "seen"} {
			t.Run(lifecycle+"/"+attention, func(t *testing.T) {
				sections := &AutomationClarifications{
					Legacy: testSection([]AutomationClarification{}),
					Typed: testSection([]AutomationClarification{{
						ID: "request-1", ClarificationID: "clar-1", Project: "project",
						Role: "coder", Status: lifecycle, Attention: attention, QuestionExcerpt: &question,
					}}),
				}
				clarifications, err := automationClarifications("/forge", sections)
				if err != nil {
					t.Fatal(err)
				}
				wantKey := "/forge/typed/project/clar-1"
				if len(clarifications) != 1 || clarifications[0].Key != wantKey ||
					clarifications[0].Lifecycle != lifecycle || clarifications[0].Attention != attention {
					t.Fatalf("clarifications = %+v, want %s with %s/%s", clarifications, wantKey, lifecycle, attention)
				}
			})
		}
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
			state.Clarifications.Typed = testSection([]AutomationClarification{{
				ClarificationID: "clar-1", Project: "project", Status: "open", Attention: "unseen",
			}})
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

func TestAutomationResponseKeepsReadAndDecodeFailuresDistinct(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr string
	}{
		{name: "read", status: http.StatusServiceUnavailable, body: "unavailable", wantErr: "read automation state"},
		{name: "decode", status: http.StatusOK, body: "{", wantErr: "decode automation projection"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.Header().Set("X-SwarmForge-Projection", AutomationProjection)
				writer.WriteHeader(test.status)
				_, _ = writer.Write([]byte(test.body))
			}))
			defer server.Close()
			_, err := NewAPI(server.URL).AutomationState(context.Background())
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want %q", err, test.wantErr)
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

func TestSnapshotDoesNotFallbackForAnOlderResolvedApprovalShape(t *testing.T) {
	state := testAutomationState()
	state.ResolvedApprovals = testSection([]AutomationResolvedApproval{{
		ID: "approval-old", Project: "project-a", Card: "old-card",
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

	_, err := (SnapshotSource{
		Root: "/forges/project-a", ConfiguredURL: server.URL,
	}).Snapshot(context.Background())
	if err == nil {
		t.Fatal("Snapshot accepted an older resolved-approval projection")
	}
	if stateRequests != 0 {
		t.Fatalf("state requests = %d, want no compatibility fallback", stateRequests)
	}
}

func TestSnapshotDoesNotFallbackWhenAutomationIsUnsupported(t *testing.T) {
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

	_, err := (SnapshotSource{Root: "/forges/forge-a", ConfiguredURL: server.URL}).Snapshot(context.Background())
	if err == nil {
		t.Fatal("Snapshot accepted an unsupported automation endpoint")
	}
	if requests["/api/automation-state"] != 1 || requests["/api/state"] != 0 {
		t.Fatalf("requests = %v, want one unsupported projection and no fallback", requests)
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
		ClarificationID: "clar-1", Status: "open", Project: "project",
		Role: "coder", QuestionExcerpt: &question,
	}
	project, id, ok := clarificationIdentity(valid)
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
			if _, _, ok := clarificationIdentity(test); ok {
				t.Error("incomplete clarification identity was accepted")
			}
		})
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
		Typed: testSection([]AutomationClarification{{
			ID: "request-1", ClarificationID: "clar-1", Status: "open",
			Project: "project", QuestionExcerpt: &question,
		}, {
			ID: "request-2", ClarificationID: "clar-1", Status: "open",
			Project: "project", QuestionExcerpt: &question,
		}}),
	}
	if _, err := automationClarifications("/forge", sections); err == nil {
		t.Fatal("duplicate typed clarification was accepted")
	}

	sections.Typed = testSection([]AutomationClarification{{
		ID: "request-1", ClarificationID: "clar-1", Status: "open",
		QuestionExcerpt: &question,
	}})
	if _, err := automationClarifications("/forge", sections); err == nil {
		t.Fatal("typed clarification with no project was accepted")
	}
}

func TestAutomationClarificationsIgnoresTerminalLegacyRows(t *testing.T) {
	question := "already answered"
	sections := &AutomationClarifications{
		Legacy: testSection([]AutomationClarification{{
			ID: "legacy-old", Status: "done", Project: "project",
			Role: "coder", QuestionExcerpt: &question,
		}}),
		Typed: testSection([]AutomationClarification{}),
	}
	clarifications, err := automationClarifications("/forge", sections)
	if err != nil {
		t.Fatal(err)
	}
	if len(clarifications) != 0 {
		t.Fatalf("terminal legacy rows produced clarifications = %+v, want none", clarifications)
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

func TestApprovalsAndClarificationsFacadesShareTheSnapshotSource(t *testing.T) {
	type source interface {
		Identity() (string, error)
		Snapshot(context.Context) (relay.DashboardSnapshot, error)
	}
	state := testAutomationState()
	server := automationServer(t, state, AutomationProjection)
	defer server.Close()
	for name, facade := range map[string]source{
		"approvals":      Approvals{Root: "/forge", ConfiguredURL: server.URL},
		"clarifications": Clarifications{Root: "/forge", ConfiguredURL: server.URL},
	} {
		t.Run(name, func(t *testing.T) {
			identity, err := facade.Identity()
			if err != nil || !strings.Contains(identity, server.URL) {
				t.Fatalf("identity = %q, %v; want the announced dashboard", identity, err)
			}
			snapshot, err := facade.Snapshot(context.Background())
			if err != nil || snapshot.Boards == nil {
				t.Fatalf("snapshot = %+v, %v; want the automation projection", snapshot, err)
			}
		})
	}
}

func TestApprovalsFacadeReportsEveryUnavailableDashboardAction(t *testing.T) {
	approvals := Approvals{Root: t.TempDir()}
	if _, err := approvals.Pending(context.Background()); err == nil {
		t.Error("Pending accepted an unannounced dashboard")
	}
	if _, err := approvals.Resolved(context.Background()); err == nil {
		t.Error("Resolved accepted an unannounced dashboard")
	}
	if err := approvals.Approve(context.Background(), "project", "approval"); err == nil {
		t.Error("Approve accepted an unannounced dashboard")
	}
	if err := approvals.SendBack(context.Background(), "project", "approval", "feedback"); err == nil {
		t.Error("SendBack accepted an unannounced dashboard")
	}
}

func TestApprovalsResolvedFacadeUsesTheDashboard(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(writer).Encode(State{ResolvedApprovals: []ResolvedApprovalRequest{{
			ID: "approval", Project: "project", Resolution: relay.ResolutionApproved,
		}}})
	}))
	defer server.Close()
	resolved, err := (Approvals{Root: "/forge", ConfiguredURL: server.URL}).Resolved(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := resolved["/forge/project/approval"]; got != relay.ResolutionApproved {
		t.Fatalf("resolved = %q, want approved", got)
	}
}

func TestURLRefusesAnEmptyAnnouncedAddress(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".swarmforge"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(URLFile)), []byte(" \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := URL(root, ""); err == nil || !strings.Contains(err.Error(), "address is empty") {
		t.Fatalf("URL error = %v, want the empty-address refusal", err)
	}
}

func TestDashboardRequestReportsEncodingAndConstructionFailures(t *testing.T) {
	api := NewAPI("http://dashboard.example")
	if _, err := api.request(context.Background(), http.MethodPost, "/api/chat", make(chan int)); err == nil {
		t.Error("request accepted a body that cannot be encoded")
	}
	if _, err := api.request(context.Background(), "bad method", "/api/state", nil); err == nil {
		t.Error("request accepted a method that cannot build an HTTP request")
	}
}

func TestDashboardExchangeReportsEveryTransportFailure(t *testing.T) {
	if _, _, _, err := NewAPI("http://dashboard.example").
		exchange(context.Background(), "bad method", "/api/state", nil, 0); err == nil {
		t.Error("exchange accepted a request that could not be built")
	}

	transport := &API{
		baseURL: "http://dashboard.example",
		client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("transport unavailable")
		})},
	}
	if _, _, _, err := transport.exchange(context.Background(), http.MethodGet, "/api/state", nil, 0); err == nil {
		t.Error("exchange accepted an unavailable transport")
	}

	response := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(errorReader{})}
	if _, err := responseBody(http.MethodGet, "/api/state", response, 0); err == nil {
		t.Error("responseBody accepted an unreadable response")
	}
	response = &http.Response{
		StatusCode: http.StatusBadGateway,
		Body:       io.NopCloser(strings.NewReader("bad gateway")),
	}
	if _, err := responseBody(http.MethodGet, "/api/state", response, 0); err == nil {
		t.Error("responseBody accepted an unsuccessful response")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, errors.New("read unavailable")
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
