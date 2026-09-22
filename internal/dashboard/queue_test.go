package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestQueuePresentsRequestsInRelayForm(t *testing.T) {
	store := fixedStore(t)
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
	if len(requests) != 1 {
		t.Fatalf("requests = %+v, want the one request", requests)
	}
	if requests[0].ID != id || requests[0].Body != "is the build green?" || requests[0].Response != "yes, the build is green" {
		t.Errorf("request = %+v, want the request text and its answer", requests[0])
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
