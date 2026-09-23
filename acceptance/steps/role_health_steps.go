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

// idlerCheckCommand is this repository's idler check: the tool a forge's own
// copy is a deployment of.
const idlerCheckCommand = "role_health.sh"

// idlerCheckRuns runs the check against one project, the way the forge runs it.
func idlerCheckRuns(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	projectDir, err := w.pathOf(captures[2], captures[1])
	if err != nil {
		return err
	}
	ctx, cancel := stepContext()
	defer cancel()
	command := exec.CommandContext(ctx,
		filepath.Join(fixtures.ProjectRoot(), "swarmforge", "scripts", idlerCheckCommand), projectDir)
	command.Dir = projectDir
	command.Env = append(os.Environ(), "ROLE_HEALTH_CLAUDE_PROJECTS="+w.claudeProjects())
	out, err := command.CombinedOutput()
	w.idlerOutput = string(out)
	w.idlerErr = err
	return nil
}

// idlerCheckReports checks what the check said about one role.
func idlerCheckReports(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	role, phrase := captures[1], captures[2]
	verdict, card, err := idlerVerdict(phrase)
	if err != nil {
		return err
	}
	line, err := idlerLine(w, role)
	if err != nil {
		return err
	}
	columns := strings.Fields(line)
	if len(columns) < 2 || columns[1] != verdict {
		return fmt.Errorf("the idler check reports the role %s as %q, want %s:\n%s", role, columns[1], verdict, w.idlerOutput)
	}
	if card != "" && !strings.Contains(line, card) {
		return fmt.Errorf("the idler check's line for %s does not name the card %s:\n%s", role, card, line)
	}
	return nil
}

// idlerCheckNamesTool checks the check judged the role against the tool its own
// row records.
func idlerCheckNamesTool(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	tool, role := captures[1], captures[2]
	line, err := idlerLine(w, role)
	if err != nil {
		return err
	}
	if !strings.Contains(line, "tool="+tool) {
		return fmt.Errorf("the idler check does not name the tool %s for the role %s:\n%s", tool, role, line)
	}
	return nil
}

// idlerCheckStatus checks the check's exit status: it fails when any role is
// stalled and passes when none is.
func idlerCheckStatus(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	if w.idlerOutput == "" {
		return fmt.Errorf("the idler check has not run")
	}
	switch captures[1] {
	case "fails":
		if w.idlerErr == nil {
			return fmt.Errorf("the idler check reported no stall:\n%s", w.idlerOutput)
		}
	case "passes":
		if w.idlerErr != nil {
			return fmt.Errorf("the idler check failed with no stall to report:\n%s", w.idlerOutput)
		}
	default:
		return fmt.Errorf("the step does not know the wording %q", captures[1])
	}
	return nil
}

// idlerCheckReportsForgeNotRunning checks the check says a forge with no role
// sessions is not running, rather than filing every role as a dead agent.
func idlerCheckReportsForgeNotRunning(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	line, err := idlerLine(w, "forge")
	if err != nil {
		return err
	}
	if columns := strings.Fields(line); len(columns) < 2 || columns[1] != "not-running" {
		return fmt.Errorf("the idler check does not report the forge as not running:\n%s", w.idlerOutput)
	}
	return nil
}

// idlerLine is the check's report line for one role.
// idlerCheckReportsTheNoteMissing checks the check says the card's note went
// missing, which is its own verdict: the board says a role holds work and
// nothing anywhere exists to hand it over.
func idlerCheckReportsTheNoteMissing(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	card := captures[1]
	for _, line := range strings.Split(strings.TrimRight(w.idlerOutput, "\n"), "\n") {
		columns := strings.Fields(line)
		if len(columns) >= 3 && columns[1] == "note-missing" && strings.Contains(columns[2], card) {
			return nil
		}
	}
	return fmt.Errorf("the idler check does not report the note for the card %s as missing:\n%s", card, w.idlerOutput)
}

// idlerLine is the check's report line for one role.
func idlerLine(w *World, role string) (string, error) {
	for _, line := range strings.Split(strings.TrimRight(w.idlerOutput, "\n"), "\n") {
		if columns := strings.Fields(line); len(columns) > 0 && columns[0] == role {
			return line, nil
		}
	}
	return "", fmt.Errorf("the idler check reports nothing for the role %s:\n%s", role, w.idlerOutput)
}

// idlerVerdict turns the wording a scenario uses into the verdict the check
// prints, and the card that wording names when it names one.
func idlerVerdict(phrase string) (string, string, error) {
	switch {
	case strings.HasPrefix(phrase, "idle holding the card "):
		return "idle-holding-card", strings.TrimPrefix(phrase, "idle holding the card "), nil
	case phrase == "idle with nothing to pick up":
		return "idle-nothing-to-pick-up", "", nil
	case phrase == "idle with nothing assigned":
		return "idle-nothing-assigned", "", nil
	case phrase == "working":
		return "working", "", nil
	case phrase == "running a tool it does not know":
		return "tool-not-known", "", nil
	case phrase == "waiting on a decision":
		return "waiting-on-a-decision", "", nil
	case phrase == "waiting on the operator":
		return "waiting-on-the-operator", "", nil
	}
	return "", "", fmt.Errorf("the step does not know the wording %q", phrase)
}
