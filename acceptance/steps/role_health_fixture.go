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

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-10-06T08:58:57+02:00","module_hash":"b9234ffb49328259839014f3c2b61b1a7ea3a9a6b43f9afacf1578fdee62f3f5","functions":[{"id":"func/World.claudeProjects","name":"World.claudeProjects","line":28,"end_line":30,"hash":"c805306ccf6450d3b1927d3061e97fae6e50a89be8c1e7442fe180dda0523d30"},{"id":"func/World.pathOf","name":"World.pathOf","line":33,"end_line":39,"hash":"54d119539bbee66465a8473abe62a224de1e03b2ba6a30e1935de0370b852c9e"},{"id":"func/roleWorktree","name":"roleWorktree","line":42,"end_line":44,"hash":"34ad2e408bb98413be2567ab91bc6dfc0a66ca1b2f72bd1988810165ab927e10"},{"id":"func/projectHandedCardToRole","name":"projectHandedCardToRole","line":48,"end_line":64,"hash":"1ee32ed4e082ff370e5cd19bbb43e66f839a390d4d5c173712cb2375f5250486"},{"id":"func/projectNoteStillInASendersOutbox","name":"projectNoteStillInASendersOutbox","line":68,"end_line":76,"hash":"db2e14ec7461b1d282dfe9616c9c3fe722f4fbc056661bd2e09091fd048351bc"},{"id":"func/projectHandoffWaitingForTheOperator","name":"projectHandoffWaitingForTheOperator","line":83,"end_line":101,"hash":"43490a838c320e2acca032a05f90c3206bbcbf2ba74c1cbb575aef982a991caf"},{"id":"func/rolesOf","name":"rolesOf","line":105,"end_line":117,"hash":"1c6ff3d24964385ae579d544ba4f041ba73da02bdd57086dfdcee2741b2d0bfd"},{"id":"func/quietWorktree","name":"quietWorktree","line":122,"end_line":143,"hash":"b0afc25f4324e7d1e886292a71c0c53f1ae29722c14f39d8db186b2987b0baa7"},{"id":"func/projectHasOpenClarification","name":"projectHasOpenClarification","line":147,"end_line":155,"hash":"07cfabd88a2dabb9194f66da71875c7af49ad6896a709268209a900bcf42295e"},{"id":"func/forgeGivesWorkingSession","name":"forgeGivesWorkingSession","line":160,"end_line":183,"hash":"b7f7a1d53a6cd8a2ad35f5d868b63f6e19a5d6a2b4e49ce5e721ad372dcdd5e3"},{"id":"func/forgeGivesLiveSessionAtAShell","name":"forgeGivesLiveSessionAtAShell","line":188,"end_line":190,"hash":"307ba5daea2c88c8bbbc483b98eaeba086b3ad20a19cd6995929304537e64cfe"},{"id":"func/forgeGivesASessionRunningATool","name":"forgeGivesASessionRunningATool","line":195,"end_line":197,"hash":"e71b70e79c3226384e46f28ba5d22b745bb4a337fb623b2e9780d418b96d4b46"},{"id":"func/theForgeEndsTheRolesSession","name":"theForgeEndsTheRolesSession","line":202,"end_line":227,"hash":"291de1606be80188593a0db94bd315686cea71db98f37de5e0ca37fa386cf041"},{"id":"func/dashedPath","name":"dashedPath","line":231,"end_line":233,"hash":"0052441bb46c9b880748bbe35ce82599f3671ad3e10010072157db426ff8d044"}]}
// mutate4go-manifest-end
