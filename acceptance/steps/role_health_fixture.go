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

// shellPaneCommand is what a pane whose agent has gone looks like: the shell the
// session was started with, left behind when the agent typed into it exited. The
// pane is still there, so it is a live session - not the session that is gone -
// whose agent has stopped.
const shellPaneCommand = "zsh"

// toolPaneCommand is a tool the agent is inside: the foreground for as long as
// it runs, like the java or mvn of a build. A check that read "the pane is not
// the agent" as gone would ring a false alarm here, so the tool's own foreground
// is a pane that is left alone.
const toolPaneCommand = "exec sleep 3600"

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

// projectNoteStillInASendersOutbox records the card's note where a sender left
// it: handed over to nobody yet, which is work waiting to be taken up.
func projectNoteStillInASendersOutbox(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	projectDir, err := w.pathOf(captures[2], captures[1])
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(projectDir, ".swarmforge", "handoffs", "outbox", "50_"+captures[3]+".handoff"),
		"task: "+captures[3]+"\n")
}

// projectHandoffWaitingForTheOperator records the handoff for a card the
// operator has yet to decide, in the place the forge parks one. The check reads
// the role that raised it out of the file, so every role the project serves
// gets its own: the scenario says the handoff is waiting, not which role raised
// it, and a role waiting on the operator is not stalled.
func projectHandoffWaitingForTheOperator(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	projectDir, err := w.pathOf(captures[2], captures[1])
	if err != nil {
		return err
	}
	card := captures[3]
	roles, err := rolesOf(projectDir)
	if err != nil {
		return err
	}
	for _, role := range roles {
		if err := writeFile(filepath.Join(projectDir, ".swarmforge", "handoffs", "pending_approval", "50_"+role+"-"+card+".handoff"),
			"from: "+role+"\ntask: "+card+"\n"); err != nil {
			return err
		}
	}
	return nil
}

// rolesOf lists the roles one project records, in the order its roles file
// holds them.
func rolesOf(projectDir string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(projectDir, ".swarmforge", "roles.tsv"))
	if err != nil {
		return nil, err
	}
	var roles []string
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if columns := strings.Split(line, "\t"); len(columns) > 0 && columns[0] != "" {
			roles = append(roles, columns[0])
		}
	}
	return roles, nil
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

// forgeGivesLiveSessionAtAShell gives the named role a live session whose pane
// is at a shell: the session outlived its agent, which is the state the check
// has to tell from a role that is simply idle.
func forgeGivesLiveSessionAtAShell(_ context.Context, world any, captures []string) error {
	return forgeGivesSession(world.(*World), captures[1], captures[2], shellPaneCommand)
}

// forgeGivesASessionRunningATool gives the named role a live session whose pane
// runs a tool the agent is inside, which is the agent at work rather than an
// agent that has gone.
func forgeGivesASessionRunningATool(_ context.Context, world any, captures []string) error {
	return forgeGivesSession(world.(*World), captures[1], captures[2], toolPaneCommand)
}

// theForgeEndsTheRolesSession takes the named role's session down, leaving the
// session that is genuinely gone: the pane is not there at all, which is the
// other reading and not the agent that has gone.
func theForgeEndsTheRolesSession(_ context.Context, world any, captures []string) error {
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
	pane, err := paneOf(projects[0], captures[2])
	if err != nil {
		return err
	}
	socket, err := w.projectSocket(projects[0])
	if err != nil {
		return err
	}
	// A session that is already down is the reading this step wants, so a kill
	// that finds nothing to kill is not a failure.
	_, _ = tmux(socket, "kill-session", "-t", pane)
	return nil
}

// dashedPath is how Claude Code names one directory's transcript record: the
// path with its separators turned into dashes.
func dashedPath(path string) string {
	return strings.ReplaceAll(strings.ReplaceAll(path, "/", "-"), "_", "-")
}
