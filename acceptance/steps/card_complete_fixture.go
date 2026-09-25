package steps

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
)

// cardCompleteHook is the finishing step the project ships, relative to the
// project root: the tooling runs <project>/swarmforge/hooks/card-complete.sh
// once a card's board row says done, so that is where a checkout of the bridge
// has to carry it.
const cardCompleteHook = "swarmforge/hooks/card-complete.sh"

// gitIdentity is the fixture's own commit identity: the fixture commits as
// itself rather than borrowing whatever identity the machine has.
var gitIdentity = []string{"-c", "user.email=fixture@example.org", "-c", "user.name=Fixture"}

// cardCompleteFixture is a throwaway project standing in for a checkout of the
// bridge: a repository of its own, an origin of the scenario's choosing, and
// the bridge's own finishing step in place where the tooling looks for it. The
// card's work lands on its own branch, as it does in the worktree that holds
// master, and a push is the only thing that reaches the origin.
type cardCompleteFixture struct {
	root   string
	branch string
	// commit is the commit the card's work landed on, which is what a push has
	// to carry to the origin.
	commit string
	hook   string
	// origin is the bare repository the scenario gave the fixture, empty when
	// the scenario asked for no remote at all.
	origin string
	// output and err are what the finishing step last said and returned.
	output string
	err    error
}

// cardCompleteProject is the throwaway checkout the card-complete scenarios work
// with, laid out under the scenario's own directory the first time a scenario
// asks for it.
func (w *World) cardCompleteProject() (*cardCompleteFixture, error) {
	if w.cardComplete != nil {
		return w.cardComplete, nil
	}
	fixture := &cardCompleteFixture{
		root:   filepath.Join(w.workDir, "card-complete", "fixture"),
		branch: "master",
	}
	if err := fixture.becomeACheckoutOfTheBridge(); err != nil {
		return nil, err
	}
	w.cardComplete = fixture
	return fixture, nil
}

// becomeACheckoutOfTheBridge lays the fixture out as a checkout of the bridge:
// its own repository, the bridge's finishing step in place where the tooling
// looks for it, and one commit holding it, so a push has something of the
// bridge's to carry.
func (f *cardCompleteFixture) becomeACheckoutOfTheBridge() error {
	if err := f.shipTheHook(); err != nil {
		return err
	}
	if _, err := git(f.root, "init", "--quiet", "--initial-branch="+f.branch); err != nil {
		return err
	}
	return commitAll(f.root, "a checkout of the bridge")
}

// shipTheHook copies the finishing step this project ships into the fixture,
// where the tooling runs it from.
func (f *cardCompleteFixture) shipTheHook() error {
	f.hook = filepath.Join(f.root, filepath.FromSlash(cardCompleteHook))
	if err := os.MkdirAll(filepath.Dir(f.hook), 0o755); err != nil {
		return err
	}
	shipped, err := os.ReadFile(filepath.Join(fixtures.ProjectRoot(), filepath.FromSlash(cardCompleteHook)))
	if err != nil {
		return fmt.Errorf("the project ships no finishing step at %s: %w", cardCompleteHook, err)
	}
	if err := os.WriteFile(f.hook, shipped, 0o755); err != nil {
		return err
	}
	return os.Chmod(f.hook, 0o755)
}

// commitAll stages everything a directory holds and commits it as the fixture's
// own identity, so a commit the fixture writes looks the same wherever it lands.
func commitAll(dir, message string) error {
	if _, err := git(dir, "add", "-A"); err != nil {
		return err
	}
	_, err := git(dir, append(append([]string{}, gitIdentity...), "commit", "--quiet", "-m", message)...)
	return err
}

// withOrigin gives the fixture a bare origin of its own, beside it, so the
// pushes the scenarios ask for land in a repository the fixture owns rather
// than anywhere the suite works.
func (f *cardCompleteFixture) withOrigin() error {
	bare := filepath.Join(filepath.Dir(f.root), "origin.git")
	if err := os.MkdirAll(bare, 0o755); err != nil {
		return err
	}
	if _, err := git(bare, "init", "--quiet", "--bare"); err != nil {
		return err
	}
	if _, err := git(f.root, "remote", "add", "origin", bare); err != nil {
		return err
	}
	f.origin = bare
	return nil
}

