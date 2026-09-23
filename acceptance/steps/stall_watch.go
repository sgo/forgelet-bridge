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

// stallWatchCommand is this repository's stall watch: the tool a forge's own
// copy is a deployment of, run the way its launch agent runs it.
const stallWatchCommand = "stall_watch.sh"

// watchScript is one of the watch's commands, run from this repository's own
// copy against a fixture forge root.
func watchScript(ctx context.Context, command string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx,
		filepath.Join(fixtures.ProjectRoot(), "swarmforge", "scripts", stallWatchCommand),
		append([]string{command}, args...)...)
}

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

// paneOf is the pane one role of a project is served in.
func paneOf(projectDir, role string) (string, error) {
	data, err := os.ReadFile(filepath.Join(projectDir, ".swarmforge", "roles.tsv"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		columns := strings.Split(line, "\t")
		if len(columns) >= 4 && columns[0] == role {
			return columns[3], nil
		}
	}
	return "", fmt.Errorf("the project %s records no role %s", projectDir, role)
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

// stallWatchRuns runs one pass of the watch for the named forge roots, the way
// the machine's launch agent runs it: one pass over every root it was installed
// for.
func stallWatchRuns(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	roots, err := w.rootsOf(captures[1])
	if err != nil {
		return err
	}
	return w.runWatch(roots)
}

// runWatch runs the watch once over every root and keeps what it said.
func (w *World) runWatch(roots []string) error {
	ctx, cancel := stepContext()
	defer cancel()
	command := watchScript(ctx, "run", roots...)
	command.Dir = roots[0]
	// Claude Code's own transcript is what says a claude session is working;
	// the fixture keeps that record inside itself so no developer's machine
	// decides what the scenario sees.
	command.Env = append(os.Environ(), "ROLE_HEALTH_CLAUDE_PROJECTS="+filepath.Join(w.workDir, "claude-projects"))
	out, err := command.CombinedOutput()
	w.watchOutput = string(out)
	w.watchRoots = roots
	if err != nil {
		return fmt.Errorf("the stall watch did not finish its pass: %v\n%s", err, w.watchOutput)
	}
	return nil
}

// watchAgentInstalled writes down the agent an install would leave behind, so
// what the machine would run is a thing to read rather than a claim. Loading a
// launch agent is the machine's business, not the suite's.
func watchAgentInstalled(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	roots, err := w.rootsOf(captures[1])
	if err != nil {
		return err
	}
	ctx, cancel := stepContext()
	defer cancel()
	command := watchScript(ctx, "print-agent", roots...)
	command.Dir = roots[0]
	out, err := command.CombinedOutput()
	w.watchAgent = string(out)
	if err != nil {
		return fmt.Errorf("the watch could not print its agent: %v\n%s", err, w.watchAgent)
	}
	return nil
}

// watchAgentRunsFor checks the agent the machine would run starts the watch,
// once, for every forge root it was installed for.
func watchAgentRunsFor(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	roots, err := w.rootsOf(captures[1])
	if err != nil {
		return err
	}
	if w.watchAgent == "" {
		return fmt.Errorf("no agent was installed to read")
	}
	if !strings.Contains(w.watchAgent, "<string>run</string>") {
		return fmt.Errorf("the agent does not run the watch:\n%s", w.watchAgent)
	}
	for _, root := range roots {
		if !strings.Contains(w.watchAgent, "<string>"+root+"</string>") {
			return fmt.Errorf("the agent does not name the forge root %s:\n%s", root, w.watchAgent)
		}
	}
	return nil
}

// forgeHoldsRequestNaming waits for the forge to hold a chat request that names
// both the role that stalled and the card it was holding.
func forgeHoldsRequestNaming(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	return waitFor(ctx, fmt.Sprintf("the forge never held a chat request naming the role %s and the card %s", captures[1], captures[2]), func() (bool, error) {
		count, err := countRequestsNaming(w, w.configured, []string{captures[1], captures[2]})
		return count >= 1, err
	})
}

// operatorDecryptsChatNaming waits for the operator to read an encrypted chat
// message naming the stalled role and the card it was holding, which is how a
// stall reaches the phone.
func operatorDecryptsChatNaming(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	roomID, err := w.chatRoom(ctx, "Chat")
	if err != nil {
		return err
	}
	var found fixtures.Message
	err = waitFor(ctx, fmt.Sprintf("the operator never decrypted a chat message naming the role %s and the card %s", captures[1], captures[2]), func() (bool, error) {
		for _, message := range operator.Messages(roomID) {
			if strings.Contains(message.Body, captures[1]) && strings.Contains(message.Body, captures[2]) {
				found = message
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		return err
	}
	if !found.Encrypted {
		return fmt.Errorf("the chat message naming %s and %s was not decrypted from an encrypted event", captures[1], captures[2])
	}
	w.anchors[found.Body] = found.EventID
	return nil
}

// namedForgeHoldsRequestNamingRole waits for one named forge to hold a chat
// request naming the stalled role and the forge the alert came from.
func namedForgeHoldsRequestNamingRole(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := w.forgeRootOf(captures[1])
	if err != nil {
		return err
	}
	ctx, cancel := stepContext()
	defer cancel()
	return waitFor(ctx, fmt.Sprintf("the forge %s never held a chat request naming the role %s and the forge it came from", captures[1], captures[2]), func() (bool, error) {
		count, err := countRequestsNaming(w, []string{root}, []string{captures[2], filepath.Base(root)})
		return count >= 1, err
	})
}

// countRequestsNaming counts the chat requests the named forge roots hold whose
// body carries every one of the named words.
func countRequestsNaming(w *World, roots, words []string) (int, error) {
	count := 0
	for _, root := range roots {
		store := w.dashboards[filepath.Base(root)]
		if store == nil {
			continue
		}
		requests, err := store.Requests()
		if err != nil {
			return 0, err
		}
		for _, request := range requests {
			names := true
			for _, word := range words {
				if !strings.Contains(request.Body, word) {
					names = false
					break
				}
			}
			if names {
				count++
			}
		}
	}
	return count, nil
}

// anotherPassFilesNothingNew runs a second pass and checks the watch asked
// about the same card once, not once per pass: a stall alarm that repeats is a
// loop the operator learns to ignore.
func anotherPassFilesNothingNew(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	card := captures[1]
	roots := w.watchRoots
	if len(roots) == 0 {
		return fmt.Errorf("the watch has not run yet")
	}
	before, err := countRequestsNaming(w, roots, []string{card})
	if err != nil {
		return err
	}
	if before == 0 {
		return fmt.Errorf("the first pass filed no chat request about %s", card)
	}
	if err := w.runWatch(roots); err != nil {
		return err
	}
	ctx, cancel := stepContext()
	defer cancel()
	if err := fixtures.Sleep(ctx, settle); err != nil {
		return err
	}
	after, err := countRequestsNaming(w, roots, []string{card})
	if err != nil {
		return err
	}
	if after != before {
		return fmt.Errorf("the second pass filed %d chat requests about %s, want the %d it already held", after-before, card, before)
	}
	return nil
}

// namedForgeBoardHoldsCard puts a card on one named forge's board, in the lane
// it is in, the way the forge's own board does.
func namedForgeBoardHoldsCard(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := w.forgeRootOf(captures[1])
	if err != nil {
		return err
	}
	if err := markProjectOpen(root); err != nil {
		return err
	}
	return setCardLane(root, captures[3], captures[2], captures[4])
}

// watchReportsItIsAlive reads the watch's own status, which is what says a
// silent watcher is alive rather than assumed.
func watchReportsItIsAlive(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	out, err := w.watchStatus()
	if err != nil {
		return err
	}
	if !strings.Contains(out, "the watch is alive") {
		return fmt.Errorf("the watch does not report itself alive:\n%s", out)
	}
	return nil
}

// watchHeartbeatGoesStale ages the heartbeat the watch left, the way a watch
// that stopped leaves one: the machine's own record, gone quiet.
func watchHeartbeatGoesStale(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	root, err := w.watchRoot()
	if err != nil {
		return err
	}
	heartbeat := filepath.Join(root, ".swarmforge", "stall-watch.heartbeat")
	stale := time.Now().Add(-10 * time.Minute)
	if err := os.Chtimes(heartbeat, stale, stale); err != nil {
		return fmt.Errorf("the watch left no heartbeat to age: %w", err)
	}
	return nil
}

// watchReportsItHasGoneQuiet checks a stale heartbeat is reported rather than
// passing quietly.
func watchReportsItHasGoneQuiet(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	out, err := w.watchStatus()
	if err != nil {
		return err
	}
	if !strings.Contains(out, "the watch has gone quiet") {
		return fmt.Errorf("the watch reports itself alive on a stale heartbeat:\n%s", out)
	}
	return nil
}

// watchStatus asks the watch what it is doing. A stale heartbeat is its own
// alarm, so the step reads what it says whatever it returns.
func (w *World) watchStatus() (string, error) {
	root, err := w.watchRoot()
	if err != nil {
		return "", err
	}
	ctx, cancel := stepContext()
	defer cancel()
	command := watchScript(ctx, "status", root)
	command.Dir = root
	out, _ := command.CombinedOutput()
	return string(out), nil
}

// watchRoot is the forge root the watch was installed from, where its heartbeat
// and its log live.
func (w *World) watchRoot() (string, error) {
	if len(w.watchRoots) == 0 {
		return "", fmt.Errorf("the watch has not run yet")
	}
	return w.watchRoots[0], nil
}

// theWatchLeftThePaneAlone checks the watch asked the operator rather than the
// role: nothing was typed into the stalled role's pane.
func theWatchLeftThePaneAlone(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := w.watchRoot()
	if err != nil {
		return err
	}
	projects, err := projectsOf(root)
	if err != nil {
		return err
	}
	if len(projects) != 1 {
		return fmt.Errorf("the forge root %s holds %d projects, want exactly one here", root, len(projects))
	}
	pane, err := paneOf(projects[0], captures[1])
	if err != nil {
		return err
	}
	socket, err := w.projectSocket(projects[0])
	if err != nil {
		return err
	}
	text, err := paneText(socket, pane)
	if err != nil {
		return err
	}
	if strings.Contains(text, askMarker) {
		return fmt.Errorf("the watch typed %q into the %s pane:\n%s", askMarker, captures[1], text)
	}
	return nil
}

// askMarker is the wording the idler check puts in a pane when it is asked to
// question a role, which the watch must never do.
const askMarker = "What blocks you?"
