package steps

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
)

// doorbellCommand is this repository's doorbell: the tool that rings a request
// the dashboard wrote down but never delivered.
const doorbellCommand = "doorbell.sh"

// theDoorbellRuns runs this repository's doorbell for one forge root, the way
// the forge's own copy would be run, and keeps what it said.
func theDoorbellRuns(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := w.forgeRootOf(captures[1])
	if err != nil {
		return err
	}
	ctx, cancel := stepContext()
	defer cancel()
	command := exec.CommandContext(ctx, filepath.Join(fixtures.ProjectRoot(), "swarmforge", "scripts", doorbellCommand), root)
	command.Dir = root
	out, err := command.CombinedOutput()
	w.doorbellOutput = string(out)
	if err != nil {
		return fmt.Errorf("the doorbell did not finish: %v\n%s", err, w.doorbellOutput)
	}
	return nil
}

// doorbellRangTheRequest checks the doorbell said a request it had no evidence
// for was rung.
func doorbellRangTheRequest(_ context.Context, world any, captures []string) error {
	return doorbellSaid(world.(*World), captures[1], "was never delivered and rung")
}

// doorbellLeftADeliveredRequestAlone checks the doorbell said a request the
// screen still shows was left where it was, and names that evidence.
func doorbellLeftADeliveredRequestAlone(_ context.Context, world any, captures []string) error {
	return doorbellSaid(world.(*World), captures[1], "was already delivered from the screen and left alone")
}

// doorbellLeftARequestTheScreenForgotAlone checks the doorbell read past the
// screen: a request the pane's scrollback still proves is delivery, not a
// reason to ring again.
func doorbellLeftARequestTheScreenForgotAlone(_ context.Context, world any, captures []string) error {
	return doorbellSaid(world.(*World), captures[1], "was already delivered from the scrollback and left alone")
}

// doorbellLedgerSays checks the doorbell's ledger says what became of one
// request, so the next reader can tell a delivery from a ring without reading
// the pane again.
func doorbellLedgerSays(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, request, err := w.requestByBody(captures[1])
	if err != nil {
		return err
	}
	ledger, err := readDoorbellLedger(root)
	if err != nil {
		return err
	}
	rung := fixtures.Contains(ledger["rung"], request.ID)
	delivered := fixtures.Contains(ledger["delivered"], request.ID)
	owed := fixtures.Contains(ledger["owed"], request.ID)
	fate := captures[2]
	if fate == "" {
		fate = captures[3]
	}
	switch fate {
	case "rung":
		if !rung {
			return fmt.Errorf("the ledger does not say the request %s was rung: %+v", request.ID, ledger)
		}
	case "delivered":
		if !delivered {
			return fmt.Errorf("the ledger does not say the request %s was delivered: %+v", request.ID, ledger)
		}
	case "still owed":
		// Looked at while the role was busy is not delivery: the request stays
		// owed, and a later pass rings it once the role is free.
		if !owed || rung || delivered {
			return fmt.Errorf("the ledger does not say the request %s is still owed: %+v", request.ID, ledger)
		}
	default:
		return fmt.Errorf("the step does not know the fate %q", fate)
	}
	return nil
}

// doorbellWaitedForTheRole checks the doorbell said it did not ring because the
// role was mid-turn, rather than injecting into it.
func doorbellWaitedForTheRole(_ context.Context, world any, captures []string) error {
	return doorbellSaid(world.(*World), captures[1], "was not rung because the role")
}

// doorbellRungTheUnansweredRequest checks the pass read a delivered request the
// dashboard still holds as unanswered and rang it again: words arriving is not
// a session acting on them.
func doorbellRungTheUnansweredRequest(_ context.Context, world any, captures []string) error {
	return doorbellSaid(world.(*World), captures[1], "was never answered and was rung again")
}

// doorbellReportsItIsStillUnanswered checks a request that has been rung its
// fill is reported rather than rung forever.
func doorbellReportsItIsStillUnanswered(_ context.Context, world any, captures []string) error {
	return doorbellSaid(world.(*World), captures[1], "is still unanswered")
}

// theRingSaysWhichRingItIs checks the ring itself says this is not the first
// time, so a pane that has seen the same alert before knows it is not new.
func theRingSaysWhichRingItIs(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	text, err := w.ringInTheMasterPane()
	if err != nil {
		return err
	}
	// A pane wraps what it holds, so the words are read the way a reader reads
	// them: with the wrapping taken out.
	read := squashed(text)
	for _, want := range []string{"the doorbell's second ring", captures[1]} {
		if !strings.Contains(read, squashed(want)) {
			return fmt.Errorf("the ring does not say %q:\n%s", want, text)
		}
	}
	return nil
}

