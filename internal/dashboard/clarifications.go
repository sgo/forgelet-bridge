package dashboard

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

// Clarifications is one forge's clarifications, reached through its dashboard:
// the bridge asks the dashboard and never edits the forge's files itself.
type Clarifications struct {
	Root          string
	ConfiguredURL string
}

// Pending reads the clarifications the dashboard is showing and the forge is
// still blocked on.
func (c Clarifications) Pending(ctx context.Context) ([]relay.Clarification, error) {
	api, err := c.client()
	if err != nil {
		return nil, err
	}
	return api.Clarifications(ctx)
}

// Answer asks the dashboard to answer a clarification, which is what wakes the
// blocked role with the operator's words.
func (c Clarifications) Answer(ctx context.Context, project, id, answer string) error {
	api, err := c.client()
	if err != nil {
		return err
	}
	return api.AnswerClarification(ctx, project, id, answer)
}

// client is the dashboard of this forge, whose address is looked up every time
// so a dashboard that moved is picked up.
func (c Clarifications) client() (*API, error) {
	url, err := URL(c.Root, c.ConfiguredURL)
	if err != nil {
		return nil, err
	}
	return NewForgeAPI(c.Root, url), nil
}

// ClarificationRequest is one clarification the dashboard is showing.
type ClarificationRequest struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Project string `json:"project"`
	Role    string `json:"role"`
	Body    string `json:"body"`
}

// ClarificationState is what the dashboard reports about its clarifications.
type ClarificationState struct {
	Clarifications []ClarificationRequest `json:"clarifications"`
}

// Clarifications reads the clarifications the dashboard is showing.
func (a *API) Clarifications(ctx context.Context) ([]relay.Clarification, error) {
	var state ClarificationState
	if err := a.call(ctx, http.MethodGet, "/api/state", nil, &state); err != nil {
		return nil, err
	}
	var pending []relay.Clarification
	for _, request := range state.Clarifications {
		if request.Status != "pending" {
			continue
		}
		pending = append(pending, relay.Clarification{
			Key:      forgeKey(a.root, request.Project, request.ID),
			ID:       request.ID,
			Project:  request.Project,
			Role:     request.Role,
			Question: strings.TrimSpace(request.Body),
		})
	}
	return pending, nil
}

// AnswerClarification answers a clarification through the dashboard, so the
// dashboard applies everything answering means: the clarification is resolved
// and the blocked role is woken with the answer.
func (a *API) AnswerClarification(ctx context.Context, project, id, answer string) error {
	body := map[string]string{"id": id, "project": project, "text": answer}
	return a.call(ctx, http.MethodPost, "/api/clarifications/"+url.PathEscape(id)+"/answer", body, nil)
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-23T14:23:24+02:00","module_hash":"c8fb73c1f524a469798b42a7a55582aa789aba3dd70cc097bbbd5c7afa59bdcf","functions":[{"id":"func/Clarifications.Pending","name":"Clarifications.Pending","line":21,"end_line":27,"hash":"39cbb6f01abc5e1dc91ca551a71c0b13d9f8a1c06cb8245557fca6d1ed2accfc"},{"id":"func/Clarifications.Answer","name":"Clarifications.Answer","line":31,"end_line":37,"hash":"d0164fc07e33793682064d65c168210257c720d6ef19c9bc797582bf0512e7bb"},{"id":"func/Clarifications.client","name":"Clarifications.client","line":41,"end_line":47,"hash":"4f7063d0149b103a239a33acc268240d8e63cd577326e7a515545701ffeb5546"},{"id":"func/API.Clarifications","name":"API.Clarifications","line":64,"end_line":83,"hash":"741352c9e0f73a3e3df4cc26dc5d99019e0c9a7795e63cfdd77783eb9e5b0d7a"},{"id":"func/API.AnswerClarification","name":"API.AnswerClarification","line":88,"end_line":91,"hash":"286ecaa6aaa5f107300c4fc115d8a525c4c1dae93dd4275284438d1026887a35"}]}
// mutate4go-manifest-end
