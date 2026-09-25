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
	w := newWorld()
	w.workDir = t.TempDir()
	fixture, err := w.cardCompleteProject()
	if err != nil {
		t.Fatalf("lay out the fixture project: %v", err)
	}
	if withOrigin {
		if err := fixture.withOrigin(); err != nil {
			t.Fatalf("give the fixture a remote: %v", err)
		}
	}
	if err := fixture.landCard(pushCard); err != nil {
		t.Fatalf("land the card's work: %v", err)
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