// withNoOrigin leaves the fixture without a remote, which is what a checkout
// nobody has pushed anywhere looks like.
func (f *cardCompleteFixture) withNoOrigin() error {
	remotes, err := git(f.root, "remote")
	if err != nil {
		return err
	}
	if remotes != "" {
		if _, err := git(f.root, "remote", "remove", "origin"); err != nil {
			return err
		}
	}
	f.origin = ""
	return nil
}

// landCard puts the finished card's work on the fixture's branch, the way the
// merge into the worktree that holds master leaves it.
func (f *cardCompleteFixture) landCard(card string) error {
	if err := os.WriteFile(filepath.Join(f.root, card+".md"), []byte("# "+card+"\n"), 0o644); err != nil {
		return err
	}
	if err := commitAll(f.root, card+" landed on "+f.branch); err != nil {
		return err
	}
	commit, err := git(f.root, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	f.commit = commit
	return nil
}

// moveOriginOn gives the fixture's origin a commit of its own, so the card's
// branch and the remote have each moved: a push that is not forced cannot land.
func (f *cardCompleteFixture) moveOriginOn() error {
	_, err := f.moveOriginOnBy(1)
	return err
}

// moveOriginOnBy moves the fixture's origin on by commits of its own, so a
// scenario can ask what the step does however far the remote has moved ahead. It
// answers the commit the origin is left holding.
func (f *cardCompleteFixture) moveOriginOnBy(commits int) (string, error) {
	elsewhere := filepath.Join(filepath.Dir(f.root), "elsewhere")
	command := exec.Command("git", "clone", "--quiet", f.origin, elsewhere)
	if out, err := command.CombinedOutput(); err != nil {
		return "", fmt.Errorf("the fixture could not take a second checkout of its origin: %w: %s", err, out)
	}
	for round := 0; round < commits; round++ {
		theirWork := fmt.Sprintf("their-own-work-%d.md", round)
		if err := os.WriteFile(filepath.Join(elsewhere, theirWork), []byte("someone else's work\n"), 0o644); err != nil {
			return "", err
		}
		if err := commitAll(elsewhere, fmt.Sprintf("their own work, round %d", round)); err != nil {
			return "", err
		}
	}
	if _, err := git(elsewhere, "push", "--quiet", "origin", f.branch); err != nil {
		return "", err
	}
	return git(elsewhere, "rev-parse", "HEAD")
}

// runHook runs the project's finishing step the way the tooling runs it: no
// arguments, the card and the project in the environment, and the workspace set
// to the project it fired for.
func (f *cardCompleteFixture) runHook(ctx context.Context, card string) {
	command := exec.CommandContext(ctx, f.hook)
	command.Dir = f.root
	command.Env = append(os.Environ(),
		"SWARMFORGE_EVENT=card-complete",
		"SWARMFORGE_PROJECT="+f.root,
		"SWARMFORGE_TASK="+card,
		"SWARMFORGE_COMMIT="+f.commit,
		"SWARMFORGE_FROM=coder",
		"SWARMFORGE_ROLE=specifier",
		"SWARMFORGE_HOOK="+f.hook,
	)
	out, err := command.CombinedOutput()
	f.output = string(out)
	f.err = err
}

// branchCommit is the commit the fixture's branch stands at now, which a push
// must not have moved.
func (f *cardCompleteFixture) branchCommit() (string, error) {
	return git(f.root, "rev-parse", "HEAD")
}

// treeChanged reports whether the finishing step left anything behind in the
// fixture's tree.
func (f *cardCompleteFixture) treeChanged() (bool, error) {
	changes, err := git(f.root, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(changes) != "", nil
}

// originHolds is the commit the fixture's origin holds on a branch, which is
// empty when the origin holds no such branch at all.
func (f *cardCompleteFixture) originHolds(branch string) (string, error) {
	// for-each-ref answers for a branch the origin does not hold as readily as
	// for one it does: no branch and no commit, rather than a git error.
	return git(f.origin, "for-each-ref", "--format=%(objectname)", "refs/heads/"+branch)
}
