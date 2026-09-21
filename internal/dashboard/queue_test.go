package dashboard

import (
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

func TestQueueCreateRequestQueuesForTheLieutenant(t *testing.T) {
	store := fixedStore(t)

	if _, err := (Queue{Store: store}).CreateRequest("please retry the invoice card"); err != nil {
		t.Fatalf("CreateRequest: %v", err)
	}

	pending, err := store.Pending()
	if err != nil {
		t.Fatalf("Pending: %v", err)
	}
	if len(pending) != 1 || pending[0].Body != "please retry the invoice card" {
		t.Errorf("pending = %+v, want the queued chat request", pending)
	}
}
