package dashboard

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

// Clarifications is one forge's clarifications, reached through its dashboard:
// the bridge asks the dashboard and never edits the forge's files itself.
type Clarifications struct {
	Root          string
	ConfiguredURL string
	// Project names the project when the dashboard serves one project root and
	// its clarification files do not carry a project field.
	Project string
}

// Pending reads the clarifications the dashboard is showing and the forge is
// still blocked on.
func (c Clarifications) Pending(ctx context.Context) ([]relay.Clarification, error) {
	api, err := c.client()
	if err != nil {
		return nil, err
	}
	pending, err := api.Clarifications(ctx)
	if err != nil {
		return nil, err
	}
	for index := range pending {
		if pending[index].Project == "" {
			pending[index].Project = c.Project
		}
	}
	return pending, nil
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

// AnswerTyped answers a project-chat clarification through the owning
// project's normal chat endpoint. A Family adapter resolves the one marked
// project below its root; a project adapter accepts only its own project.
func (c Clarifications) AnswerTyped(ctx context.Context, project, id, answer string) error {
	root, configuredURL, err := c.typedProject(project)
	if err != nil {
		return err
	}
	url, err := URL(root, configuredURL)
	if err != nil {
		return fmt.Errorf("find the dashboard for project %s: %w", project, err)
	}
	return NewForgeAPI(root, url).ChatWithClarification(ctx, answer, id)
}

func (c Clarifications) typedProject(project string) (string, string, error) {
	if err := validClarificationProject(project); err != nil {
		return "", "", err
	}
	if c.Project != "" {
		if project != c.Project {
			return "", "", fmt.Errorf(
				"typed clarification for project %s was read from project %s", project, c.Project)
		}
		return c.Root, c.ConfiguredURL, nil
	}
	return c.resolveTypedProject(project)
}

func validClarificationProject(project string) error {
	if project == "" || filepath.Base(project) != project || strings.ContainsAny(project, `/\`) {
		return fmt.Errorf("typed clarification has an invalid project %q", project)
	}
	return nil
}

func (c Clarifications) resolveTypedProject(project string) (string, string, error) {
	if _, err := os.Stat(filepath.Join(c.Root, ".swarmforge", "single-agent")); err == nil {
		if project != filepath.Base(c.Root) {
			return "", "", fmt.Errorf(
				"typed clarification for project %s was read from project %s", project, filepath.Base(c.Root))
		}
		return c.Root, c.ConfiguredURL, nil
	}
	if err := c.requireOpenProject(project); err != nil {
		return "", "", err
	}
	root := filepath.Join(c.Root, "projects", project)
	if _, err := os.Stat(filepath.Join(root, ".swarmforge", "single-agent")); err != nil {
		return "", "", fmt.Errorf("project %s is not one marked project under %s: %w", project, c.Root, err)
	}
	return root, "", nil
}

func (c Clarifications) requireOpenProject(project string) error {
	data, err := os.ReadFile(filepath.Join(c.Root, ".swarmforge", "open-projects"))
	if err != nil {
		return fmt.Errorf("read open projects for %s: %w", c.Root, err)
	}
	matches := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == project {
			matches++
		}
	}
	if matches != 1 {
		return fmt.Errorf("typed clarification project %s resolves to %d open projects under %s",
			project, matches, c.Root)
	}
	return nil
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

// Clarifications reads the clarifications the dashboard is showing.
func (a *API) Clarifications(ctx context.Context) ([]relay.Clarification, error) {
	state, err := a.State(ctx)
	if err != nil {
		return nil, err
	}
	return a.clarifications(state), nil
}

// clarifications derives the pending clarifications from a complete dashboard
// state.
func (a *API) clarifications(state State) []relay.Clarification {
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
	return pending
}

// AnswerClarification answers a clarification through the dashboard, so the
// dashboard applies everything answering means: the clarification is resolved
// and the blocked role is woken with the answer.
func (a *API) AnswerClarification(ctx context.Context, project, id, answer string) error {
	body := map[string]string{"id": id, "project": project, "text": answer}
	return a.call(ctx, http.MethodPost, "/api/clarifications/"+url.PathEscape(id)+"/answer", body, nil)
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-11-14T23:43:17+01:00","module_hash":"d8c4a44ce8cd5e665579bbde19a887167140d3b1311ebc36f81cac497d4ef8c1","functions":[{"id":"func/Clarifications.Pending","name":"Clarifications.Pending","line":24,"end_line":39,"hash":"3d59f2037ea31827414df5a5a4e8132745c3396254e3c40135eae6bfc094af11"},{"id":"func/Clarifications.Answer","name":"Clarifications.Answer","line":43,"end_line":49,"hash":"d0164fc07e33793682064d65c168210257c720d6ef19c9bc797582bf0512e7bb"},{"id":"func/Clarifications.client","name":"Clarifications.client","line":53,"end_line":59,"hash":"4f7063d0149b103a239a33acc268240d8e63cd577326e7a515545701ffeb5546"},{"id":"func/API.Clarifications","name":"API.Clarifications","line":71,"end_line":77,"hash":"5948c65df4496ab36d0a5f7cda9d943ff4a48fe54d8a7c5ac68c8c351a279512"},{"id":"func/API.clarifications","name":"API.clarifications","line":81,"end_line":96,"hash":"696c0e7de92a24ca3d3cd3bd995caa2e3b060ac691c2b5e74d53251167183d6a"},{"id":"func/API.AnswerClarification","name":"API.AnswerClarification","line":101,"end_line":104,"hash":"286ecaa6aaa5f107300c4fc115d8a525c4c1dae93dd4275284438d1026887a35"}]}
// mutate4go-manifest-end
