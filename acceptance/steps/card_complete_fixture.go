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

// cardCompleteProject lays out the fixture under the scenario's own directory,
// once per scenario: the repository, the card's branch, and the bridge's own
// finishing step copied in as the checkout carries it.
func (w *World) cardCompleteProject() (*cardCompleteFixture, error) {
	if w.cardComplete != nil {
		return w.cardComplete, nil
	}
	fixture := &cardCompleteFixture{
		root:   filepath.Join(w.workDir, "card-complete", "fixture"),
		branch: "master",
	}
	fixture.hook = filepath.Join(fixture.root, filepath.FromSlash(cardCompleteHook))
	if err := os.MkdirAll(filepath.Dir(fixture.hook), 0o755); err != nil {
		return nil, err
	}
	shipped, err := os.ReadFile(filepath.Join(fixtures.ProjectRoot(), filepath.FromSlash(cardCompleteHook)))
	if err != nil {
		return nil, fmt.Errorf("the project ships no finishing step at %s: %w", cardCompleteHook, err)
	}
	if err := os.WriteFile(fixture.hook, shipped, 0o755); err != nil {
		return nil, err
	}
	if err := os.Chmod(fixture.hook, 0o755); err != nil {
		return nil, err
	}
	if _, err := git(fixture.root, "init", "--quiet", "--initial-branch="+fixture.branch); err != nil {
		return nil, err
	}
	if _, err := git(fixture.root, "add", "-A"); err != nil {
		return nil, err
	}
	if _, err := git(fixture.root, append(append([]string{}, gitIdentity...),
		"commit", "--quiet", "-m", "a checkout of the bridge")...); err != nil {
		return nil, err
	}
	w.cardComplete = fixture
	return fixture, nil
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
	if _, err := git(f.root, "add", "-A"); err != nil {
		return err
	}
	if _, err := git(f.root, append(append([]string{}, gitIdentity...),
		"commit", "--quiet", "-m", card+" landed on "+f.branch)...); err != nil {
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
	elsewhere := filepath.Join(filepath.Dir(f.root), "elsewhere")
	command := exec.Command("git", "clone", "--quiet", f.origin, elsewhere)
	if out, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("the fixture could not take a second checkout of its origin: %w: %s", err, out)
	}
	if err := os.WriteFile(filepath.Join(elsewhere, "their-own-work.md"), []byte("someone else's work\n"), 0o644); err != nil {
		return err
	}
	if _, err := git(elsewhere, "add", "-A"); err != nil {
		return err
	}
	if _, err := git(elsewhere, append(append([]string{}, gitIdentity...),
		"commit", "--quiet", "-m", "their own work")...); err != nil {
		return err
	}
	_, err := git(elsewhere, "push", "--quiet", "origin", f.branch)
	return err
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
