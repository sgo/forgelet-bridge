package fixtures

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// The endpoints the bridge reaches a forge's dashboard on, as the dashboard
// serves them.
const (
	statePath  = "/api/state"
	approveDir = "/api/approvals/"
	approveEnd = "/approve"
	retryPath  = "/api/tasks/retry"
)

// DashboardCall is one request a forge's dashboard was asked to handle.
type DashboardCall struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Body   string `json:"body"`
}

// Dashboard is the fixture stand-in for a forge's dashboard: it serves the
// endpoints the bridge has to use, applies the same file effects the desktop
// dashboard applies, and records what it was asked to do.
type Dashboard struct {
	URL  string
	Root string

	approvals *approvalStore
	calls     string
	server    *http.Server

	mu  sync.Mutex
	log []DashboardCall
}

// StartDashboard serves a fixture forge's dashboard on a free local port, and
// announces itself the way the real dashboard does.
func StartDashboard(root string) (*Dashboard, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	dashboard := &Dashboard{
		URL:       fmt.Sprintf("http://%s", listener.Addr().String()),
		Root:      root,
		approvals: NewApprovals(root),
		calls:     filepath.Join(root, ".swarmforge", "dashboard-calls.jsonl"),
	}
	server := &http.Server{Handler: dashboard}
	dashboard.server = server
	go func() { _ = server.Serve(listener) }()

	if err := os.MkdirAll(filepath.Join(root, ".swarmforge"), 0o755); err != nil {
		server.Close()
		return nil, err
	}
	urlFile := filepath.Join(root, filepath.FromSlash(".swarmforge/dashboard-url"))
	if err := os.WriteFile(urlFile, []byte(dashboard.URL+"\n"), 0o644); err != nil {
		server.Close()
		return nil, err
	}
	return dashboard, nil
}

// Stop shuts the dashboard down.
func (d *Dashboard) Stop() {
	if d != nil && d.server != nil {
		_ = d.server.Close()
	}
}

// Calls are the requests the dashboard has handled.
func (d *Dashboard) Calls() []DashboardCall {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]DashboardCall(nil), d.log...)
}

// WasAsked reports whether the dashboard handled a request with this shape.
func (d *Dashboard) WasAsked(method, path, contains string) bool {
	for _, call := range d.Calls() {
		if call.Method == method && call.Path == path && strings.Contains(call.Body, contains) {
			return true
		}
	}
	return false
}

func (d *Dashboard) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	body := requestBody(request)
	d.record(DashboardCall{Method: request.Method, Path: request.URL.Path, Body: body})

	d.route(writer, request, body)
}

// requestBody reads a request's body: what could be read of it, empty when the
// dashboard was asked nothing.
func requestBody(request *http.Request) string {
	if request.Body == nil {
		return ""
	}
	data, _ := io.ReadAll(request.Body) // a body that ends early still counts
	return string(data)
}

// route sends a request to the endpoint that handles it.
func (d *Dashboard) route(writer http.ResponseWriter, request *http.Request, body string) {
	switch {
	case request.Method == http.MethodGet && request.URL.Path == statePath:
		d.state(writer)
	case request.Method == http.MethodPost && strings.HasPrefix(request.URL.Path, approveDir) && strings.HasSuffix(request.URL.Path, approveEnd):
		d.approve(writer, request.URL.Path, body)
	case request.Method == http.MethodPost && request.URL.Path == retryPath:
		d.retry(writer, body)
	default:
		http.Error(writer, "Not found", http.StatusNotFound)
	}
}

// state serves the approvals of the forge's open projects, in the shape the
// dashboard serves them.
func (d *Dashboard) state(writer http.ResponseWriter) {
	pending, err := d.approvals.Pending()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	approvals := make([]map[string]any, 0, len(pending))
	for _, approval := range pending {
		approvals = append(approvals, map[string]any{
			"id":        approval.ID,
			"project":   approval.Project,
			"task":      approval.Card,
			"task_id":   approval.TaskID,
			"gate":      approval.Gate,
			"from":      approval.From,
			"to":        approval.To,
			"artifacts": approval.Artifacts,
		})
	}
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(map[string]any{"forge": true, "approvals": approvals})
}

// approve applies what approving means, exactly as the desktop dashboard does.
func (d *Dashboard) approve(writer http.ResponseWriter, path, body string) {
	id := strings.TrimSuffix(strings.TrimPrefix(path, approveDir), approveEnd)
	project := projectOf(body)
	if project == "" {
		http.Error(writer, "Missing project", http.StatusBadRequest)
		return
	}
	if err := d.approvals.Approve(project, unescapeID(id)); err != nil {
		http.Error(writer, err.Error(), http.StatusNotFound)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write([]byte(`{"ok":true}`))
}

// retry applies what sending an approval back means, exactly as the desktop
// dashboard does.
func (d *Dashboard) retry(writer http.ResponseWriter, body string) {
	fields := fieldsOf(body)
	if fields["project"] == "" || fields["id"] == "" {
		http.Error(writer, "Missing project or approval id", http.StatusBadRequest)
		return
	}
	if err := d.approvals.SendBack(fields["project"], fields["id"], fields["comments"]); err != nil {
		http.Error(writer, err.Error(), http.StatusNotFound)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write([]byte(`{"ok":true}`))
}

func (d *Dashboard) record(call DashboardCall) {
	d.mu.Lock()
	d.log = append(d.log, call)
	d.mu.Unlock()

	file, err := os.OpenFile(d.calls, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	_ = json.NewEncoder(file).Encode(call)
}

// ReadDashboardCalls reads the requests a fixture dashboard has handled.
func ReadDashboardCalls(root string) ([]DashboardCall, error) {
	data, err := os.ReadFile(filepath.Join(root, ".swarmforge", "dashboard-calls.jsonl"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var calls []DashboardCall
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var call DashboardCall
		if err := json.Unmarshal([]byte(line), &call); err == nil {
			calls = append(calls, call)
		}
	}
	return calls, nil
}

func projectOf(body string) string { return fieldsOf(body)["project"] }

func fieldsOf(body string) map[string]string {
	fields := map[string]string{}
	_ = json.Unmarshal([]byte(body), &fields)
	return fields
}

func unescapeID(id string) string {
	unescaped, err := url.PathUnescape(id)
	if err != nil {
		return id
	}
	return unescaped
}
