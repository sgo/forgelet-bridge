// Command typed-clarification-e2e proves the bridge's typed answer route
// against the real project and Family dashboards, without a Matrix homeserver.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/internal/dashboard"
)

type report struct {
	ClarificationID   string `json:"clarification_id"`
	Project           string `json:"project"`
	InitialLifecycle  string `json:"initial_lifecycle"`
	InitialAttention  string `json:"initial_attention"`
	FinalLifecycle    string `json:"final_lifecycle"`
	Excerpt           string `json:"question_excerpt"`
	ProjectAgentWoken bool   `json:"project_agent_woken"`
	AttentionCleared  bool   `json:"attention_cleared"`
}

func main() {
	if err := runCommand(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runCommand(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("typed-clarification-e2e", flag.ContinueOnError)
	output := flags.String("output", "", "optional JSON report path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	result, err := run(context.Background())
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if *output != "" {
		if err := os.MkdirAll(filepath.Dir(*output), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(*output, encoded, 0o644); err != nil {
			return err
		}
	}
	_, err = stdout.Write(encoded)
	return err
}

func run(ctx context.Context) (report, error) {
	work, err := os.MkdirTemp("", "forgelet-typed-clarification-")
	if err != nil {
		return report{}, err
	}
	if os.Getenv("FORGELET_TYPED_E2E_KEEP") == "" {
		defer os.RemoveAll(work)
	} else {
		fmt.Fprintln(os.Stderr, "kept fixture at", work)
	}

	forgeRoot := filepath.Join(work, "forge-a")
	if err := prepareDashboardRoot(forgeRoot); err != nil {
		return report{}, err
	}
	family, err := fixtures.StartDashboard(ctx, forgeRoot)
	if err != nil {
		return report{}, fmt.Errorf("start Family dashboard: %w", err)
	}
	defer family.Stop()

	const (
		project         = "project-alpha"
		clarificationID = "clar-e2e"
		answer          = "run qa"
		unrelatedPath   = "/Users/operator/private/project"
		unrelatedToken  = "ghp_1234567890abcdefghijklmnopqrstuvwxyz"
		questionBody    = "Which stage should run next?\n"
	)
	projectRoot := filepath.Join(forgeRoot, "projects", project)
	if err := os.MkdirAll(filepath.Join(projectRoot, ".swarmforge"), 0o755); err != nil {
		return report{}, err
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".swarmforge", "single-agent"), nil, 0o644); err != nil {
		return report{}, err
	}
	if err := prepareDashboardRoot(projectRoot); err != nil {
		return report{}, err
	}
	projectDashboard, err := fixtures.StartDashboard(ctx, projectRoot)
	if err != nil {
		return report{}, fmt.Errorf("start project dashboard: %w", err)
	}
	defer projectDashboard.Stop()
	if err := writeSingleAgentRole(projectRoot); err != nil {
		return report{}, err
	}
	if err := os.WriteFile(filepath.Join(forgeRoot, ".swarmforge", "open-projects"),
		[]byte(project+"\n"), 0o644); err != nil {
		return report{}, err
	}

	questionFile := filepath.Join(work, "question.md")
	if err := os.WriteFile(questionFile,
		[]byte(questionBody+"Inspect "+unrelatedPath+" with "+unrelatedToken+"\n"), 0o644); err != nil {
		return report{}, err
	}
	command := exec.CommandContext(ctx,
		"bb", filepath.Join(fixtures.ProjectRoot(), "swarmforge", "scripts", "pack_dashboard_request.bb"),
		"clarify-chat", clarificationID, questionFile, "--root", projectRoot)
	if out, err := command.CombinedOutput(); err != nil {
		return report{}, fmt.Errorf("create typed clarification: %w: %s", err, out)
	}

	projectAPI := dashboard.NewForgeAPI(projectRoot, projectDashboard.URL)
	initial, err := waitForTyped(ctx, projectAPI, clarificationID, "open")
	if err != nil {
		return report{}, fmt.Errorf("%w; project state: %s; project dashboard log:\n%s",
			err, dashboardStateError(ctx, projectDashboard.URL), logTail(projectRoot, "dashboard.log"))
	}
	if initial.Attention != "unseen" || initial.Question == "" ||
		strings.Contains(initial.Question, unrelatedPath) ||
		strings.Contains(initial.Question, unrelatedToken) {
		return report{}, fmt.Errorf("project projection = %+v, want a bounded redacted unseen question", initial)
	}

	clarifications := dashboard.Clarifications{Root: forgeRoot}
	if err := clarifications.AnswerTyped(ctx, project, clarificationID, answer); err != nil {
		return report{}, fmt.Errorf("answer through Family adapter: %w", err)
	}
	final, err := waitForTyped(ctx, projectAPI, clarificationID, "answered")
	if err != nil {
		return report{}, err
	}
	woken := projectAnswerQueued(projectRoot, clarificationID, answer)
	attentionCleared, err := projectAttentionCleared(ctx, projectDashboard.URL)
	if err != nil {
		return report{}, err
	}
	if !woken || !attentionCleared {
		return report{}, fmt.Errorf("answer wake = %t, attention cleared = %t, want both true",
			woken, attentionCleared)
	}

	return report{
		ClarificationID:   clarificationID,
		Project:           project,
		InitialLifecycle:  initial.Lifecycle,
		InitialAttention:  initial.Attention,
		FinalLifecycle:    final.Lifecycle,
		Excerpt:           initial.Question,
		ProjectAgentWoken: woken,
		AttentionCleared:  attentionCleared,
	}, nil
}

func projectAnswerQueued(projectRoot, clarificationID, answer string) bool {
	dir := filepath.Join(projectRoot, ".swarmforge", "dashboard", "requests", "pending")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		content := string(data)
		if !strings.Contains(content, "clarification_id: "+clarificationID+"\n") {
			continue
		}
		_, body, found := strings.Cut(content, "\n\n")
		if found && strings.TrimSpace(body) == answer {
			return true
		}
	}
	return false
}

func writeSingleAgentRole(root string) error {
	line := strings.Join([]string{
		"agent", "master", root, "forge-e2e-agent", "Agent", "codex", "task",
	}, "\t")
	return os.WriteFile(filepath.Join(root, ".swarmforge", "roles.tsv"), []byte(line+"\n"), 0o644)
}

func prepareDashboardRoot(root string) error {
	if err := fixtures.PrepareSwarmRoot(root); err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(root, ".swarmforge", "roles.tsv"))
	if err != nil {
		return err
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		columns := strings.Split(line, "\t")
		if len(columns) < 3 {
			continue
		}
		for _, dir := range []string{"outbox", "sent", "failed", "inbox/new"} {
			if err := os.MkdirAll(filepath.Join(columns[2], ".swarmforge", "handoffs", dir), 0o755); err != nil {
				return err
			}
		}
	}
	return os.MkdirAll(filepath.Join(root, ".swarmforge", "handoffs", "pending_approval"), 0o755)
}

func dashboardStateError(ctx context.Context, baseURL string) string {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/state", nil)
	if err != nil {
		return err.Error()
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err.Error()
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	return fmt.Sprintf("%s %s", response.Status, strings.TrimSpace(string(body)))
}

func logTail(root, name string) string {
	data, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		return err.Error()
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) > 20 {
		lines = lines[len(lines)-20:]
	}
	return strings.Join(lines, "\n")
}

type typedRow struct {
	ID        string
	Lifecycle string
	Attention string
	Question  string
}

func waitForTyped(ctx context.Context, api *dashboard.API, id, lifecycle string) (typedRow, error) {
	deadline := time.Now().Add(30 * time.Second)
	var last error
	for time.Now().Before(deadline) {
		state, err := api.AutomationState(ctx)
		if err == nil {
			for _, item := range state.Clarifications.Typed.Items {
				if item.ClarificationID == id && item.Status == lifecycle && item.QuestionExcerpt != nil {
					return typedRow{
						ID: id, Lifecycle: item.Status, Attention: item.Attention,
						Question: strings.TrimSpace(*item.QuestionExcerpt),
					}, nil
				}
			}
		} else {
			last = err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return typedRow{}, fmt.Errorf("typed clarification %s never became %s: %w", id, lifecycle, last)
}

func projectAttentionCleared(ctx context.Context, baseURL string) (bool, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/state", nil)
	if err != nil {
		return false, err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return false, fmt.Errorf("project state returned %s", response.Status)
	}
	var state struct {
		ClarificationAttention struct {
			Unresolved int `json:"unresolved"`
		} `json:"clarification_attention"`
	}
	if err := json.NewDecoder(response.Body).Decode(&state); err != nil {
		return false, err
	}
	return state.ClarificationAttention.Unresolved == 0, nil
}
