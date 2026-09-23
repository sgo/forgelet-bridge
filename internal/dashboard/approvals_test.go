package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestApprovalsReachesTheDashboardItAnnounces(t *testing.T) {
	asked := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		asked <- request.Method + " " + request.URL.Path
		_ = json.NewEncoder(writer).Encode(map[string]any{"approvals": []map[string]any{
			{"id": "approval-1", "project": "forgelet-bridge", "task": "phone-approvals",
				"gate": "spec → refactorer", "from": "coder", "to": "refactorer"},
		}})
	}))
	defer server.Close()

	root := announced(t, server.URL)
	approvals := Approvals{Root: root}

	pending, err := approvals.Pending(context.Background())
	if err != nil {
		t.Fatalf("Pending: %v", err)
	}
	if got := <-asked; got != "GET /api/state" {
		t.Errorf("dashboard was asked %q, want the state endpoint", got)
	}
	if len(pending) != 1 || pending[0].Key != forgeKey(root, "forgelet-bridge", "approval-1") || pending[0].Gate != "coder → refactorer" {
		t.Errorf("pending = %+v, want the approval the dashboard is showing", pending)
	}
}

func TestApprovalsReachesTheConfiguredDashboard(t *testing.T) {
	asked := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		asked <- request.Method + " " + request.URL.Path
		_, _ = writer.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	approvals := Approvals{Root: t.TempDir(), ConfiguredURL: server.URL}
	if err := approvals.Approve(context.Background(), "forgelet-bridge", "approval-1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if got := <-asked; got != "POST /api/approvals/approval-1/approve" {
		t.Errorf("dashboard was asked %q, want the approve endpoint", got)
	}
}

func TestApprovalsSendBackReachesTheRetryEndpoint(t *testing.T) {
	asked := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		asked <- request.Method + " " + request.URL.Path
		_, _ = writer.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	approvals := Approvals{Root: t.TempDir(), ConfiguredURL: server.URL}
	if err := approvals.SendBack(context.Background(), "forgelet-bridge", "approval-1", "the total is wrong"); err != nil {
		t.Fatalf("SendBack: %v", err)
	}
	if got := <-asked; got != "POST /api/tasks/retry" {
		t.Errorf("dashboard was asked %q, want the retry endpoint", got)
	}
}

func TestApprovalsExplainsAForgeWithoutADashboard(t *testing.T) {
	approvals := Approvals{Root: t.TempDir()}

	if _, err := approvals.Pending(context.Background()); err == nil {
		t.Fatal("Pending succeeded for a forge whose dashboard never announced itself")
	}
}

// announced tells a forge where its dashboard is, the way the real dashboard
// does when it starts.
func announced(t *testing.T, url string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".swarmforge"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".swarmforge", "dashboard-url"), []byte(url+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}
