package steps

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// forgeRootOf is the fixture forge root one forge name stands for.
func (w *World) forgeRootOf(name string) (string, error) {
	store, err := w.declaredForge(name)
	if err != nil {
		return "", err
	}
	return store.Root(), nil
}

// rootsOf turns a Gherkin list of forge names into the roots they stand for.
func (w *World) rootsOf(text string) ([]string, error) {
	var roots []string
	for _, name := range forgeNames(text) {
		root, err := w.forgeRootOf(name)
		if err != nil {
			return nil, err
		}
		roots = append(roots, root)
	}
	if len(roots) == 0 {
		return nil, fmt.Errorf("the step names no forge root")
	}
	return roots, nil
}

// forgeRootsHoldProject gives every named fixture forge a project of its own,
// the way one bridge carries several forges that each serve the same project.
func forgeRootsHoldProject(ctx context.Context, world any, captures []string) error {
	w := world.(*World)
	for _, name := range forgeNames(captures[1]) {
		store, err := w.forge(ctx, name)
		if err != nil {
			return err
		}
		if err := setUpProject(store.Root(), captures[2]); err != nil {
			return err
		}
	}
	return nil
}

// projectRecordsRole writes a role's row in a project's own roles file, the way
// a forge that runs one agent per role records which agent that is.
func projectRecordsRole(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	project, forge, role, tool := captures[1], captures[2], captures[3], captures[4]
	root, err := w.forgeRootOf(forge)
	if err != nil {
		return err
	}
	dir := filepath.Join(root, "projects", project)
	path := filepath.Join(dir, ".swarmforge", "roles.tsv")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("the project %s has no roles file: %w", project, err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	replaced := false
	for index, line := range lines {
		columns := strings.Split(line, "\t")
		if len(columns) == 0 || columns[0] != role {
			continue
		}
		for len(columns) < 8 {
			columns = append(columns, "")
		}
		columns[5] = tool
		lines[index] = strings.Join(columns, "\t")
		replaced = true
	}
	if !replaced {
		lines = append(lines, strings.Join([]string{
			role, role, filepath.Join(dir, "worktrees", role), "fixture-" + role,
			strings.ToUpper(role[:1]) + role[1:], tool, "task", "forward-only",
		}, "\t"))
	}
	return writeFile(path, strings.Join(lines, "\n")+"\n")
}

// forgeGivesLiveSession starts the role sessions a forge is running, the way
// the machine does: one tmux session per role, up and quiet. A session judged
// alive by its pane rather than by the agent's name is what the check reads.
func forgeGivesLiveSession(_ context.Context, world any, captures []string) error {
	return forgeGivesSession(world.(*World), captures[1], captures[2], paneCommand)
}

// forgeGivesSession starts the forge's role sessions with the named role
// running the command the scenario asked for.
func forgeGivesSession(w *World, forge, role, command string) error {
	root, err := w.forgeRootOf(forge)
	if err != nil {
		return err
	}
	projects, err := projectsOf(root)
	if err != nil {
		return err
	}
	if len(projects) != 1 {
		return fmt.Errorf("the forge root %s holds %d projects, want exactly one here", forge, len(projects))
	}
	return w.serveSessions(projects[0], role, command)
}

// serveRoleSessions starts one quiet tmux session per role of a project, named
// after the role's pane, so the role the scenario names is a live session and
// the ones it does not name are not read as dead agents.
func (w *World) serveRoleSessions(projectDir, liveRole string) error {
	return w.serveSessions(projectDir, liveRole, paneCommand)
}

// paneCommand is what a fixture pane runs: something that is not a shell - a
// shell is what a session that ended looks like - that prints nothing on its
// own and echoes what is typed into it, which is how a pane shows a request the
// dashboard, or the doorbell, typed.
const paneCommand = "cat"

// busyPaneCommand is what a role mid-turn looks like to the check that judges
// it: codex says so in its own pane line, whatever the terminal draws.
const busyPaneCommand = `zsh -c 'echo "esc to interrupt"; exec cat'`

// serveSessions starts one tmux session per role of a project, with the role
// the scenario names running the command it was given and the others running a
// quiet pane.
func (w *World) serveSessions(projectDir, liveRole, liveCommand string) error {
	panes, err := rolePanes(projectDir)
	if err != nil {
		return err
	}
	livePane, err := paneOf(projectDir, liveRole)
	if err != nil {
		return err
	}
	socket, err := w.projectSocket(projectDir)
	if err != nil {
		return err
	}
	for _, pane := range panes {
		command := paneCommand
		if pane == livePane {
			command = liveCommand
			// The role's session is what the scenario is describing now, whether
			// an earlier step left it working or quiet: a session already there
			// is replaced, so a role that was mid-turn can be free.
			_, _ = tmux(socket, "kill-session", "-t", pane)
		}
		if err := startPane(socket, pane, command); err != nil {
			return err
		}
	}
	return nil
}

// projectsOf lists the projects one fixture forge root serves.
func projectsOf(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, "projects"))
	if err != nil {
		return nil, fmt.Errorf("the forge root %s serves no projects: %w", root, err)
	}
	var projects []string
	for _, entry := range entries {
		if entry.IsDir() {
			projects = append(projects, filepath.Join(root, "projects", entry.Name()))
		}
	}
	return projects, nil
}

