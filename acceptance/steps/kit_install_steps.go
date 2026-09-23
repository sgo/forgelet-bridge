package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// kitScripts are the files the kit leaves in the forge's own scripts, one per
// tool, and the name each tool is reported under.
var kitScripts = []struct {
	subject string
	file    string
}{
	{"route gate", "route_card.sh"},
	{"idler check", "role_health.sh"},
	{"stall watch", "stall_watch.sh"},
}

// adapterInstallsKit runs the served forge root's adapter, which installs the
// kit into that forge, and remembers the tree it left behind.
func adapterInstallsKit(_ context.Context, world any, captures []string) error {
	return servedForgeInstallsKit(world.(*World), captures[1])
}

// adapterRunsTheKitInstallAgain runs the same command a second time, which is
// where an installer that is not idempotent shows itself.
func adapterRunsTheKitInstallAgain(_ context.Context, world any, captures []string) error {
	return servedForgeInstallsKit(world.(*World), captures[1])
}

// servedForgeInstallsKit runs install-kit for one fixture forge root. A
// self-check that read nothing leaves the installer's exit non-zero, which is
// what the report is for: the step keeps the output and lets the scenario read
// it rather than failing on the status.
func servedForgeInstallsKit(w *World, name string) error {
	ctx, cancel := stepContext()
	defer cancel()
	served, err := w.declaredForge(name)
	if err != nil {
		return err
	}
	w.servedRoot = served.Root()
	w.adapterOutput, w.kitErr = w.adapterCommand(ctx, "install-kit")
	snapshot, err := snapshotTree(served.Root())
	if err != nil {
		return err
	}
	// The step that says the installer changed nothing compares the tree with
	// the one the last install left, whichever installer left it.
	w.rulesSnapshot = snapshot
	return nil
}

// installerInstalledTheKit checks the report says the kit landed.
func installerInstalledTheKit(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	phrase := "installed the route gate, the idler check and the stall watch"
	if !strings.Contains(w.adapterOutput, phrase) {
		return fmt.Errorf("the installer's output does not say %q:\n%s", phrase, w.adapterOutput)
	}
	return nil
}

// installerSaysTheKitWasAlreadyCurrent checks the second install reports the
// kit it found rather than claiming it installed it again.
func installerSaysTheKitWasAlreadyCurrent(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	if !strings.Contains(w.adapterOutput, "the kit was already current") {
		return fmt.Errorf("the installer's output does not say the kit was already current:\n%s", w.adapterOutput)
	}
	return nil
}

// forgeCarriesTheKit checks the forge's own scripts now hold every tool.
func forgeCarriesTheKit(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := w.forgeRootOf(captures[1])
	if err != nil {
		return err
	}
	for _, tool := range kitScripts {
		path := filepath.Join(root, "swarmforge", "scripts", tool.file)
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("the forge root %s does not carry the %s: %w", captures[1], tool.subject, err)
		}
		if info.Mode()&0o111 == 0 {
			return fmt.Errorf("the %s the forge root %s carries is not executable: %s", tool.subject, captures[1], path)
		}
	}
	return nil
}

// idlerSelfCheckNamesItsCommandPaneAndMarker checks the report says what the
// check ran and what it looked for: a self-check that names neither is the
// silence this card exists to end.
func idlerSelfCheckNamesItsCommandPaneAndMarker(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	line, err := kitSelfCheckLine(w.adapterOutput, "idler check")
	if err != nil {
		return err
	}
	if !strings.Contains(line, `ran "role_health.sh `) {
		return fmt.Errorf("the self-check does not name the command it ran:\n%s", line)
	}
	if pane, ok := kitValue(line, "read pane"); !ok || pane == "-" {
		return fmt.Errorf("the self-check does not name the pane it read:\n%s", line)
	}
	if marker, ok := kitValue(line, "found marker"); !ok || marker == "" {
		return fmt.Errorf("the self-check does not name the marker it looked for:\n%s", line)
	}
	return nil
}

// idlerSelfCheckNamesTheCardAndTheMail checks the same line says what it read:
// the board row and the inbox, not just that a pane was there.
func idlerSelfCheckNamesTheCardAndTheMail(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	line, err := kitSelfCheckLine(w.adapterOutput, "idler check")
	if err != nil {
		return err
	}
	card, ok := kitValue(line, "card")
	if !ok || card == "-" {
		return fmt.Errorf("the self-check does not name the card it read:\n%s", line)
	}
	mail, ok := kitValue(line, "mail")
	if !ok {
		return fmt.Errorf("the self-check does not name the mail it read:\n%s", line)
	}
	_, count, found := strings.Cut(mail, "=")
	if !found {
		return fmt.Errorf("the self-check's mail reading %q is not a count:\n%s", mail, line)
	}
	if tally, err := strconv.Atoi(count); err != nil || tally < 1 {
		return fmt.Errorf("the self-check read %q as no mail at all:\n%s", mail, line)
	}
	return nil
}

// idlerSelfCheckReportsTheForgeNotRunning checks a forge whose roles have no
// sessions is reported as that, rather than passing as a quiet install.
func idlerSelfCheckReportsTheForgeNotRunning(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	line, err := kitSelfCheckLine(w.adapterOutput, "idler check")
	if err != nil {
		return err
	}
	if !strings.Contains(line, "the forge is not running") {
		return fmt.Errorf("the self-check does not report the forge as not running:\n%s", line)
	}
	return nil
}

// installerSaysTheSelfCheckFailed checks a tool that read nothing is a failure
// on the page rather than a clean-looking silence.
func installerSaysTheSelfCheckFailed(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	if !strings.Contains(w.adapterOutput, "self-check failed") {
		return fmt.Errorf("the installer's output never says a self-check failed:\n%s", w.adapterOutput)
	}
	return nil
}

// installerLeftTheGatePolicyAlone checks the report names the policy it found
// in the forge's own lieutenant prompt and left where it was.
func installerLeftTheGatePolicyAlone(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	if !strings.Contains(w.adapterOutput, "left the gate policy in the lieutenant prompt alone") {
		return fmt.Errorf("the installer's output does not say it left the gate policy alone:\n%s", w.adapterOutput)
	}
	return nil
}

// kitSelfCheckLine is the installer's line about one tool's self-check, whether
// that self-check passed or failed.
func kitSelfCheckLine(output, subject string) (string, error) {
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "self-check "+subject) || strings.HasPrefix(line, "self-check failed "+subject) {
			return line, nil
		}
	}
	return "", fmt.Errorf("the installer's output carries no self-check for the %s:\n%s", subject, output)
}

// kitValue is the word a self-check line put after a label.
func kitValue(line, label string) (string, bool) {
	fields := strings.Fields(line)
	for index, field := range fields {
		if field == strings.Split(label, " ")[0] && index+1 < len(fields) {
			// The labels the report uses are two words, so the value follows
			// the whole label.
			rest := strings.Join(fields[index:], " ")
			if strings.HasPrefix(rest, label+" ") {
				value := strings.TrimPrefix(rest, label+" ")
				value, _, _ = strings.Cut(value, ",")
				return strings.TrimSpace(value), true
			}
		}
	}
	return "", false
}
