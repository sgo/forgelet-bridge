package steps

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
)

// cardCompleteCase is the fixture a test works with, laid out the way a
// scenario lays it out: a project of its own, the card's work on its branch, and
// the bridge's own finishing step in place.
func cardCompleteCase(t *testing.T, withOrigin bool) *cardCompleteFixture {
	t.Helper()
	fixture := checkoutOfTheBridge(t)
	if withOrigin {
		if err := fixture.withOrigin(); err != nil {
			t.Fatalf("give the fixture a remote: %v", err)
		}
	} else {
		// A checkout that had a remote and lost it is the shape a project with
		// nowhere to push has, which is what the scenario's "no remote" means.
		if err := fixture.withOrigin(); err != nil {
			t.Fatalf("give the fixture a remote: %v", err)
		}
		if err := fixture.withNoOrigin(); err != nil {
			t.Fatalf("leave the fixture without a remote: %v", err)
		}
	}
	if err := fixture.landCard(pushCard); err != nil {
		t.Fatalf("land the card's work: %v", err)
	}
	return fixture
}

// checkoutOfTheBridge lays the fixture project out the way the scenario's own
// first step does, in a directory of its own, and leaves the card unlanded so a
// test can look at what a checkout of the bridge is before anything happens to
// it.
func checkoutOfTheBridge(t *testing.T) *cardCompleteFixture {
	t.Helper()
	w := newWorld()
	w.workDir = t.TempDir()
	fixture, err := w.cardCompleteProject()
	if err != nil {
		t.Fatalf("lay out the fixture project: %v", err)
	}
	return fixture
}

// pushCard is the card the feature works with.
const pushCard = "push-the-bridge-when-a-card-lands"

func TestTheFinishingStepLeavesTheCardOnTheRemote(t *testing.T) {
	fixture := cardCompleteCase(t, true)

	fixture.runHook(context.Background(), pushCard)

	if fixture.err != nil {
		t.Fatalf("the finishing step failed: %v\n%s", fixture.err, fixture.output)
	}
	for _, want := range []string{"pushed", pushCard} {
		if !strings.Contains(fixture.output, want) {
			t.Errorf("the finishing step does not say %q:\n%s", want, fixture.output)
		}
	}
	held, err := fixture.originHolds(fixture.branch)
	if err != nil {
		t.Fatal(err)
	}
	if held != fixture.commit {
		t.Errorf("the remote holds %q on %s, want the card's commit %s", held, fixture.branch, fixture.commit)
	}
}

func TestTheFinishingStepMindsNothingWhenTheRemoteAlreadyHoldsTheCard(t *testing.T) {
	fixture := cardCompleteCase(t, true)
	if _, err := git(fixture.root, "push", "--quiet", "origin", fixture.branch); err != nil {
		t.Fatalf("push the card before the step runs: %v", err)
	}

	fixture.runHook(context.Background(), pushCard)

	if fixture.err != nil {
		t.Fatalf("the finishing step failed on a card that was already pushed: %v\n%s", fixture.err, fixture.output)
	}
	if !strings.Contains(fixture.output, "nothing new to push") {
		t.Errorf("the finishing step does not say there was nothing to push:\n%s", fixture.output)
	}
	held, err := fixture.originHolds(fixture.branch)
	if err != nil {
		t.Fatal(err)
	}
	if held != fixture.commit {
		t.Errorf("the remote holds %q, want the card's commit %s", held, fixture.commit)
	}
}

func TestTheFinishingStepReportsARefusedPushRatherThanForcing(t *testing.T) {
	fixture := cardCompleteCase(t, true)
	if err := fixture.moveOriginOn(); err != nil {
		t.Fatalf("move the remote on: %v", err)
	}
	theirs, err := fixture.originHolds(fixture.branch)
	if err != nil {
		t.Fatal(err)
	}

	fixture.runHook(context.Background(), pushCard)

	if fixture.err == nil {
		t.Fatalf("the finishing step reported success over a push that could not land:\n%s", fixture.output)
	}
	if !strings.Contains(fixture.output, "the push failed") {
		t.Errorf("the finishing step does not say the push failed:\n%s", fixture.output)
	}
	held, err := fixture.originHolds(fixture.branch)
	if err != nil {
		t.Fatal(err)
	}
	if held != theirs {
		t.Errorf("the remote moved to %q, and a step that does not force cannot have moved it from %q", held, theirs)
	}
}

func TestTheFinishingStepStandsDownWhenThereIsNoRemoteToPushTo(t *testing.T) {
	fixture := cardCompleteCase(t, false)

	fixture.runHook(context.Background(), pushCard)

	if fixture.err != nil {
		t.Fatalf("a project with no remote failed the finishing step: %v\n%s", fixture.err, fixture.output)
	}
	if !strings.Contains(fixture.output, "no remote to push to") {
		t.Errorf("the finishing step does not say there is no remote to push to:\n%s", fixture.output)
	}
}