// projectSocket is the tmux socket a project's roles are read on, recorded the
// way the forge records it. The socket itself lives in the system's short-path
// scratch: a unix socket path holds about a hundred bytes, and the worktree a
// scenario runs in is longer than that before anything is added to it.
//
// The forge root records the same socket: one forge has one set of panes, and
// both the check, which reads a project's, and a tool that reads the forge
// root's — the doorbell rings the pane the dashboard would — have to find them.
func (w *World) projectSocket(projectDir string) (string, error) {
	stateDir := filepath.Join(projectDir, ".swarmforge")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return "", err
	}
	record := filepath.Join(stateDir, "tmux-socket")
	if data, err := os.ReadFile(record); err == nil {
		if socket := strings.TrimSpace(string(data)); socket != "" {
			if err := w.recordForgeSocket(projectDir, socket); err != nil {
				return "", err
			}
			return socket, nil
		}
	}
	dir, err := os.MkdirTemp("", "swarmforge-tmux-")
	if err != nil {
		return "", err
	}
	socket := filepath.Join(dir, "sock")
	if err := os.WriteFile(record, []byte(socket+"\n"), 0o644); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	if err := w.recordForgeSocket(projectDir, socket); err != nil {
		return "", err
	}
	w.sockets = appendUnique(w.sockets, socket)
	w.socketDirs = append(w.socketDirs, dir)
	return socket, nil
}

// recordForgeSocket writes the same socket into the forge root the project
// belongs to, so a tool that reads the forge root finds the panes the project's
// roles live in.
func (w *World) recordForgeSocket(projectDir, socket string) error {
	root := filepath.Dir(filepath.Dir(projectDir))
	stateDir := filepath.Join(root, ".swarmforge")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(stateDir, "tmux-socket"), []byte(socket+"\n"), 0o644)
}

// rolePanes reads the pane each role of a project is served in.
func rolePanes(projectDir string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(projectDir, ".swarmforge", "roles.tsv"))
	if err != nil {
		return nil, err
	}
	var panes []string
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		columns := strings.Split(line, "\t")
		if len(columns) >= 4 && strings.TrimSpace(columns[3]) != "" {
			panes = append(panes, columns[3])
		}
	}
	return panes, nil
}

// roleColumn is one column of a project's role row, the row whose first column
// is the role. It is how the fixture reads a role's worktree or its pane.
func roleColumn(projectDir, role string, index int) (string, error) {
	data, err := os.ReadFile(filepath.Join(projectDir, ".swarmforge", "roles.tsv"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		columns := strings.Split(line, "\t")
		if len(columns) > index && columns[0] == role {
			return columns[index], nil
		}
	}
	return "", fmt.Errorf("the project %s records no role %s", projectDir, role)
}

// paneOf is the pane one role of a project is served in.
func paneOf(projectDir, role string) (string, error) {
	return roleColumn(projectDir, role, 3)
}

// startPane brings up one session on a socket running the command the fixture
// gave it.
func startPane(socket, pane, command string) error {
	if _, err := tmux(socket, "has-session", "-t", pane); err == nil {
		return nil
	}
	if _, err := tmux(socket, "new-session", "-d", "-s", pane, command); err != nil {
		return fmt.Errorf("the fixture could not start the pane %s: %w", pane, err)
	}
	return nil
}

// paneText is what a pane currently shows.
func paneText(socket, pane string) (string, error) {
	return tmux(socket, "capture-pane", "-p", "-t", pane)
}

// tmux runs one command against a project's tmux server.
func tmux(socket string, args ...string) (string, error) {
	ctx, cancel := stepContext()
	defer cancel()
	command := exec.CommandContext(ctx, "tmux", append([]string{"-S", socket}, args...)...)
	out, err := command.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("tmux %s: %w: %s", strings.Join(args, " "), err, out)
	}
	return string(out), nil
}

// killTmuxServer takes a fixture's tmux server down with the rest of it.
func killTmuxServer(socket string) {
	command := exec.Command("tmux", "-S", socket, "kill-server")
	_ = command.Run()
}
