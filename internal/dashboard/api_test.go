package dashboard

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestURLPrefersTheConfiguredAddress(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".swarmforge"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".swarmforge", "dashboard-url"), []byte("http://127.0.0.1:1111\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	url, err := URL(root, "http://127.0.0.1:2222/")
	if err != nil {
		t.Fatalf("URL: %v", err)
	}
	if url != "http://127.0.0.1:2222" {
		t.Errorf("url = %q, want the configured address", url)
	}
}

func TestURLFallsBackToWhatTheDashboardAnnounced(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".swarmforge"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".swarmforge", "dashboard-url"), []byte("http://127.0.0.1:2222\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	url, err := URL(root, "")
	if err != nil {
		t.Fatalf("URL: %v", err)
	}
	if url != "http://127.0.0.1:2222" {
		t.Errorf("url = %q", url)
	}
}

func TestURLExplainsWhenTheDashboardHasNotAnnouncedItself(t *testing.T) {
	if _, err := URL(t.TempDir(), ""); err == nil {
		t.Fatal("URL accepted a forge whose dashboard never announced itself")
	}
}

func TestApprovalsReadsTheDashboardState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/state" {
			t.Errorf("path = %q, want the state endpoint", request.URL.Path)
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"approvals": []map[string]any{
				{"id": "approval-1", "project": "forgelet-bridge", "task": "phone-approvals", "task_id": "id-1",
					"gate": "spec → refactorer", "from": "coder", "to": "refactorer",
					"artifacts": []string{"internal/bridge/bridge.go"}},
				{"id": "approval-2", "project": "forgelet-bridge", "task": "another", "gate": "spec → architect"},
			},
		})
	}))
	defer server.Close()

	approvals, err := NewAPI(server.URL).Approvals(context.Background())
	if err != nil {
		t.Fatalf("Approvals: %v", err)
	}
	if len(approvals) != 2 {
		t.Fatalf("approvals = %+v, want two", approvals)
	}
	if approvals[0].Gate != "coder → refactorer" {
		t.Errorf("gate = %q, want the roles the forge reports", approvals[0].Gate)
	}
	if approvals[0].Key != "forgelet-bridge/approval-1" || approvals[0].Card != "phone-approvals" ||
		len(approvals[0].Artifacts) != 1 {
		t.Errorf("approval = %+v", approvals[0])
	}
	if approvals[1].Gate != "spec → architect" {
		t.Errorf("gate = %q, want the gate as reported", approvals[1].Gate)
	}
}

func TestApproveCallsTheDashboardEndpoint(t *testing.T) {
	var gotPath, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		gotPath = request.URL.Path
		body, _ := io.ReadAll(request.Body)
		gotBody = string(body)
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{}`))
	}))
	defer server.Close()

	if err := NewAPI(server.URL).Approve(context.Background(), "forgelet-bridge", "approval-1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if gotPath != "/api/approvals/approval-1/approve" {
		t.Errorf("path = %q, want the approve endpoint", gotPath)
	}
	if !strings.Contains(gotBody, `"project":"forgelet-bridge"`) || !strings.Contains(gotBody, `"id":"approval-1"`) {
		t.Errorf("body = %q, want the approval and its project", gotBody)
	}
}

func TestSendBackCallsTheRetryEndpoint(t *testing.T) {
	var gotPath, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		gotPath = request.URL.Path
		body, _ := io.ReadAll(request.Body)
		gotBody = string(body)
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{}`))
	}))
	defer server.Close()

	if err := NewAPI(server.URL).SendBack(context.Background(), "forgelet-bridge", "approval-1", "still wrong"); err != nil {
		t.Fatalf("SendBack: %v", err)
	}
	if gotPath != "/api/tasks/retry" {
		t.Errorf("path = %q, want the retry endpoint", gotPath)
	}
	if !strings.Contains(gotBody, `"comments":"still wrong"`) {
		t.Errorf("body = %q, want the feedback", gotBody)
	}
}

func TestCallsReportWhatTheDashboardAnswered(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Error(writer, "Unknown approval: approval-1", http.StatusNotFound)
	}))
	defer server.Close()

	err := NewAPI(server.URL).Approve(context.Background(), "forgelet-bridge", "approval-1")
	if err == nil || !strings.Contains(err.Error(), "Unknown approval") {
		t.Fatalf("error = %v, want the dashboard's answer", err)
	}
}
