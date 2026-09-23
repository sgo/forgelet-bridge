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
	return w.serveRoleSessions(projects[0], captures[2])
}

// serveRoleSessions starts one quiet tmux session per role of a project, named
// after the role's pane, so the role the scenario names is a live session and
// the ones it does not name are not read as dead agents.
func (w *World) serveRoleSessions(projectDir, liveRole string) error {
	panes, err := rolePanes(projectDir)
	if err != nil {
		return err
	}
	if _, err := paneOf(projectDir, liveRole); err != nil {
		return err
	}
	socket, err := w.projectSocket(projectDir)
	if err != nil {
		return err
	}
	for _, pane := range panes {
		if err := startPane(socket, pane); err != nil {
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
func (w *World) projectSocket(projectDir string) (string, error) {
	stateDir := filepath.Join(projectDir, ".swarmforge")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return "", err
	}
	record := filepath.Join(stateDir, "tmux-socket")
	if data, err := os.ReadFile(record); err == nil {
		if socket := strings.TrimSpace(string(data)); socket != "" {
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
	w.sockets = appendUnique(w.sockets, socket)
	w.socketDirs = append(w.socketDirs, dir)
	return socket, nil
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

// startPane brings up one quiet session on a socket: a session that is up,
// running something other than a shell, and printing nothing, which is what a
// role between turns looks like.
func startPane(socket, pane string) error {
	if _, err := tmux(socket, "has-session", "-t", pane); err == nil {
		return nil
	}
	if _, err := tmux(socket, "new-session", "-d", "-s", pane, "sleep 600"); err != nil {
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
