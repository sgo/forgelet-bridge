package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadHandoffReadsWhoHandedTheWorkOver(t *testing.T) {
	root := t.TempDir()
	writePendingHandoff(t, root, "forgelet-bridge", "approval-1", `id: approval-1
to: refactorer, architect
task: phone-message-wake
role: coder
artifacts: internal/bridge/bridge.go, internal/relay/relay.go

Re-read your role and constitution.
`)

	details, err := readHandoff(root, "forgelet-bridge", "approval-1")
	if err != nil {
		t.Fatalf("readHandoff: %v", err)
	}
	if details.from != "coder" || details.to != "refactorer" {
		t.Errorf("handoff = %+v, want the roles the handoff names", details)
	}
	want := []string{"internal/bridge/bridge.go", "internal/relay/relay.go"}
	if !reflect.DeepEqual(details.artifacts, want) {
		t.Errorf("artifacts = %v, want %v", details.artifacts, want)
	}
}

func TestReadHandoffExplainsAMissingHandoff(t *testing.T) {
	if _, err := readHandoff(t.TempDir(), "forgelet-bridge", "approval-1"); err == nil {
		t.Fatal("readHandoff accepted an approval with no handoff behind it")
	}
}

func TestApprovalsDecideWithTheHandoffTheDashboardDoesNotSpellOut(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"approvals": []map[string]any{{
				"id": "approval-1", "project": "forgelet-bridge", "task": "phone-message-wake",
				"gate":      "spec → refactorer",
				"artifacts": []string{"internal/bridge/bridge.go"},
			}},
		})
	}))
	defer server.Close()

	root := t.TempDir()
	writePendingHandoff(t, root, "forgelet-bridge", "approval-1", `id: approval-1
to: refactorer
task: phone-message-wake
role: coder
artifacts: internal/bridge/apply.go, internal/relay/relay.go

Re-read your role and constitution.
`)

	approvals, err := NewForgeAPI(root, server.URL).Approvals(context.Background())
	if err != nil {
		t.Fatalf("Approvals: %v", err)
	}
	if len(approvals) != 1 {
		t.Fatalf("approvals = %+v, want the one the dashboard is showing", approvals)
	}
	if approvals[0].Gate != "coder → refactorer" {
		t.Errorf("gate = %q, want the roles the handoff names", approvals[0].Gate)
	}
	want := []string{"internal/bridge/apply.go", "internal/relay/relay.go"}
	if !reflect.DeepEqual(approvals[0].Artifacts, want) {
		t.Errorf("artifacts = %v, want the files the handoff says changed", approvals[0].Artifacts)
	}
}

// writePendingHandoff lays a pending handoff where a project keeps one.
func writePendingHandoff(t *testing.T, root, project, id, content string) {
	t.Helper()
	path := filepath.Join(root, "projects", project, ".swarmforge", "handoffs", "pending_approval", id+".handoff")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// approvalsState serves the dashboard state that reports one approval, so a
// test can say what the dashboard itself says about it.
func approvalsState(t *testing.T, approval map[string]any) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(writer).Encode(map[string]any{"approvals": []map[string]any{approval}})
	}))
	t.Cleanup(server.Close)
	return server
}

func TestApprovalsFallBackToTheDashboardsGateWhenTheHandoffNamesOneRole(t *testing.T) {
	// A handoff that names the role but not who it went to does not have a
	// handover to show, so the gate the dashboard reports is what the operator
	// decides with.
	server := approvalsState(t, map[string]any{
		"id": "approval-1", "project": "forgelet-bridge", "gate": "spec → refactorer",
	})
	root := t.TempDir()
	writePendingHandoff(t, root, "forgelet-bridge", "approval-1", `id: approval-1
role: coder
task: phone-message-wake

Re-read your role and constitution.
`)

	approvals, err := NewForgeAPI(root, server.URL).Approvals(context.Background())
	if err != nil {
		t.Fatalf("Approvals: %v", err)
	}
	if len(approvals) != 1 || approvals[0].Gate != "spec → refactorer" {
		t.Errorf("approvals = %+v, want the gate the dashboard reports", approvals)
	}
}

func TestApprovalsKeepTheDashboardsFilesWhenTheHandoffNamesNone(t *testing.T) {
	server := approvalsState(t, map[string]any{
		"id": "approval-1", "project": "forgelet-bridge", "gate": "spec → refactorer",
		"artifacts": []string{"internal/bridge/bridge.go"},
	})
	root := t.TempDir()
	writePendingHandoff(t, root, "forgelet-bridge", "approval-1", `id: approval-1
to: refactorer
role: coder
task: phone-message-wake

Re-read your role and constitution.
`)

	approvals, err := NewForgeAPI(root, server.URL).Approvals(context.Background())
	if err != nil {
		t.Fatalf("Approvals: %v", err)
	}
	want := []string{"internal/bridge/bridge.go"}
	if len(approvals) != 1 || !reflect.DeepEqual(approvals[0].Artifacts, want) {
		t.Errorf("artifacts = %+v, want the files the dashboard reports when the handoff names none", approvals)
	}
}

func TestApprovalsTakeTheOneFileTheHandoffNames(t *testing.T) {
	server := approvalsState(t, map[string]any{
		"id": "approval-1", "project": "forgelet-bridge", "gate": "spec → refactorer",
		"artifacts": []string{"internal/bridge/bridge.go", "internal/relay/relay.go"},
	})
	root := t.TempDir()
	writePendingHandoff(t, root, "forgelet-bridge", "approval-1", `id: approval-1
to: refactorer
role: coder
task: phone-message-wake
artifacts: internal/bridge/apply.go

Re-read your role and constitution.
`)

	approvals, err := NewForgeAPI(root, server.URL).Approvals(context.Background())
	if err != nil {
		t.Fatalf("Approvals: %v", err)
	}
	want := []string{"internal/bridge/apply.go"}
	if len(approvals) != 1 || !reflect.DeepEqual(approvals[0].Artifacts, want) {
		t.Errorf("artifacts = %+v, want the one file the handoff names", approvals)
	}
}
