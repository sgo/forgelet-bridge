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

// forgeScheduleCommand is this repository's schedule: the cadence that makes
// the watch's pass and the doorbell's, and the thing the installed agent runs.
const forgeScheduleCommand = "forge_schedule.sh"

// theForgeScheduleRuns runs this repository's schedule over the fixture forge
// roots, the way the machine's agent runs it. A pass that failed is named and
// the rest still runs, so the step keeps the words and lets the scenario read
// them rather than failing on the status.
func theForgeScheduleRuns(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	roots, err := w.rootsOf(captures[1])
	if err != nil {
		return err
	}
	ctx, cancel := stepContext()
	defer cancel()
	command := exec.CommandContext(ctx, filepath.Join(fixtures.ProjectRoot(), "swarmforge", "scripts", forgeScheduleCommand),
		append([]string{"run"}, roots...)...)
	command.Dir = roots[0]
	out, _ := command.CombinedOutput()
	w.scheduleOutput = string(out)
	w.scheduleRoots = roots
	// The schedule's words carry every pass's, so a step about the doorbell
	// reads the same whether the doorbell ran alone or was run by the machine.
	w.doorbellOutput = w.scheduleOutput
	return nil
}

// theScheduleLeavesEvidenceItRan checks the schedule's own heartbeat: a cadence
// that stopped is visible rather than assumed.
func theScheduleLeavesEvidenceItRan(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	if len(w.scheduleRoots) == 0 {
		return fmt.Errorf("the schedule has not run")
	}
	heartbeat := filepath.Join(w.scheduleRoots[0], ".swarmforge", "forge-schedule.heartbeat")
	data, err := os.ReadFile(heartbeat)
	if err != nil {
		return fmt.Errorf("the schedule left no evidence it ran: %w", err)
	}
	line := strings.TrimSpace(string(data))
	if !strings.Contains(line, "roots=") {
		return fmt.Errorf("the schedule's heartbeat does not say what it ran: %q", line)
	}
	return nil
}

// theScheduleNamesTheRootItCouldNotServe checks a root the schedule could not
// serve is named rather than skipped in silence.
func theScheduleNamesTheRootItCouldNotServe(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	if w.scheduleOutput == "" {
		return fmt.Errorf("the schedule has not run")
	}
	if !strings.Contains(w.scheduleOutput, "could not serve the forge root") {
		return fmt.Errorf("the schedule names no root it could not serve:\n%s", w.scheduleOutput)
	}
	return nil
}
