package dashboard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixedStore(t *testing.T) *Store {
	t.Helper()
	root := t.TempDir()
	store := New(root)
	at := time.Date(2026, 9, 21, 20, 4, 32, 588996000, time.UTC)
	store.now = func() time.Time { return at }
	return store
}

func TestCreateRequestWritesDashboardFormat(t *testing.T) {
	store := fixedStore(t)

	id, err := store.CreateRequest("is the build green?")
	if err != nil {
		t.Fatalf("CreateRequest: %v", err)
	}
	if id != "req-20260921T200432.588996000Z" {
		t.Errorf("id = %q, want a timestamped request id", id)
	}

	path := filepath.Join(store.dir(pendingDir), id+".request")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read request: %v", err)
	}
	want := "id: req-20260921T200432.588996000Z\n" +
		"status: pending\n" +
		"created_at: 2026-09-21T20:04:32.588996Z\n" +
		"\nis the build green?\n"
	if string(data) != want {
		t.Errorf("request file =\n%q\nwant\n%q", data, want)
	}
}

func TestRequestsReadsPendingThenDone(t *testing.T) {
	store := fixedStore(t)
	root := store.root
	writeRequest(t, root, pendingDir, "req-1", "id: req-1\nstatus: pending\ncreated_at: 2026-09-21T20:00:00Z\n\nfirst\n")
	writeRequest(t, root, doneDir, "req-2", "id: req-2\nstatus: done\ncreated_at: 2026-09-21T20:01:00Z\nresponse: second answer\n\nsecond\n")

	requests, err := store.Requests()
	if err != nil {
		t.Fatalf("Requests: %v", err)
	}
	if len(requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(requests))
	}
	if requests[0].ID != "req-1" || requests[0].Body != "first" {
		t.Errorf("first request = %+v", requests[0])
	}
	if requests[0].Done() {
		t.Error("first request should be pending")
	}
	if requests[1].Response != "second answer" {
		t.Errorf("response = %q, want %q", requests[1].Response, "second answer")
	}
	if !requests[1].Done() {
		t.Error("second request should be done")
	}
}

func TestRequestsUnescapeMultiLineResponse(t *testing.T) {
	store := fixedStore(t)
	writeRequest(t, store.root, doneDir, "req-1",
		"id: req-1\nstatus: done\nresponse: line one\\nline two\n\nbody\n")

	requests, err := store.Requests()
	if err != nil {
		t.Fatalf("Requests: %v", err)
	}
	if got := requests[0].Response; got != "line one\nline two" {
		t.Errorf("response = %q, want unescaped newlines", got)
	}
}

func TestAnswerMovesRequestToDone(t *testing.T) {
	store := fixedStore(t)
	id, err := store.CreateRequest("please retry the invoice card")
	if err != nil {
		t.Fatalf("CreateRequest: %v", err)
	}

	if err := store.Answer(id, "retried, the card is queued"); err != nil {
		t.Fatalf("Answer: %v", err)
	}

	pending, err := store.Pending()
	if err != nil {
		t.Fatalf("Pending: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("pending = %+v, want none", pending)
	}

	requests, err := store.Requests()
	if err != nil {
		t.Fatalf("Requests: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(requests))
	}
	answered := requests[0]
	if answered.Status != StatusDone || answered.Response != "retried, the card is queued" {
		t.Errorf("answered request = %+v", answered)
	}
	if answered.CreatedAt != "2026-09-21T20:04:32.588996Z" {
		t.Errorf("created_at = %q, want the original timestamp", answered.CreatedAt)
	}
}

func TestRequestsMissingQueueIsEmpty(t *testing.T) {
	store := New(t.TempDir())
	requests, err := store.Requests()
	if err != nil {
		t.Fatalf("Requests: %v", err)
	}
	if len(requests) != 0 {
		t.Errorf("requests = %+v, want none", requests)
	}
}

func TestCreateRequestRejectsEmptyBody(t *testing.T) {
	store := fixedStore(t)
	if _, err := store.CreateRequest("   "); err == nil {
		t.Fatal("CreateRequest accepted a blank body")
	}
}

func TestAnswerUnknownRequestFails(t *testing.T) {
	store := fixedStore(t)
	if err := store.Answer("req-nope", "answer"); err == nil {
		t.Fatal("Answer accepted an unknown request")
	}
}

func TestCreateRequestIDsAreUnique(t *testing.T) {
	store := fixedStore(t)
	first, err := store.CreateRequest("one")
	if err != nil {
		t.Fatalf("CreateRequest: %v", err)
	}
	second, err := store.CreateRequest("two")
	if err != nil {
		t.Fatalf("CreateRequest: %v", err)
	}
	if first == second {
		t.Fatalf("ids collided: %q", first)
	}
	if !strings.HasPrefix(second, first) {
		t.Errorf("second id = %q, want it derived from %q", second, first)
	}
}

func writeRequest(t *testing.T, root, kind, id, body string) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(requestsDir), kind)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+fileSuffix), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
