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

// doorbellLeftADeliveredRequestAlone checks the doorbell said a request that
// was already delivered was left where it was.
func doorbellLeftADeliveredRequestAlone(_ context.Context, world any, captures []string) error {
	return doorbellSaid(world.(*World), captures[1], "was already delivered and left alone")
}

// doorbellWaitedForTheRole checks the doorbell said it did not ring because the
// role was mid-turn, rather than injecting into it.
func doorbellWaitedForTheRole(_ context.Context, world any, captures []string) error {
	return doorbellSaid(world.(*World), captures[1], "was not rung because the role")
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
