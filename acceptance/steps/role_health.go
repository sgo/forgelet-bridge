package steps

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
)

// idlerCheckCommand is this repository's idler check: the tool a forge's own
// copy is a deployment of.
const idlerCheckCommand = "role_health.sh"

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
	data, err := os.ReadFile(filepath.Join(projectDir, ".swarmforge", "roles.tsv"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		columns := strings.Split(line, "\t")
		if len(columns) >= 3 && columns[0] == role {
			return columns[2], nil
		}
	}
	return "", fmt.Errorf("the project %s records no role %s", projectDir, role)
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

// idlerCheckRuns runs the check against one project, the way the forge runs it.
func idlerCheckRuns(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	projectDir, err := w.pathOf(captures[2], captures[1])
	if err != nil {
		return err
	}
	ctx, cancel := stepContext()
	defer cancel()
	command := exec.CommandContext(ctx,
		filepath.Join(fixtures.ProjectRoot(), "swarmforge", "scripts", idlerCheckCommand), projectDir)
	command.Dir = projectDir
	command.Env = append(os.Environ(), "ROLE_HEALTH_CLAUDE_PROJECTS="+w.claudeProjects())
	out, err := command.CombinedOutput()
	w.idlerOutput = string(out)
	w.idlerErr = err
	return nil
}

// idlerCheckReports checks what the check said about one role.
func idlerCheckReports(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	role, phrase := captures[1], captures[2]
	verdict, card, err := idlerVerdict(phrase)
	if err != nil {
		return err
	}
	line, err := idlerLine(w, role)
	if err != nil {
		return err
	}
	columns := strings.Fields(line)
	if len(columns) < 2 || columns[1] != verdict {
		return fmt.Errorf("the idler check reports the role %s as %q, want %s:\n%s", role, columns[1], verdict, w.idlerOutput)
	}
	if card != "" && !strings.Contains(line, card) {
		return fmt.Errorf("the idler check's line for %s does not name the card %s:\n%s", role, card, line)
	}
	return nil
}

// idlerCheckNamesTool checks the check judged the role against the tool its own
// row records.
func idlerCheckNamesTool(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	tool, role := captures[1], captures[2]
	line, err := idlerLine(w, role)
	if err != nil {
		return err
	}
	if !strings.Contains(line, "tool="+tool) {
		return fmt.Errorf("the idler check does not name the tool %s for the role %s:\n%s", tool, role, line)
	}
	return nil
}

// idlerCheckStatus checks the check's exit status: it fails when any role is
// stalled and passes when none is.
func idlerCheckStatus(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	if w.idlerOutput == "" {
		return fmt.Errorf("the idler check has not run")
	}
	switch captures[1] {
	case "fails":
		if w.idlerErr == nil {
			return fmt.Errorf("the idler check reported no stall:\n%s", w.idlerOutput)
		}
	case "passes":
		if w.idlerErr != nil {
			return fmt.Errorf("the idler check failed with no stall to report:\n%s", w.idlerOutput)
		}
	default:
		return fmt.Errorf("the step does not know the wording %q", captures[1])
	}
	return nil
}

// idlerCheckReportsForgeNotRunning checks the check says a forge with no role
// sessions is not running, rather than filing every role as a dead agent.
func idlerCheckReportsForgeNotRunning(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	line, err := idlerLine(w, "forge")
	if err != nil {
		return err
	}
	if columns := strings.Fields(line); len(columns) < 2 || columns[1] != "not-running" {
		return fmt.Errorf("the idler check does not report the forge as not running:\n%s", w.idlerOutput)
	}
	return nil
}

// idlerLine is the check's report line for one role.
func idlerLine(w *World, role string) (string, error) {
	for _, line := range strings.Split(strings.TrimRight(w.idlerOutput, "\n"), "\n") {
		if columns := strings.Fields(line); len(columns) > 0 && columns[0] == role {
			return line, nil
		}
	}
	return "", fmt.Errorf("the idler check reports nothing for the role %s:\n%s", role, w.idlerOutput)
}

// idlerVerdict turns the wording a scenario uses into the verdict the check
// prints, and the card that wording names when it names one.
func idlerVerdict(phrase string) (string, string, error) {
	switch {
	case strings.HasPrefix(phrase, "idle holding the card "):
		return "idle-holding-card", strings.TrimPrefix(phrase, "idle holding the card "), nil
	case strings.HasPrefix(phrase, "assigned the card ") && strings.HasSuffix(phrase, " but not yet handed over"):
		card := strings.TrimSuffix(strings.TrimPrefix(phrase, "assigned the card "), " but not yet handed over")
		return "assigned-not-taken", card, nil
	case phrase == "idle with nothing assigned":
		return "idle-nothing-assigned", "", nil
	case phrase == "working":
		return "working", "", nil
	case phrase == "running a tool it does not know":
		return "tool-not-known", "", nil
	case phrase == "waiting on a decision":
		return "waiting-on-a-decision", "", nil
	}
	return "", "", fmt.Errorf("the step does not know the wording %q", phrase)
}
