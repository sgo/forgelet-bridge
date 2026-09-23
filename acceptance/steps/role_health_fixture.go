package steps

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// claudeProjects is where the fixture keeps the transcript record the check
// reads to decide whether a claude session is working. It lives inside the
// fixture so no developer's own transcript decides what a scenario sees.
func (w *World) claudeProjects() string {
	return filepath.Join(w.workDir, "claude-projects")
}

// pathOf is one named project of a fixture forge root.
func (w *World) pathOf(forge, project string) (string, error) {
	root, err := w.forgeRootOf(forge)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "projects", project), nil
}

// roleWorktree is the tree one role of a project works in.
func roleWorktree(projectDir, role string) (string, error) {
	return roleColumn(projectDir, role, 2)
}

// projectHandedCardToRole puts the card's note in the role's own inbox, the way
// the forge hands work over: the role has it and has gone quiet on it.
func projectHandedCardToRole(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	projectDir, err := w.pathOf(captures[2], captures[1])
	if err != nil {
		return err
	}
	card, role := captures[3], captures[4]
	worktree, err := roleWorktree(projectDir, role)
	if err != nil {
		return err
	}
	if err := quietWorktree(worktree); err != nil {
		return err
	}
	return writeFile(filepath.Join(worktree, ".swarmforge", "handoffs", "inbox", "in_process", "50_"+card+".handoff"),
		"task: "+card+"\n")
}

// quietWorktree gives a role's tree a history of its own whose last commit is
// long past, the way a role that stopped while holding a card looks: without
// it the check would read the fixture's own fresh commit as the role's work.
func quietWorktree(worktree string) error {
	if _, err := os.Stat(filepath.Join(worktree, ".git")); err == nil {
		return nil
	}
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		return err
	}
	past := time.Now().Add(-30 * time.Minute).Format(time.RFC3339)
	for _, args := range [][]string{
		{"init", "--quiet"},
		{"-c", "user.email=fixture@example.org", "-c", "user.name=Fixture",
			"commit", "--quiet", "--allow-empty", "-m", "fixture worktree"},
	} {
		command := exec.Command("git", append([]string{"-C", worktree}, args...)...)
		command.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+past, "GIT_COMMITTER_DATE="+past)
		if out, err := command.CombinedOutput(); err != nil {
			return fmt.Errorf("the fixture could not make the role's tree quiet: git %s: %w: %s",
				strings.Join(args, " "), err, out)
		}
	}
	return nil
}

// projectHasOpenClarification records a clarification the role is waiting on,
// in the project's own store, where the check reads it.
func projectHasOpenClarification(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	projectDir, err := w.pathOf(captures[2], captures[1])
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(projectDir, ".swarmforge", "dashboard", "clarifications", "pending", "req-01.request"),
		"role: "+captures[3]+"\nquestion: which lane should the refund card start in?\n")
}

// forgeGivesWorkingSession starts the role sessions a forge is running and
// gives the named role a session the check reads as working: its own agent's
// record says so, while the pane shows a version rather than the agent's name.
func forgeGivesWorkingSession(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := w.forgeRootOf(captures[1])
	if err != nil {
		return err
	}
	projects, err := projectsOf(root)
	if err != nil {
		return err
	}
	if len(projects) != 1 {
		return fmt.Errorf("the forge root %s holds %d projects, want exactly one here", captures[1], len(projects))
	}
	role := captures[2]
	if err := w.serveRoleSessions(projects[0], role); err != nil {
		return err
	}
	worktree, err := roleWorktree(projects[0], role)
	if err != nil {
		return err
	}
	transcript := filepath.Join(w.claudeProjects(), dashedPath(worktree), "session.jsonl")
	return writeFile(transcript, "{\"type\":\"assistant\"}\n")
}

// dashedPath is how Claude Code names one directory's transcript record: the
// path with its separators turned into dashes.
func dashedPath(path string) string {
	return strings.ReplaceAll(strings.ReplaceAll(path, "/", "-"), "_", "-")
}
