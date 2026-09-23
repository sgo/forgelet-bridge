package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClarificationsReadsThePendingOnesTheDashboardIsShowing(t *testing.T) {
	asked := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		asked <- request.Method + " " + request.URL.Path
		_ = json.NewEncoder(writer).Encode(map[string]any{"clarifications": []map[string]any{
			{"id": "clar-1", "status": "pending", "role": "coder", "project": "forgelet-bridge",
				"body": "which lane should the refund card start in?"},
			{"id": "clar-2", "status": "done", "role": "specifier", "project": "forgelet-bridge",
				"body": "already answered", "response": "yes"},
		}})
	}))
	defer server.Close()

	root := announced(t, server.URL)
	clarifications := Clarifications{Root: root}

	pending, err := clarifications.Pending(context.Background())
	if err != nil {
		t.Fatalf("Pending: %v", err)
	}
	if got := <-asked; got != "GET /api/state" {
		t.Errorf("dashboard was asked %q, want the state endpoint", got)
	}
	if len(pending) != 1 {
		t.Fatalf("pending = %+v, want only the clarification still waiting", pending)
	}
	want := struct{ Key, Project, Role, Question string }{
		Key:      forgeKey(root, "forgelet-bridge", "clar-1"),
		Project:  "forgelet-bridge",
		Role:     "coder",
		Question: "which lane should the refund card start in?",
	}
	got := struct{ Key, Project, Role, Question string }{
		Key: pending[0].Key, Project: pending[0].Project, Role: pending[0].Role, Question: pending[0].Question,
	}
	if got != want {
		t.Errorf("pending = %+v, want %+v", got, want)
	}
	if pending[0].ID != "clar-1" {
		t.Errorf("id = %q, want the dashboard's own id", pending[0].ID)
	}
}

func TestClarificationsAnswersThroughTheDashboard(t *testing.T) {
	asked := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		asked <- request.Method + " " + request.URL.Path
		_, _ = writer.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	clarifications := Clarifications{Root: t.TempDir(), ConfiguredURL: server.URL}
	if err := clarifications.Answer(context.Background(), "forgelet-bridge", "clar-1", "yes"); err != nil {
		t.Fatalf("Answer: %v", err)
	}
	if got := <-asked; got != "POST /api/clarifications/clar-1/answer" {
		t.Errorf("dashboard was asked %q, want the answer endpoint", got)
	}
}

func TestClarificationsExplainsAForgeWithoutADashboard(t *testing.T) {
	clarifications := Clarifications{Root: t.TempDir()}

	if _, err := clarifications.Pending(context.Background()); err == nil {
		t.Fatal("Pending succeeded for a forge whose dashboard never announced itself")
	}
}
