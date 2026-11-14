package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSnapshotSourceCarriesEveryConsumerFromOneStateRead(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		if request.Method != http.MethodGet || request.URL.Path != "/api/state" {
			t.Errorf("request = %s %s, want one state read", request.Method, request.URL.Path)
		}
		_ = json.NewEncoder(writer).Encode(State{
			Approvals: []ApprovalRequest{{
				ID:      "approval-1",
				Project: "forgelet-bridge",
				Card:    "snapshot-card",
				From:    "coder",
				To:      "refactorer",
			}},
			ResolvedApprovals: []ResolvedApprovalRequest{{
				ID:         "approval-old",
				Project:    "forgelet-bridge",
				Resolution: "approved",
			}},
			Clarifications: []ClarificationRequest{{
				ID:      "clarification-1",
				Status:  "pending",
				Project: "forgelet-bridge",
				Role:    "coder",
				Body:    testText,
			}, {
				ID:     "clarification-answered",
				Status: "answered",
				Body:   testText,
			}},
		})
	}))
	defer server.Close()

	source := SnapshotSource{Root: "/forges/forge-a", ConfiguredURL: server.URL, Project: "project-a"}
	snapshot, err := source.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if requests != 1 {
		t.Fatalf("state requests = %d, want one", requests)
	}
	if len(snapshot.Approvals) != 1 || snapshot.Approvals[0].Key != "/forges/forge-a/forgelet-bridge/approval-1" {
		t.Errorf("approvals = %+v, want the pending approval once", snapshot.Approvals)
	}
	if got := snapshot.ResolvedApprovals["/forges/forge-a/forgelet-bridge/approval-old"]; got != "approved" {
		t.Errorf("resolved approval = %q, want approved", got)
	}
	if len(snapshot.Clarifications) != 1 || snapshot.Clarifications[0].Question != testText {
		t.Errorf("clarifications = %+v, want only the pending clarification", snapshot.Clarifications)
	}
}

func TestSnapshotSourceNamesTheProjectWhenTheDashboardDoesNot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"clarifications":[{"id":"clar-1","status":"pending","role":"coder","body":"next?"}]}`))
	}))
	defer server.Close()

	snapshot, err := (SnapshotSource{Root: t.TempDir(), ConfiguredURL: server.URL, Project: "project-a"}).
		Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(snapshot.Clarifications) != 1 || snapshot.Clarifications[0].Project != "project-a" {
		t.Errorf("clarifications = %+v, want the source project", snapshot.Clarifications)
	}
}

var testText = "which lane should this start in?"
