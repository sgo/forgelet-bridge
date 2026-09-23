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
	// root is the forge the dashboard serves, for the handoffs its state does
	// not spell out.
	root string
}

// NewAPI builds a client for a dashboard at a base URL.
func NewAPI(baseURL string) *API {
	return &API{baseURL: strings.TrimRight(baseURL, "/"), client: http.DefaultClient}
}

// NewForgeAPI builds a client for the dashboard of a forge root.
func NewForgeAPI(root, baseURL string) *API {
	api := NewAPI(baseURL)
	api.root = root
	return api
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
	return NewForgeAPI(a.Root, url), nil
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

// Chat asks the dashboard to take a chat message the way its clients give it:
// the dashboard queues the request and wakes the lieutenant, which is why the
// bridge must not write the queue itself.
func (a *API) Chat(ctx context.Context, text string) error {
	return a.call(ctx, http.MethodPost, "/api/chat", map[string]string{"text": text}, nil)
}

// Approvals reads the approvals the dashboard is showing.
func (a *API) Approvals(ctx context.Context) ([]relay.Approval, error) {
	var state State
	if err := a.call(ctx, http.MethodGet, "/api/state", nil, &state); err != nil {
		return nil, err
	}
	approvals := make([]relay.Approval, 0, len(state.Approvals))
	for _, request := range state.Approvals {
		approval := relay.Approval{
			Key:       forgeKey(a.root, request.Project, request.ID),
			Project:   request.Project,
			ID:        request.ID,
			Card:      request.Card,
			Gate:      request.gate(),
			Artifacts: request.Artifacts,
		}
		// The dashboard names the gate its own way and keeps only the
		// documents it can comment on; the handoff says who handed the work
		// over and what changed, which is what the operator decides with.
		if details, err := readHandoff(a.root, request.Project, request.ID); err == nil {
			if details.from != "" && details.to != "" {
				approval.Gate = details.from + " → " + details.to
			}
			if len(details.artifacts) > 0 {
				approval.Artifacts = details.artifacts
			}
		}
		approvals = append(approvals, approval)
	}
	return approvals, nil
}

// forgeKey names one item inside one forge: what the bridge remembers is per
// forge, so the same project and id in two forges are two items, not one.
func forgeKey(root, project, id string) string {
	return root + "/" + project + "/" + id
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

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-23T14:26:27+02:00","module_hash":"7b40e48309019f27c0c7e4459f010e5395747b9e5196e22a493b2f2f89b23793","functions":[{"id":"func/URL","name":"URL","line":24,"end_line":37,"hash":"02435bf9efa0da8437614814eb0491ea73522de78bfd0c2a4c25c16ba9b3f103"},{"id":"func/dashboardAddress","name":"dashboardAddress","line":41,"end_line":43,"hash":"b76dd39e121cb0ee924beff64dbc18ab006ee205f49714645c81417027ca8a86"},{"id":"func/NewAPI","name":"NewAPI","line":56,"end_line":58,"hash":"9f67e71d7be154031c299fc890b6a646278e87625f2bfe01612333013a6ec1ee"},{"id":"func/NewForgeAPI","name":"NewForgeAPI","line":61,"end_line":65,"hash":"beec8d70a512a1f1f9ddd9bebf8f513ec285b618b76f775ca461bda915472b5a"},{"id":"func/Approvals.Pending","name":"Approvals.Pending","line":75,"end_line":81,"hash":"ef0c4e28072730d93ed8f54ead0ee4dd26c3b3af3b9d92395a20279e4d7d9383"},{"id":"func/Approvals.Approve","name":"Approvals.Approve","line":84,"end_line":90,"hash":"dd5c9e0c32bf7fddd77ed9df501a1eb46c000b40809321994b846bad432beb61"},{"id":"func/Approvals.SendBack","name":"Approvals.SendBack","line":93,"end_line":99,"hash":"cca8f8845d3a0f192314e1e235deacd539c0da098b4c82930b6a587684e87dc7"},{"id":"func/Approvals.client","name":"Approvals.client","line":103,"end_line":109,"hash":"4211b729e1f50037a1951c0132a1b6c039dc8a376a94573aa5c18dc5824deca5"},{"id":"func/API.Chat","name":"API.Chat","line":131,"end_line":133,"hash":"398a79c219221477b37ca392b678c0590d51ac1b76519427fecf986c88b45fca"},{"id":"func/API.Approvals","name":"API.Approvals","line":136,"end_line":165,"hash":"c8fa181a635a4b6baca3c2cdd0c957abe272be1a3b7efcbd628dddfec10410ca"},{"id":"func/forgeKey","name":"forgeKey","line":169,"end_line":171,"hash":"af8ef28a37c8ca5da468f5fc9640b3d2b542bb246cb2e0e5f82536914c62500f"},{"id":"func/API.Approve","name":"API.Approve","line":175,"end_line":178,"hash":"8ef15c887c5b867ee8d942c11fd9de792523bee41abfa529c9c1bc601dc6a32c"},{"id":"func/API.SendBack","name":"API.SendBack","line":182,"end_line":185,"hash":"fe863288823571b281980c77e1ce6b1c9b1b5526d42e58f3626ab7826f6800b3"},{"id":"func/ApprovalRequest.gate","name":"ApprovalRequest.gate","line":189,"end_line":194,"hash":"11d7e24233706f967539b6d653cf6b1e6ba19c47bc181edb42cb330378f00ac9"},{"id":"func/API.call","name":"API.call","line":196,"end_line":217,"hash":"ed70d9387061331e57004fb0f855e2f5c2712bcb4aaf2f253f7d8f22ad155c2f"},{"id":"func/API.request","name":"API.request","line":221,"end_line":238,"hash":"4721157c005933e4f45d589fe9bb65e0dd3da721df8b2ecab475f46f3ad39b24"},{"id":"func/responseError","name":"responseError","line":242,"end_line":247,"hash":"efe5dae148dd33495d0d899815f1b887cdcc8075a74c262d501a55ef204ca4c3"}]}
// mutate4go-manifest-end