// A step that pushed but also committed, or left a file behind, would move the
// tree the tooling just merged into.
func TestTheFinishingStepLeavesTheFixtureProjectAlone(t *testing.T) {
	fixture := cardCompleteCase(t, true)

	fixture.runHook(context.Background(), pushCard)

	changed, err := fixture.treeChanged()
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("the finishing step left the fixture project's tree dirty")
	}
	head, err := fixture.branchCommit()
	if err != nil {
		t.Fatal(err)
	}
	if head != fixture.commit {
		t.Errorf("the finishing step moved %s to %s, want %s", fixture.branch, head, fixture.commit)
	}
}

func TestTheProjectShipsItsFinishingStepWhereTheToolingLooks(t *testing.T) {
	path := filepath.Join(fixtures.ProjectRoot(), filepath.FromSlash(cardCompleteHook))

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("the project ships no finishing step at %s: %v", cardCompleteHook, err)
	}
	if info.Mode()&0o111 == 0 {
		t.Errorf("the finishing step at %s cannot be run: %s", cardCompleteHook, info.Mode())
	}
}

// TestTheFixtureStartsAsACheckoutWithTheBridgesOwnCommit pins the shape every
// scenario stands on: the fixture is a checkout of the bridge before a card
// lands, so what a push carries is a card on top of the bridge rather than the
// first thing the repository has ever held.
func TestTheFixtureStartsAsACheckoutWithTheBridgesOwnCommit(t *testing.T) {
	fixture := checkoutOfTheBridge(t)

	commits, err := git(fixture.root, "rev-list", "--count", "HEAD")
	if err != nil {
		t.Fatalf("the fixture is not a checkout: it holds no commit: %v", err)
	}
	if commits != "1" {
		t.Errorf("the fixture holds %s commits, want the bridge's own before a card lands", commits)
	}
}

// TestTheFixtureWithoutARemoteKeepsNoRemote pins what the no-remote scenario
// means: a fixture asked for no remote has none, so the hook stands down for the
// remote's absence rather than for a fixture that only says so.
func TestTheFixtureWithoutARemoteKeepsNoRemote(t *testing.T) {
	fixture := checkoutOfTheBridge(t)
	if err := fixture.withOrigin(); err != nil {
		t.Fatalf("give the fixture a remote: %v", err)
	}
	if err := fixture.withNoOrigin(); err != nil {
		t.Fatalf("leave the fixture without a remote: %v", err)
	}

	remotes, err := git(fixture.root, "remote")
	if err != nil {
		t.Fatal(err)
	}
	if remotes != "" {
		t.Errorf("the fixture still has %q as a remote", remotes)
	}
	if fixture.origin != "" {
		t.Errorf("the fixture still names %q as the origin it pushes to", fixture.origin)
	}
}

// TestMovingTheOriginOnMovesItByTheCommitsItWasAskedFor pins the fixture's own
// arithmetic, which the scenarios that move a remote on rest on: the remote ends
// up holding the commits the scenario asked for, and the commit the fixture
// answers with is the one the remote holds.
func TestMovingTheOriginOnMovesItByTheCommitsItWasAskedFor(t *testing.T) {
	fixture := cardCompleteCase(t, true)

	theirs, err := fixture.moveOriginOnBy(2)
	if err != nil {
		t.Fatalf("move the remote on: %v", err)
	}

	if theirs == "" {
		t.Fatal("the fixture answers with no commit for a remote it moved on")
	}
	held, err := fixture.originHolds(fixture.branch)
	if err != nil {
		t.Fatal(err)
	}
	if held != theirs {
		t.Errorf("the remote holds %q, and the fixture answers with %q", held, theirs)
	}
	commits, err := git(fixture.origin, "rev-list", "--count", fixture.branch)
	if err != nil {
		t.Fatal(err)
	}
	if commits != "2" {
		t.Errorf("the remote holds %s commits, want the 2 the scenario asked for", commits)
	}
}

// TestTheFixtureSeesAChangedTree is the reading the step's promise rests on: a
// fixture the step has written to says its tree changed, so "changed nothing"
// can fail when the step leaves something behind.
func TestTheFixtureSeesAChangedTree(t *testing.T) {
	fixture := cardCompleteCase(t, true)
	changed, err := fixture.treeChanged()
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("the fixture is dirty before the finishing step runs")
	}

	if err := os.WriteFile(filepath.Join(fixture.root, "left-behind.md"), []byte("the step's own file\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err = fixture.treeChanged()
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("the fixture does not see a file left behind in its tree")
	}
}
