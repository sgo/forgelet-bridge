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
		return nil, fmt.Errorf("find the dashboard for %s: %w", a.Root, err)
	}
	return api.Approvals(ctx)
}

// Resolved reads how the dashboard resolved the approvals it made away from
// the room, keyed by approval: the ending the room reports for an approval the
// bridge did not carry out itself.
func (a Approvals) Resolved(ctx context.Context) (map[string]string, error) {
	api, err := a.client()
	if err != nil {
		return nil, fmt.Errorf("find the dashboard for %s: %w", a.Root, err)
	}
	return api.ResolvedApprovals(ctx)
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
	Approvals         []ApprovalRequest         `json:"approvals"`
	ResolvedApprovals []ResolvedApprovalRequest `json:"resolved_approvals"`
	Clarifications    []ClarificationRequest    `json:"clarifications"`
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

// ResolvedApprovalRequest is one approval the dashboard resolved away from the
// room, as the dashboard wrote it down: the project it belongs to, its id, and
// how it ended, worded the way the bridge words its own resolutions.
type ResolvedApprovalRequest struct {
	ID         string `json:"id"`
	Project    string `json:"project"`
	Resolution string `json:"resolution"`
}

// Chat asks the dashboard to take a chat message the way its clients give it:
// the dashboard queues the request and wakes the lieutenant, which is why the
// bridge must not write the queue itself.
func (a *API) Chat(ctx context.Context, text string) error {
	return a.call(ctx, http.MethodPost, "/api/chat", map[string]string{"text": text}, nil)
}

// ChatWithClarification answers a typed project-chat clarification through the
// same endpoint clients use, so the dashboard marks it answered and wakes the
// owning project agent.
func (a *API) ChatWithClarification(ctx context.Context, text, clarificationID string) error {
	body := map[string]string{"text": text, "clarification_id": clarificationID}
	return a.call(ctx, http.MethodPost, "/api/chat", body, nil)
}

// Approvals reads the approvals the dashboard is showing.
func (a *API) Approvals(ctx context.Context) ([]relay.Approval, error) {
	state, err := a.State(ctx)
	if err != nil {
		return nil, err
	}
	return a.approvals(state), nil
}

// State reads the complete dashboard payload once.
func (a *API) State(ctx context.Context) (State, error) {
	var state State
	if err := a.call(ctx, http.MethodGet, "/api/state", nil, &state); err != nil {
		return State{}, fmt.Errorf("read dashboard state: %w", err)
	}
	return state, nil
}

// approvals derives the pending approvals from a complete dashboard state.
func (a *API) approvals(state State) []relay.Approval {
	approvals := make([]relay.Approval, 0, len(state.Approvals))
	for _, request := range state.Approvals {
		approvals = append(approvals, a.approval(request))
	}
	return approvals
}

// approval adds the handoff's roles and changed files to the safe approval
// identity the dashboard reports.
func (a *API) approval(request ApprovalRequest) relay.Approval {
	approval := relay.Approval{
		Key:       forgeKey(a.root, request.Project, request.ID),
		Project:   request.Project,
		ID:        request.ID,
		Card:      request.Card,
		Gate:      request.gate(),
		Artifacts: request.Artifacts,
	}
	if details, err := readHandoff(a.root, request.Project, request.ID); err == nil {
		if details.from != "" && details.to != "" {
			approval.Gate = details.from + " → " + details.to
		}
		if len(details.artifacts) > 0 {
			approval.Artifacts = details.artifacts
		}
	}
	return approval
}

// forgeKey names one item inside one forge: what the bridge remembers is per
// forge, so the same project and id in two forges are two items, not one.
func forgeKey(root, project, id string) string {
	return root + "/" + project + "/" + id
}

// ResolvedApprovals reads how the dashboard resolved the approvals it made away
// from the room, keyed the way the bridge keys an approval, so the room can
// report the ending the desk recorded rather than only that the approval is
// gone.
func (a *API) ResolvedApprovals(ctx context.Context) (map[string]string, error) {
	state, err := a.State(ctx)
	if err != nil {
		return nil, err
	}
	return a.resolvedApprovals(state), nil
}

// resolvedApprovals derives the desk resolutions from a complete dashboard
// state.
func (a *API) resolvedApprovals(state State) map[string]string {
	resolved := make(map[string]string, len(state.ResolvedApprovals))
	for _, request := range state.ResolvedApprovals {
		if request.ID == "" || request.Resolution == "" {
			continue
		}
		resolved[forgeKey(a.root, request.Project, request.ID)] = request.Resolution
	}
	return resolved
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
	_, _, data, err := a.exchange(ctx, method, path, body, 0)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(data, out)
}

// exchange performs one dashboard request and returns its headers, status and
// body. A positive limit bounds how much of the response is accepted.
func (a *API) exchange(ctx context.Context, method, path string, body any, limit int64) (http.Header, int, []byte, error) {
	request, err := a.request(ctx, method, path, body)
	if err != nil {
		return nil, 0, nil, err
	}
	response, err := a.client.Do(request)
	if err != nil {
		return nil, 0, nil, err
	}
	defer response.Body.Close()
	data, err := responseBody(method, path, response, limit)
	if err != nil {
		return response.Header, response.StatusCode, nil, err
	}
	return response.Header, response.StatusCode, data, nil
}

// responseBody reads a response within the caller's bound and turns an
// unsuccessful answer into the dashboard's error.
func responseBody(method, path string, response *http.Response, limit int64) ([]byte, error) {
	data, err := io.ReadAll(limitedReader(response, limit))
	if err != nil {
		return nil, err
	}
	if err := responseSizeError(method, path, limit, len(data)); err != nil {
		return nil, err
	}
	if err := responseError(method, path, response, data); err != nil {
		return nil, err
	}
	return data, nil
}

func limitedReader(response *http.Response, limit int64) io.Reader {
	if limit > 0 {
		return io.LimitReader(response.Body, limit+1)
	}
	return response.Body
}

func responseSizeError(method, path string, limit int64, size int) error {
	if limit > 0 && int64(size) > limit {
		return fmt.Errorf("%s %s: response exceeds %d bytes", method, path, limit)
	}
	return nil
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
// {"version":1,"tested_at":"2026-11-16T17:32:38+01:00","module_hash":"3b95fb152ffaca17a90f4ac5fe303790496c144780f380a7d1db8535783e53b4","functions":[{"id":"func/URL","name":"URL","line":24,"end_line":37,"hash":"02435bf9efa0da8437614814eb0491ea73522de78bfd0c2a4c25c16ba9b3f103"},{"id":"func/dashboardAddress","name":"dashboardAddress","line":41,"end_line":43,"hash":"b76dd39e121cb0ee924beff64dbc18ab006ee205f49714645c81417027ca8a86"},{"id":"func/NewAPI","name":"NewAPI","line":56,"end_line":58,"hash":"9f67e71d7be154031c299fc890b6a646278e87625f2bfe01612333013a6ec1ee"},{"id":"func/NewForgeAPI","name":"NewForgeAPI","line":61,"end_line":65,"hash":"beec8d70a512a1f1f9ddd9bebf8f513ec285b618b76f775ca461bda915472b5a"},{"id":"func/Approvals.Pending","name":"Approvals.Pending","line":75,"end_line":81,"hash":"761383ec898c5bc2ebc2abbd07ffb324856e10a6e610d047096954ed9da1e22b"},{"id":"func/Approvals.Resolved","name":"Approvals.Resolved","line":86,"end_line":92,"hash":"6393e7f3bc8cec2f0bae7e839483bbc1d6b5833f3c0573e13a3517699c7b13fb"},{"id":"func/Approvals.Approve","name":"Approvals.Approve","line":95,"end_line":101,"hash":"dd5c9e0c32bf7fddd77ed9df501a1eb46c000b40809321994b846bad432beb61"},{"id":"func/Approvals.SendBack","name":"Approvals.SendBack","line":104,"end_line":110,"hash":"cca8f8845d3a0f192314e1e235deacd539c0da098b4c82930b6a587684e87dc7"},{"id":"func/Approvals.client","name":"Approvals.client","line":114,"end_line":120,"hash":"4211b729e1f50037a1951c0132a1b6c039dc8a376a94573aa5c18dc5824deca5"},{"id":"func/API.Chat","name":"API.Chat","line":153,"end_line":155,"hash":"398a79c219221477b37ca392b678c0590d51ac1b76519427fecf986c88b45fca"},{"id":"func/API.Approvals","name":"API.Approvals","line":158,"end_line":164,"hash":"aca6d6ee166ffd50c7412203ec207b0cd215c1c345a36bd503e78a2b28a732cf"},{"id":"func/API.State","name":"API.State","line":167,"end_line":173,"hash":"adb7af5fb376ef83932531b55dee9c1f4c57b3543683a040b414d249693eb1c4"},{"id":"func/API.compatibilityState","name":"API.compatibilityState","line":177,"end_line":187,"hash":"260268a0f2ad7372f907df3434018dafc530b357b615c76eec52f5c5497cf895"},{"id":"func/API.approvals","name":"API.approvals","line":190,"end_line":196,"hash":"52646b0dda70803655af1d8fe5620da8213e188ad4186eb6398e6faddedbdb7e"},{"id":"func/API.approval","name":"API.approval","line":200,"end_line":218,"hash":"56b91cb0194d1911b077db11e0fe5af7e813ca9ba81c150abb0f12e182a13bbd"},{"id":"func/forgeKey","name":"forgeKey","line":222,"end_line":224,"hash":"af8ef28a37c8ca5da468f5fc9640b3d2b542bb246cb2e0e5f82536914c62500f"},{"id":"func/API.ResolvedApprovals","name":"API.ResolvedApprovals","line":230,"end_line":236,"hash":"fb05dc44311251761b19f65b5fc659bec892490be1e5d73ff1ce95e2ca7fb1a1"},{"id":"func/API.resolvedApprovals","name":"API.resolvedApprovals","line":240,"end_line":249,"hash":"29cfa7b23dfebde2fa2a490bf668fb96c6f0bb016782089cd5a2c7d405d3a077"},{"id":"func/API.Approve","name":"API.Approve","line":253,"end_line":256,"hash":"8ef15c887c5b867ee8d942c11fd9de792523bee41abfa529c9c1bc601dc6a32c"},{"id":"func/API.SendBack","name":"API.SendBack","line":260,"end_line":263,"hash":"fe863288823571b281980c77e1ce6b1c9b1b5526d42e58f3626ab7826f6800b3"},{"id":"func/ApprovalRequest.gate","name":"ApprovalRequest.gate","line":267,"end_line":272,"hash":"11d7e24233706f967539b6d653cf6b1e6ba19c47bc181edb42cb330378f00ac9"},{"id":"func/API.call","name":"API.call","line":274,"end_line":283,"hash":"797627578b8e3338ff8a3cf48abc21a4b49b4e03a6bea313c97c0a5a0a590c1c"},{"id":"func/API.exchange","name":"API.exchange","line":287,"end_line":302,"hash":"cb7a484cb47169140ac04b6f3e4ca7b9e160831d078e52cef63c18c435f398e4"},{"id":"func/responseBody","name":"responseBody","line":306,"end_line":318,"hash":"170c57f98f78145d17b5d1c2b3bd7b1c2b90c7adf1d11b5fee20f8d09a6e6b0c"},{"id":"func/limitedReader","name":"limitedReader","line":320,"end_line":325,"hash":"2bf79a43e23ff57963754bcf081c03dd1d503b88d78d61bca95d4b94fa2bddcf"},{"id":"func/responseSizeError","name":"responseSizeError","line":327,"end_line":332,"hash":"324875ada6684d2e2fd59784a3e2c0e9a775405443b6eaa60159d5bb01391044"},{"id":"func/API.request","name":"API.request","line":336,"end_line":353,"hash":"4721157c005933e4f45d589fe9bb65e0dd3da721df8b2ecab475f46f3ad39b24"},{"id":"func/responseError","name":"responseError","line":357,"end_line":362,"hash":"efe5dae148dd33495d0d899815f1b887cdcc8075a74c262d501a55ef204ca4c3"}]}
// mutate4go-manifest-end
