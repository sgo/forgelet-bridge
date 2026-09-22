package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

// URLFile is the file a forge's dashboard writes its address to.
const URLFile = ".swarmforge/dashboard-url"

// URL reads the address of the forge's dashboard, the way the dashboard
// announces itself. An address the configuration does not give - or gives as
// nothing the bridge could reach - falls back to that file.
func URL(root, configured string) (string, error) {
	if address := dashboardAddress(configured); address != "" {
		return address, nil
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(URLFile)))
	if err != nil {
		return "", fmt.Errorf("the forge's dashboard has not announced itself: %w", err)
	}
	address := dashboardAddress(string(data))
	if address == "" {
		return "", fmt.Errorf("the forge's dashboard address is empty")
	}
	return address, nil
}

// dashboardAddress is an announced or configured address, without the
// separators that would break the paths the bridge adds to it.
func dashboardAddress(raw string) string {
	return strings.TrimRight(strings.TrimSpace(raw), "/")
}

// API is a forge's dashboard, reached the way the dashboard itself is reached:
// over its endpoints, so its own handling is what takes effect.
type API struct {
	baseURL string
	client  *http.Client
}

// NewAPI builds a client for a dashboard at a base URL.
func NewAPI(baseURL string) *API {
	return &API{baseURL: strings.TrimRight(baseURL, "/"), client: http.DefaultClient}
}

// Approvals is one forge's approvals, reached through its dashboard: the
// bridge asks the dashboard and never edits the forge's files itself.
type Approvals struct {
	Root          string
	ConfiguredURL string
}

// Pending reads the approvals the dashboard is showing.
func (a Approvals) Pending(ctx context.Context) ([]relay.Approval, error) {
	api, err := a.client()
	if err != nil {
		return nil, err
	}
	return api.Approvals(ctx)
}

// Approve asks the dashboard to approve an approval.
func (a Approvals) Approve(ctx context.Context, project, id string) error {
	api, err := a.client()
	if err != nil {
		return err
	}
	return api.Approve(ctx, project, id)
}

// SendBack asks the dashboard to send an approval back with feedback.
func (a Approvals) SendBack(ctx context.Context, project, id, feedback string) error {
	api, err := a.client()
	if err != nil {
		return err
	}
	return api.SendBack(ctx, project, id, feedback)
}

// client is the dashboard of this forge, whose address is looked up every time
// so a dashboard that moved is picked up.
func (a Approvals) client() (*API, error) {
	url, err := URL(a.Root, a.ConfiguredURL)
	if err != nil {
		return nil, err
	}
	return NewAPI(url), nil
}

// State is what the dashboard reports about its forge.
type State struct {
	Approvals []ApprovalRequest `json:"approvals"`
}

// ApprovalRequest is one approval the dashboard is showing.
type ApprovalRequest struct {
	ID        string   `json:"id"`
	Project   string   `json:"project"`
	Card      string   `json:"task"`
	TaskID    string   `json:"task_id"`
	Gate      string   `json:"gate"`
	From      string   `json:"from"`
	To        string   `json:"to"`
	Artifacts []string `json:"artifacts"`
}

// Approvals reads the approvals the dashboard is showing.
func (a *API) Approvals(ctx context.Context) ([]relay.Approval, error) {
	var state State
	if err := a.call(ctx, http.MethodGet, "/api/state", nil, &state); err != nil {
		return nil, err
	}
	approvals := make([]relay.Approval, 0, len(state.Approvals))
	for _, request := range state.Approvals {
		approvals = append(approvals, relay.Approval{
			Key:       request.Project + "/" + request.ID,
			Project:   request.Project,
			ID:        request.ID,
			Card:      request.Card,
			Gate:      request.gate(),
			Artifacts: request.Artifacts,
		})
	}
	return approvals, nil
}

// Approve approves an approval through the dashboard, so the dashboard applies
// everything approving means.
func (a *API) Approve(ctx context.Context, project, id string) error {
	body := map[string]string{"id": id, "project": project}
	return a.call(ctx, http.MethodPost, "/api/approvals/"+url.PathEscape(id)+"/approve", body, nil)
}

// SendBack sends an approval back with feedback through the dashboard, so the
// dashboard applies everything sending it back means.
func (a *API) SendBack(ctx context.Context, project, id, feedback string) error {
	body := map[string]string{"id": id, "project": project, "comments": feedback}
	return a.call(ctx, http.MethodPost, "/api/tasks/retry", body, nil)
}

// gate is the gate as the operator should read it: the handover roles when the
// forge reports them, otherwise the gate exactly as reported.
func (r ApprovalRequest) gate() string {
	if strings.TrimSpace(r.From) != "" && strings.TrimSpace(r.To) != "" {
		return strings.TrimSpace(r.From) + " → " + strings.TrimSpace(r.To)
	}
	return r.Gate
}

func (a *API) call(ctx context.Context, method, path string, body any, out any) error {
	request, err := a.request(ctx, method, path, body)
	if err != nil {
		return err
	}
	response, err := a.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if err := responseError(method, path, response, data); err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(data, out)
}

// request builds one call to the dashboard, with its body encoded when it has
// one.
func (a *API) request(ctx context.Context, method, path string, body any) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, a.baseURL+path, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	return request, nil
}

// responseError turns an answer the dashboard refused into an error naming the
// call and what the dashboard said about it.
func responseError(method, path string, response *http.Response, data []byte) error {
	if response.StatusCode >= 200 && response.StatusCode <= 299 {
		return nil
	}
	return fmt.Errorf("%s %s: %s: %s", method, path, response.Status, strings.TrimSpace(string(data)))
}
