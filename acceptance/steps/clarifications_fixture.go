package steps

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/internal/dashboard"
)

// clarificationProjectPaths are the places a fixture project keeps its
// clarifications, the way the forge's dashboard holds them.
type clarificationProjectPaths struct {
	pending string
	done    string
}

func clarificationPaths(root, project string) clarificationProjectPaths {
	base := filepath.Join(root, "projects", project, ".swarmforge", "dashboard", "clarifications")
	return clarificationProjectPaths{pending: filepath.Join(base, "pending"), done: filepath.Join(base, "done")}
}

// clarificationID is the id the fixture's clarifications carry.
func clarificationID(project string) string {
	return "clar-" + project
}

func clarificationFactKey(project string) string {
	return "typed:" + project + ":" + clarificationID(project)
}

// seedPendingClarification asks the owning project dashboard for a typed
// clarification through the same command an agent uses.
func seedPendingClarification(w *World, role, project, question string) error {
	root, err := singleForge(w)
	if err != nil {
		return err
	}
	projectRoot := filepath.Join(root, "projects", project)
	if err := fixtures.PrepareSwarmRoot(projectRoot); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(projectRoot, ".swarmforge"), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".swarmforge", "single-agent"), nil, 0o644); err != nil {
		return err
	}
	if _, ok := w.running[project]; !ok {
		running, err := fixtures.StartDashboard(context.Background(), projectRoot)
		if err != nil {
			return err
		}
		w.running[project] = running
	}
	if err := markProjectOpen(root); err != nil {
		return err
	}
	questionFile := filepath.Join(projectRoot, "tmp", "clarification-question.txt")
	if err := os.MkdirAll(filepath.Dir(questionFile), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(questionFile, []byte(question+"\n"), 0o644); err != nil {
		return err
	}
	script := filepath.Join(fixtures.ProjectRoot(), "swarmforge", "scripts", "pack_dashboard_request.sh")
	command := exec.Command(script, "clarify-chat", clarificationID(project), questionFile, "--root", projectRoot)
	command.Dir = projectRoot
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create typed clarification for %s through its dashboard: %w: %s",
			project, err, strings.TrimSpace(string(output)))
	}
	_ = role
	return nil
}

// pendingClarification seeds a clarification the forge's dashboard is showing.
func pendingClarification(_ context.Context, world any, captures []string) error {
	return seedPendingClarification(world.(*World), captures[1], captures[2], captures[3])
}

// clarificationAnswered checks the forge recorded the operator's answer
// exactly, without the question they quoted back.
func clarificationAnswered(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := singleForge(w)
	if err != nil {
		return err
	}
	ctx, cancel := stepContext()
	defer cancel()
	project, answer := captures[1], captures[2]
	return waitFor(ctx, fmt.Sprintf("the forge never recorded the clarification for %s as answered with %q", project, answer), func() (bool, error) {
		for _, directory := range []string{"pending", "done"} {
			files, err := filepath.Glob(filepath.Join(
				root, "projects", project, ".swarmforge", "dashboard", "requests", directory, "*.request"))
			if err != nil {
				return false, err
			}
			for _, file := range files {
				data, err := os.ReadFile(file)
				if err != nil {
					return false, err
				}
				headers, body, found := strings.Cut(string(data), "\n\n")
				if !found || !strings.Contains(headers, "clarification_id: "+clarificationID(project)) {
					continue
				}
				if strings.TrimSpace(body) == answer {
					return true, nil
				}
			}
		}
		return false, nil
	})
}

// recordedAnswer is the response a done clarification carries.
func recordedAnswer(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if value, found := strings.CutPrefix(line, "response: "); found {
			return strings.ReplaceAll(value, `\n`, "\n")
		}
	}
	return ""
}

// blockedRoleWoken checks the dashboard woke the blocked role's own pane with
// the answer.
func blockedRoleWoken(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	root, err := singleForge(w)
	if err != nil {
		return err
	}
	role, answer := captures[1], captures[2]
	return waitFor(ctx, fmt.Sprintf("the blocked role %s was never woken with the answer %q", role, answer), func() (bool, error) {
		for _, running := range w.running {
			lines, err := running.Typed()
			if err != nil {
				return false, err
			}
			for _, line := range lines {
				if strings.Contains(line, "Clarification requested from: "+role) &&
					strings.Contains(strings.Join(lines, "\n"), "Answer:\n"+answer) {
					return true, nil
				}
			}
			for _, line := range lines {
				if strings.TrimSpace(line) == answer {
					return true, nil
				}
			}
		}
		for _, directory := range []string{"pending", "done"} {
			files, err := filepath.Glob(filepath.Join(root, "projects", "*", ".swarmforge",
				"dashboard", "requests", directory, "*.request"))
			if err != nil {
				return false, err
			}
			for _, file := range files {
				data, err := os.ReadFile(file)
				if err != nil {
					return false, err
				}
				_, body, found := strings.Cut(string(data), "\n\n")
				if found && strings.TrimSpace(body) == answer {
					return true, nil
				}
			}
		}
		return false, nil
	})
}

// clarificationStillWaiting checks the forge is still waiting for the answer.
func clarificationStillWaiting(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	root, err := singleForge(w)
	if err != nil {
		return err
	}
	// Wait for the bridge to have handled everything before the operator's
	// events, so an answer it should not have carried back shows up here.
	if err := w.caughtUp(ctx); err != nil {
		return err
	}
	project := captures[1]
	pending := filepath.Join(root, "projects", project, ".swarmforge", "dashboard", "requests", "pending",
		clarificationID(project)+".request")
	if _, err := os.Stat(pending); err != nil {
		return fmt.Errorf("the clarification for %s is no longer pending: %v", project, err)
	}
	return nil
}

// operatorAnsweredFromTheDesktop resolves the clarification the way the
// desktop dashboard does.
func operatorAnsweredFromTheDesktop(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := singleForge(w)
	if err != nil {
		return err
	}
	ctx, cancel := stepContext()
	defer cancel()
	url, err := dashboard.URL(filepath.Join(root, "projects", captures[1]), "")
	if err != nil {
		return err
	}
	return dashboard.NewAPI(url).ChatWithClarification(ctx, captures[2], clarificationID(captures[1]))
}
