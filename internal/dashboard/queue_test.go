package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQueuePresentsRequestsInRelayForm(t *testing.T) {
	store := fixedStore(t)
	writeRequest(t, store.root, pendingDir,
		"req-app",
		"id: req-app\nstatus: pending\nsource: app.completed-card-report\ncreated_at: 2026-11-09T10:00:00Z\n\nis the build green?\n")
	id, err := store.CreateRequest("is the build green?")
	if err != nil {
		t.Fatalf("CreateRequest: %v", err)
	}
	if err := store.Answer(id, "yes, the build is green"); err != nil {
		t.Fatalf("Answer: %v", err)
	}

	requests, err := Queue{Store: store}.Requests()
	if err != nil {
		t.Fatalf("Requests: %v", err)
	}
	if len(requests) != 2 {
		t.Fatalf("requests = %+v, want the sourced and ordinary requests", requests)
	}
	if requests[0].ID != "req-app" || requests[0].Source != "app.completed-card-report" ||
		requests[0].Body != "is the build green?" || requests[0].Response != "" {
		t.Errorf("request = %+v, want the request text and its answer", requests[0])
	}
	if requests[1].ID != id || requests[1].Body != "is the build green?" || requests[1].Response != "yes, the build is green" {
		t.Errorf("request = %+v, want the request text and its answer", requests[1])
	}
}

func TestQueueCreateRequestReportsADashboardThatRefusesIt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "not now", http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := Queue{Store: fixedStore(t), ConfiguredURL: server.URL}.CreateRequest(context.Background(), "is the build green?")

	if err == nil {
		t.Fatal("CreateRequest accepted a chat the dashboard refused")
	}
}

func TestQueueRequestsReportsTheQueueItCouldNotRead(t *testing.T) {
	store := fixedStore(t)
	if err := os.MkdirAll(filepath.Dir(store.dir(pendingDir)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.dir(pendingDir), []byte("not a queue\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := (Queue{Store: store}).Requests()
	if err == nil || !strings.Contains(err.Error(), store.Root()) {
		t.Fatalf("error = %v, want the unreadable queue named", err)
	}
}

func TestQueueCreateRequestAsksTheForgeToTakeIt(t *testing.T) {
	store := fixedStore(t)
	var path, text string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		path = request.URL.Path
		body := map[string]string{}
		_ = json.NewDecoder(request.Body).Decode(&body)
		text = body["text"]
		_, _ = writer.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	queue := Queue{Store: store, Root: store.Root(), ConfiguredURL: server.URL}
	if _, err := queue.CreateRequest(context.Background(), "please retry the invoice card"); err != nil {
		t.Fatalf("CreateRequest: %v", err)
	}

	if path != "/api/chat" || text != "please retry the invoice card" {
		t.Errorf("the forge was asked %s %q, want the chat request at /api/chat", path, text)
	}
	pending, err := store.Pending()
	if err != nil {
		t.Fatalf("Pending: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("pending = %+v, want the bridge not to write the queue itself", pending)
	}
}
