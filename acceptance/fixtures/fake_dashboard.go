package fixtures

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"

	"github.com/unclebob/forgelet-bridge/internal/dashboard"
)

// FakeDashboard is a deterministic dashboard endpoint for state-read and
// failure scenarios. It counts the complete state responses the real bridge
// requests without building the production dashboard's expensive payload.
type FakeDashboard struct {
	server *httptest.Server

	mu       sync.Mutex
	state    dashboard.State
	fail     bool
	requests uint64
	failed   uint64
	bytes    uint64
}

// FakeDashboardCounts is what one fake dashboard has served.
type FakeDashboardCounts struct {
	Requests uint64
	Failed   uint64
	Bytes    uint64
}

// StartFakeDashboard starts a fake dashboard on a loopback address.
func StartFakeDashboard() *FakeDashboard {
	fake := &FakeDashboard{}
	fake.server = httptest.NewServer(http.HandlerFunc(fake.serve))
	return fake
}

// URL is the address the bridge is configured to use.
func (f *FakeDashboard) URL() string {
	return strings.TrimRight(f.server.URL, "/")
}

// SetState replaces the complete state the next reads return.
func (f *FakeDashboard) SetState(state dashboard.State) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state = state
}

// UpdateState changes the current state while preserving fields the caller
// does not replace.
func (f *FakeDashboard) UpdateState(update func(dashboard.State) dashboard.State) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state = update(f.state)
}

// SetFailure makes state reads fail until another state is supplied or failure
// is cleared.
func (f *FakeDashboard) SetFailure(fail bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fail = fail
}

// Counts reads the request, failure and state-body byte totals.
func (f *FakeDashboard) Counts() FakeDashboardCounts {
	f.mu.Lock()
	defer f.mu.Unlock()
	return FakeDashboardCounts{Requests: f.requests, Failed: f.failed, Bytes: f.bytes}
}

// Close stops the fake dashboard.
func (f *FakeDashboard) Close() {
	if f != nil && f.server != nil {
		f.server.Close()
	}
}

func (f *FakeDashboard) serve(writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet && request.URL.Path == "/api/state" {
		f.serveState(writer)
		return
	}
	if strings.HasPrefix(request.URL.Path, "/api/") {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{}`))
		return
	}
	http.NotFound(writer, request)
}

func (f *FakeDashboard) serveState(writer http.ResponseWriter) {
	f.mu.Lock()
	f.requests++
	if f.fail {
		f.failed++
		f.mu.Unlock()
		http.Error(writer, "fake dashboard state unavailable", http.StatusServiceUnavailable)
		return
	}
	body, err := json.Marshal(f.state)
	if err == nil {
		f.bytes += uint64(len(body))
	}
	f.mu.Unlock()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write(body)
}