// theDoorbellDidNotRingItAgain checks the pass left the pane as it was: a ring
// carries the command that answers the request, so a ring of this one would show
// it, and the pass says it rang nothing.
func theDoorbellDidNotRingItAgain(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	body := captures[1]
	_, request, err := w.requestByBody(body)
	if err != nil {
		return err
	}
	text, err := w.ringInTheMasterPane()
	if err != nil {
		return err
	}
	if ring := squashed("Answer with: pack_dashboard_request.sh answer " + request.ID); strings.Contains(squashed(text), ring) {
		return fmt.Errorf("the pane holds a ring for the request %q, and the pass had already rung it its fill:\n%s\n\nthe pass said:\n%s",
			body, text, w.doorbellOutput)
	}
	if strings.Contains(w.doorbellOutput, "rung again") || strings.Contains(w.doorbellOutput, "rung into") {
		return fmt.Errorf("the doorbell rang a request it had already rung its fill:\n%s", w.doorbellOutput)
	}
	return nil
}

// squashed is a reading with its whitespace taken out: a pane wraps what it
// holds, and the words are the same words whatever the width.
func squashed(text string) string {
	return strings.Join(strings.Fields(text), "")
}

// doorbellSaysNothingIsPending checks a queue with nothing left in it is read as
// exactly that: a request that was answered is gone, not pending.
func doorbellSaysNothingIsPending(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	if !strings.Contains(w.doorbellOutput, "nothing pending") {
		return fmt.Errorf("the doorbell does not say nothing is pending:\n%s", w.doorbellOutput)
	}
	return nil
}

// doorbellSaid checks the doorbell's report about one request carried a phrase.
func doorbellSaid(w *World, body, phrase string) error {
	if w.doorbellOutput == "" {
		return fmt.Errorf("the doorbell has not run")
	}
	wanted := fmt.Sprintf("the chat request %q %s", body, phrase)
	if !strings.Contains(w.doorbellOutput, wanted) {
		return fmt.Errorf("the doorbell does not say %q:\n%s", wanted, w.doorbellOutput)
	}
	return nil
}

// theMasterPaneHoldsTheRequestTheDoorbellTyped checks the request's own id is
// in the pane the doorbell rang: the typing is the delivery, and the pane is
// the only evidence of it there is.
func theMasterPaneHoldsTheRequestTheDoorbellTyped(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	body := captures[1]
	root, request, err := w.requestByBody(body)
	if err != nil {
		return err
	}
	pane, socket, err := w.masterPane(root)
	if err != nil {
		return err
	}
	text, err := paneText(socket, pane)
	if err != nil {
		return err
	}
	if !strings.Contains(text, "["+request.ID+"]") || !strings.Contains(text, body) {
		return fmt.Errorf("the pane %s does not hold the request the doorbell typed ([%s] %s):\n%s",
			pane, request.ID, body, text)
	}
	return nil
}

// theRingSaysTheGateIsTheOperators checks the ring carries the approval's gate:
// the operator decides it, and the session that meets it may not approve for
// them. The words ride with the ring rather than waiting in a prompt that may
// never be read.
func theRingSaysTheGateIsTheOperators(_ context.Context, world any, _ []string) error {
	return theRingSays(world.(*World),
		"the gate is the operator's",
		"Do not approve unless the operator says to")
}

// theRingSaysTheAnswerIsTheOperatorsToGive checks the other half: a
// clarification is the operator's answer to give, and the session may not answer
// for them.
func theRingSaysTheAnswerIsTheOperatorsToGive(_ context.Context, world any, _ []string) error {
	return theRingSays(world.(*World),
		"the answer is the operator's to give",
		"Do not answer it unless the operator says to")
}

// theRingCarriesNeitherClause checks a request that is neither an approval nor a
// clarification carries neither clause, so the words mean what they say.
func theRingCarriesNeitherClause(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	text, err := w.ringInTheMasterPane()
	if err != nil {
		return err
	}
	for _, clause := range []string{
		"the gate is the operator's",
		"Do not approve unless the operator says to",
		"the answer is the operator's to give",
		"Do not answer it unless the operator says to",
	} {
		if strings.Contains(text, clause) {
			return fmt.Errorf("the ring carries %q, and this request holds no such gate:\n%s", clause, text)
		}
	}
	return nil
}

// theRingSays checks the ring the doorbell typed carried the words of one gate.
func theRingSays(w *World, wants ...string) error {
	text, err := w.ringInTheMasterPane()
	if err != nil {
		return err
	}
	for _, want := range wants {
		if !strings.Contains(text, want) {
			return fmt.Errorf("the ring does not say %q:\n%s", want, text)
		}
	}
	return nil
}

// ringInTheMasterPane is what the doorbell typed into the master role's pane:
// the ring itself, which is the only place a clause reaches a session.
func (w *World) ringInTheMasterPane() (string, error) {
	root, err := w.theForgeRoot()
	if err != nil {
		return "", err
	}
	pane, socket, err := w.masterPane(root)
	if err != nil {
		return "", err
	}
	return paneText(socket, pane)
}
