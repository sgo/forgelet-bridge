package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
)

// theFixtureProjectIsACheckoutOfTheBridge lays out the throwaway project the
// card-complete scenarios work with: a project of its own, holding the bridge's
// own finishing step where the tooling looks for it, so a scenario asks what
// the step does rather than describing what it should do.
func theFixtureProjectIsACheckoutOfTheBridge(_ context.Context, world any, _ []string) error {
	_, err := world.(*World).cardCompleteProject()
	return err
}

// theFixtureProjectHasARemote gives the fixture the origin its push has to
// reach.
func theFixtureProjectHasARemote(_ context.Context, world any, _ []string) error {
	fixture, err := world.(*World).cardCompleteProject()
	if err != nil {
		return err
	}
	return fixture.withOrigin()
}

// theFixtureProjectHasNoRemote leaves the fixture with nowhere to push, which
// is not a failure: there is no remote rather than work that was lost.
func theFixtureProjectHasNoRemote(_ context.Context, world any, _ []string) error {
	fixture, err := world.(*World).cardCompleteProject()
	if err != nil {
		return err
	}
	return fixture.withNoOrigin()
}

// theFixtureProjectHoldsTheFinishedCardOnItsMaster lands the card's work on the
// fixture's own branch: one commit, which is what a push has to carry.
func theFixtureProjectHoldsTheFinishedCardOnItsMaster(_ context.Context, world any, captures []string) error {
	fixture, err := world.(*World).cardCompleteProject()
	if err != nil {
		return err
	}
	return fixture.landCard(captures[1])
}

// theFixtureProjectsRemoteHasACommitOfItsOwn moves the fixture's origin on, so
// the card's branch and the remote have each moved: pushing is refused unless
// the step forces, and forcing is not the step's business.
func theFixtureProjectsRemoteHasACommitOfItsOwn(_ context.Context, world any, _ []string) error {
	fixture, err := world.(*World).cardCompleteProject()
	if err != nil {
		return err
	}
	return fixture.moveOriginOn()
}

// theCardsWorkHasAlreadyReachedTheRemote pushes the fixture's branch the way a
// card whose work is already pushed leaves the remote.
func theCardsWorkHasAlreadyReachedTheRemote(_ context.Context, world any, _ []string) error {
	fixture, err := world.(*World).cardCompleteProject()
	if err != nil {
		return err
	}
	_, err = git(fixture.root, "push", "--quiet", "origin", fixture.branch)
	return err
}

// theFinishingStepRunsForTheCard runs the fixture's copy of the project's
// finishing step the way the tooling runs it: no arguments, the card in the
// environment. Running it again is the same step, which is where a step that
// minds being run twice shows itself.
func theFinishingStepRunsForTheCard(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	fixture, err := w.cardCompleteProject()
	if err != nil {
		return err
	}
	ctx, cancel := stepContext()
	defer cancel()
	fixture.runHook(ctx, captures[1])
	return nil
}

// theFixtureProjectsRemoteHoldsTheCardsWorkOnItsMaster checks the push landed:
// the origin's branch stands at the commit the card's work landed on.
func theFixtureProjectsRemoteHoldsTheCardsWorkOnItsMaster(_ context.Context, world any, _ []string) error {
	fixture, err := world.(*World).cardCompleteProject()
	if err != nil {
		return err
	}
	held, err := fixture.originHolds(fixture.branch)
	if err != nil {
		return err
	}
	if held != fixture.commit {
		return fmt.Errorf("the remote holds %s on %s, and the card landed on %s", heldOrNothing(held), fixture.branch, fixture.commit)
	}
	return nil
}

// theFixtureProjectsRemoteHoldsNothingOfTheCard checks a refused push left the
// remote where it was rather than overwriting it.
func theFixtureProjectsRemoteHoldsNothingOfTheCard(_ context.Context, world any, _ []string) error {
	fixture, err := world.(*World).cardCompleteProject()
	if err != nil {
		return err
	}
	held, err := fixture.originHolds(fixture.branch)
	if err != nil {
		return err
	}
	if held == fixture.commit {
		return fmt.Errorf("the remote holds the card's work %s, which a refused push cannot have put there", fixture.commit)
	}
	return nil
}

