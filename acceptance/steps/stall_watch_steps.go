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

// askMarker is the wording the idler check puts in a pane when it is asked to
// question a role, which the watch must never do.
const askMarker = "What blocks you?"

// watchScript is one of the watch's commands, run from this repository's own
// copy against a fixture forge root.
func watchScript(ctx context.Context, command string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx,
		filepath.Join(fixtures.ProjectRoot(), "swarmforge", "scripts", stallWatchCommand),
		append([]string{command}, args...)...)
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
	command.Env = append(os.Environ(), "ROLE_HEALTH_CLAUDE_PROJECTS="+w.claudeProjects())
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
	return watchStatusSaying(world.(*World), "the watch is alive", "the watch does not report itself alive")
}

// watchReportsItHasGoneQuiet checks a stale heartbeat is reported rather than
// passing quietly.
func watchReportsItHasGoneQuiet(_ context.Context, world any, _ []string) error {
	return watchStatusSaying(world.(*World), "the watch has gone quiet", "the watch reports itself alive on a stale heartbeat")
}

// watchStatusSaying checks the watch's own status says what it should, and
// complains with the given wording when it does not.
func watchStatusSaying(w *World, phrase, complaint string) error {
	out, err := w.watchStatus()
	if err != nil {
		return err
	}
	if !strings.Contains(out, phrase) {
		return fmt.Errorf("%s:\n%s", complaint, out)
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
