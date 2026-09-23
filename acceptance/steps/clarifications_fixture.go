package steps

import (
	"context"
	"fmt"
	"os"
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
	return "clarification-" + project
}

// seedPendingClarification seeds a clarification the way the forge's dashboard
// holds it: a pending clarification in the project, which the dashboard lists
// for the bridge to carry into the room.
func seedPendingClarification(w *World, role, project, question string) error {
	root, err := singleForge(w)
	if err != nil {
		return err
	}
	projectRoot := filepath.Join(root, "projects", project)
	// A project is a swarm root of its own: the dashboard wakes the blocked
	// role in the role's own pane, which is inside the project.
	if err := fixtures.PrepareSwarmRoot(projectRoot); err != nil {
		return err
	}
	if err := os.MkdirAll(clarificationPaths(root, project).pending, 0o755); err != nil {
		return err
	}
	headers := []string{
		"id: " + clarificationID(project),
		"status: pending",
		"role: " + role,
		"created_at: 2026-09-23T09:00:00Z",
	}
	content := strings.Join(headers, "\n") + "\n\n" + question + "\n"
	if err := os.WriteFile(filepath.Join(clarificationPaths(root, project).pending, clarificationID(project)+".request"), []byte(content), 0o644); err != nil {
		return err
	}
	return markProjectOpen(root)
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
	done := filepath.Join(clarificationPaths(root, project).done, clarificationID(project)+".request")
	return waitFor(ctx, fmt.Sprintf("the forge never recorded the clarification for %s as answered with %q", project, answer), func() (bool, error) {
		data, err := os.ReadFile(done)
		if err != nil {
			return false, nil
		}
		return recordedAnswer(string(data)) == answer, nil
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
	running, ok := w.running[filepath.Base(root)]
	if !ok {
		return fmt.Errorf("the fixture forge root %s does not have its dashboard running", root)
	}
	role, answer := captures[1], captures[2]
	// The dashboard writes the wake into the pane through its tmux stub, which
	// records the arguments of the call: the answer is one line of it.
	typed := "Answer:\\n" + answer
	return waitFor(ctx, fmt.Sprintf("the blocked role %s was never woken with the answer %q", role, answer), func() (bool, error) {
		lines, err := running.Typed()
		if err != nil {
			return false, err
		}
		for _, line := range lines {
			if strings.Contains(line, "Clarification requested from: "+role) && strings.Contains(line, typed) {
				return true, nil
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
	pending := filepath.Join(clarificationPaths(root, project).pending, clarificationID(project)+".request")
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
	url, err := dashboard.URL(root, "")
	if err != nil {
		return err
	}
	return dashboard.NewAPI(url).AnswerClarification(ctx, captures[1], clarificationID(captures[1]), captures[2])
}
