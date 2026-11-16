package fixtures

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"

	"github.com/unclebob/forgelet-bridge/internal/dashboard"
)

// FakeDashboard is a deterministic dashboard endpoint for automation-state
// scenarios. It counts the projection responses the real bridge requests
// without building the production dashboard's expensive payload.
type FakeDashboard struct {
	server *httptest.Server

	mu            sync.Mutex
	state         dashboard.AutomationState
	fullState     dashboard.State
	fail          bool
	malformed     bool
	unsupported   bool
	requests      uint64
	stateRequests uint64
	failed        uint64
	bytes         uint64
}

// FakeDashboardCounts is what one fake dashboard has served.
type FakeDashboardCounts struct {
	Requests      uint64
	StateRequests uint64
	Failed        uint64
	Bytes         uint64
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

// SetState replaces the automation projection the next reads return.
func (f *FakeDashboard) SetState(state dashboard.AutomationState) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state = state
	f.malformed = false
	f.unsupported = false
}

// SetFullState replaces the bounded full-state response used by dashboards
// that have not yet rolled out the current automation projection.
func (f *FakeDashboard) SetFullState(state dashboard.State) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fullState = state
}

// SetUnsupportedWithFullState switches the automation endpoint to unsupported
// and supplies its compatibility state in the same update.
func (f *FakeDashboard) SetUnsupportedWithFullState(state dashboard.State) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.unsupported = true
	f.fullState = state
}

// UpdateState changes the current state while preserving fields the caller
// does not replace.
func (f *FakeDashboard) UpdateState(update func(dashboard.AutomationState) dashboard.AutomationState) {
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

// SetMalformed makes the next automation reads return a malformed projection.
func (f *FakeDashboard) SetMalformed(malformed bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.malformed = malformed
}

// SetUnsupported makes the automation endpoint answer 404.
func (f *FakeDashboard) SetUnsupported(unsupported bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.unsupported = unsupported
}

// Counts reads the projection request, full-state request, failure and byte
// totals.
func (f *FakeDashboard) Counts() FakeDashboardCounts {
	f.mu.Lock()
	defer f.mu.Unlock()
	return FakeDashboardCounts{
		Requests: f.requests, StateRequests: f.stateRequests,
		Failed: f.failed, Bytes: f.bytes,
	}
}

// Close stops the fake dashboard.
func (f *FakeDashboard) Close() {
	if f != nil && f.server != nil {
		f.server.Close()
	}
}

func (f *FakeDashboard) serve(writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet && request.URL.Path == "/api/automation-state" {
		f.serveAutomation(writer)
		return
	}
	if request.Method == http.MethodGet && request.URL.Path == "/api/state" {
		f.serveFullState(writer)
		return
	}
	if strings.HasPrefix(request.URL.Path, "/api/") {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{}`))
		return
	}
	http.NotFound(writer, request)
}

func (f *FakeDashboard) serveAutomation(writer http.ResponseWriter) {
	f.mu.Lock()
	f.requests++
	if f.unsupported {
		f.failed++
		f.mu.Unlock()
		http.Error(writer, "automation state unsupported", http.StatusNotFound)
		return
	}
	if f.fail {
		f.failed++
		f.mu.Unlock()
		http.Error(writer, "fake dashboard automation state unavailable", http.StatusServiceUnavailable)
		return
	}
	state := f.state
	malformed := f.malformed
	f.mu.Unlock()
	if malformed {
		f.countFailure()
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("X-SwarmForge-Projection", dashboard.AutomationProjection)
		_, _ = writer.Write([]byte(`{"projection":"automation-v1","identity":{"kind":"family"}}`))
		return
	}
	if state.Projection == "" {
		state.Projection = dashboard.AutomationProjection
	}
	body, err := json.Marshal(state)
	if err == nil {
		f.mu.Lock()
		f.bytes += uint64(len(body))
		f.mu.Unlock()
	}
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("X-SwarmForge-Projection", dashboard.AutomationProjection)
	_, _ = writer.Write(body)
}

func (f *FakeDashboard) serveFullState(writer http.ResponseWriter) {
	f.mu.Lock()
	f.stateRequests++
	state := f.fullState
	f.mu.Unlock()
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(state)
}

func (f *FakeDashboard) countFailure() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failed++
}
