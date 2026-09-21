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

func TestRequestForBodyAnswersWithTheNewestPendingMatch(t *testing.T) {
	store := fixedStore(t)
	writeRequest(t, store.root, pendingDir, "req-1", "id: req-1\nstatus: pending\n\nis the build green?\n")
	writeRequest(t, store.root, doneDir, "req-2", "id: req-2\nstatus: done\nresponse: yes\n\nplease retry the invoice card\n")
	writeRequest(t, store.root, pendingDir, "req-3", "id: req-3\nstatus: pending\n\nis the build green?\n")

	request, found, err := store.RequestForBody("is the build green?")
	if err != nil {
		t.Fatalf("RequestForBody: %v", err)
	}
	if !found || request.ID != "req-3" || request.Done() {
		t.Errorf("request = %+v, %v, want the newest request still waiting for an answer", request, found)
	}
}

func TestRequestForBodySkipsAnsweredRequestsAndMissingText(t *testing.T) {
	store := fixedStore(t)
	writeRequest(t, store.root, doneDir, "req-1", "id: req-1\nstatus: done\nresponse: retried\n\nplease retry the invoice card\n")
	writeRequest(t, store.root, pendingDir, "req-2", "id: req-2\nstatus: pending\n\nis the build green?\n")

	for _, text := range []string{"please retry the invoice card", "something else"} {
		request, found, err := store.RequestForBody(text)
		if err != nil {
			t.Fatalf("RequestForBody(%q): %v", text, err)
		}
		if found {
			t.Errorf("RequestForBody(%q) = %+v, want no open request", text, request)
		}
	}
}

func TestRenderAndParseKeepEveryField(t *testing.T) {
	request := Request{
		ID:        "req-1",
		Status:    StatusPending,
		Role:      "lieutenant",
		Body:      "is the build green?",
		Response:  "yes, the build is green",
		CreatedAt: "2026-09-21T20:04:32.588996Z",
		UpdatedAt: "2026-09-21T20:05:32.588996Z",
	}

	if got := Parse(render(request)); got != request {
		t.Errorf("round trip = %+v, want %+v", got, request)
	}
}

func TestRequestsIgnoreEntriesThatAreNotRequestFiles(t *testing.T) {
	store := fixedStore(t)
	writeRequest(t, store.root, pendingDir, "req-1", "id: req-1\nstatus: pending\n\nis the build green?\n")
	if err := os.MkdirAll(filepath.Join(store.dir(pendingDir), "notes.request"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store.dir(pendingDir), "notes.txt"), []byte("not a request\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	requests := requestsOf(t, store)
	if len(requests) != 1 || requests[0].ID != "req-1" {
		t.Errorf("requests = %+v, want only the request file", requests)
	}
}

func TestRequestsReadTheIDTheRequestFileCarries(t *testing.T) {
	store := fixedStore(t)
	writeRequest(t, store.root, pendingDir, "req-file", "id: req-header\nstatus: pending\n\nis the build green?\n")

	requests := requestsOf(t, store)
	if len(requests) != 1 || requests[0].ID != "req-header" {
		t.Errorf("requests = %+v, want the id the request file carries", requests)
	}
}

func TestRequestsKeepFileOrderForRequestsThatShareAnID(t *testing.T) {
	store := fixedStore(t)
	writeRequest(t, store.root, pendingDir, "a", "id: req-1\nstatus: pending\n\nfirst\n")
	writeRequest(t, store.root, pendingDir, "b", "id: req-1\nstatus: pending\n\nsecond\n")

	requests := requestsOf(t, store)
	if len(requests) != 2 || requests[0].Body != "first" || requests[1].Body != "second" {
		t.Errorf("requests = %+v, want the stable file order kept", requests)
	}
}

func TestRequestForBodyAnswersASingleOpenRequest(t *testing.T) {
	store := fixedStore(t)
	writeRequest(t, store.root, pendingDir, "req-1", "id: req-1\nstatus: pending\n\nis the build green?\n")

	request, found, err := store.RequestForBody("is the build green?")
	if err != nil {
		t.Fatalf("RequestForBody: %v", err)
	}
	if !found || request.ID != "req-1" {
		t.Errorf("request = %+v, %v, want the one open request", request, found)
	}
}

func TestRequestForBodyReportsAQueueItCannotRead(t *testing.T) {
	store := New(t.TempDir())
	if err := os.MkdirAll(filepath.Dir(store.dir(pendingDir)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.dir(pendingDir), []byte("not a queue\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	request, found, err := store.RequestForBody("is the build green?")
	if err == nil {
		t.Fatal("RequestForBody read a queue that is not a queue")
	}
	if found {
		t.Errorf("request = %+v, want no match from a queue that could not be read", request)
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

// requestsOf reads the queue a test has just written to.
func requestsOf(t *testing.T, store *Store) []Request {
	t.Helper()
	requests, err := store.Requests()
	if err != nil {
		t.Fatalf("Requests: %v", err)
	}
	return requests
}
