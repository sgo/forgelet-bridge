package steps

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
)

// laneState is what the worktree the suite runs from held when the scenario
// started: its head, its branches, and its changes.
type laneState struct {
	head     string
	branches string
	changes  string
}

// rememberLane records the worktree the suite runs from, so the scenario can
// show afterwards that nothing in it moved.
func rememberLane(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	state, err := readLaneState()
	if err != nil {
		return err
	}
	w.lane = state
	return nil
}

// laneHeld checks the worktree the suite runs from still holds its head, its
// branches and its changes.
func laneHeld(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	if w.lane == nil {
		return fmt.Errorf("the suite did not remember the worktree it runs from")
	}
	now, err := readLaneState()
	if err != nil {
		return err
	}
	if now.head != w.lane.head {
		return fmt.Errorf("the worktree the suite runs from moved its head: %s -> %s", w.lane.head, now.head)
	}
	if now.branches != w.lane.branches {
		return fmt.Errorf("the worktree the suite runs from gained or lost branches:\n%s\n->\n%s", w.lane.branches, now.branches)
	}
	if now.changes != w.lane.changes {
		return fmt.Errorf("the worktree the suite runs from changed its changes:\n%s\n->\n%s", w.lane.changes, now.changes)
	}
	return nil
}

// readLaneState reads the git state of the worktree the suite runs from.
func readLaneState() (*laneState, error) {
	root := fixtures.ProjectRoot()
	head, err := git(root, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	branches, err := git(root, "for-each-ref", "--format=%(refname) %(objectname)", "refs/heads", "refs/tags")
	if err != nil {
		return nil, err
	}
	changes, err := git(root, "status", "--porcelain")
	if err != nil {
		return nil, err
	}
	return &laneState{head: head, branches: branches, changes: changes}, nil
}

func git(root string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

// fixtureHead is the commit the fixture's own repository is on, which is the
// commit a fixture handoff can name: the fixture's git world ends at the
// fixture, so a hash from anywhere else means nothing to it.
func fixtureHead(root string) string {
	head, err := git(root, "rev-parse", "HEAD")
	if err != nil {
		return ""
	}
	return head
}

// fixtureHoldsSnapshot checks the dashboard's snapshot of a card landed in the
// fixture's own repository, not in the worktree running the suite.
func fixtureHoldsSnapshot(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	card := captures[1]
	root, err := singleForge(w)
	if err != nil {
		return err
	}
	dashboard, ok := w.running[filepath.Base(root)]
	if !ok {
		return fmt.Errorf("the fixture forge root %s does not have its dashboard running", root)
	}
	ctx, cancel := stepContext()
	defer cancel()
	return waitFor(ctx, fmt.Sprintf("the fixture never held the snapshot of %s", card), func() (bool, error) {
		branches, err := dashboard.FixtureSnapshots(card)
		if err != nil {
			return false, err
		}
		return len(branches) > 0, nil
	})
}