// theFinishingStepSaysItPushedTheCard checks the step names both what it did and
// the card it did it for.
func theFinishingStepSaysItPushedTheCard(_ context.Context, world any, captures []string) error {
	return hookSays(world.(*World), captures[1], "pushed")
}

// theFinishingStepSaysThePushFailed checks a failed push is reported rather than
// passed over in silence.
func theFinishingStepSaysThePushFailed(_ context.Context, world any, _ []string) error {
	return hookSays(world.(*World), "the push failed")
}

// theFinishingStepSaysThereWasNothingNewToPush checks a card whose work is
// already on the remote is read as exactly that.
func theFinishingStepSaysThereWasNothingNewToPush(_ context.Context, world any, _ []string) error {
	return hookSays(world.(*World), "nothing new to push")
}

// theFinishingStepSaysThereIsNoRemoteToPushTo checks a project with nowhere to
// send its work says so instead of failing.
func theFinishingStepSaysThereIsNoRemoteToPushTo(_ context.Context, world any, _ []string) error {
	return hookSays(world.(*World), "no remote to push to")
}

// theFinishingStepSucceeded checks the step's own exit status: a step that
// failed has told the tooling to print HOOK_FAILED, and the scenario says which
// of the two this run was.
func theFinishingStepSucceeded(_ context.Context, world any, _ []string) error {
	fixture, err := world.(*World).cardCompleteProject()
	if err != nil {
		return err
	}
	if fixture.err != nil {
		return fmt.Errorf("the finishing step failed: %v\n%s", fixture.err, fixture.output)
	}
	return nil
}

// theFinishingStepFailed checks the other verdict, so a step that failed
// quietly is read as a failure whatever its page said.
func theFinishingStepFailed(_ context.Context, world any, _ []string) error {
	fixture, err := world.(*World).cardCompleteProject()
	if err != nil {
		return err
	}
	if fixture.err == nil {
		return fmt.Errorf("the finishing step reported success over a push that did not land:\n%s", fixture.output)
	}
	return nil
}

// theFinishingStepChangedNothingInTheFixtureProjectTree checks the step pushed
// and nothing else: no commit, no branch moved, no file left behind.
func theFinishingStepChangedNothingInTheFixtureProjectTree(_ context.Context, world any, _ []string) error {
	fixture, err := world.(*World).cardCompleteProject()
	if err != nil {
		return err
	}
	changed, err := fixture.treeChanged()
	if err != nil {
		return err
	}
	if changed {
		return fmt.Errorf("the finishing step left the fixture project's tree dirty")
	}
	head, err := fixture.branchCommit()
	if err != nil {
		return err
	}
	if head != fixture.commit {
		return fmt.Errorf("the finishing step moved %s to %s, and the card landed on %s", fixture.branch, head, fixture.commit)
	}
	return nil
}

// theProjectShipsItsFinishingStepAt checks the deliverable itself: the bridge
// carries the step at the path the tooling runs, and runs it.
func theProjectShipsItsFinishingStepAt(_ context.Context, world any, captures []string) error {
	path := filepath.Join(fixtures.ProjectRoot(), filepath.FromSlash(captures[1]))
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("the project ships no finishing step at %s: %w", captures[1], err)
	}
	if info.Mode()&0o111 == 0 {
		return fmt.Errorf("the finishing step the project ships at %s cannot be run: %s", captures[1], info.Mode())
	}
	return nil
}

// hookSays checks the finishing step's own page carries what the scenario read.
func hookSays(w *World, wants ...string) error {
	fixture, err := w.cardCompleteProject()
	if err != nil {
		return err
	}
	for _, want := range wants {
		if !strings.Contains(fixture.output, want) {
			return fmt.Errorf("the finishing step does not say %q:\n%s", want, fixture.output)
		}
	}
	return nil
}

// heldOrNothing is a commit a remote holds, or a word for its holding none.
func heldOrNothing(commit string) string {
	if strings.TrimSpace(commit) == "" {
		return "nothing"
	}
	return commit
}
