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
