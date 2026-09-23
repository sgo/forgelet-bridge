// Package kit installs the tools that make a forge behave - the route gate, the
// idler check and the stall watch - into a forge's own scripts, with the watch's
// agent, and self-checks each one against the forge it was installed into.
//
// The kit ships tools, not policy: how strict the gate is belongs to the
// forge's own lieutenant prompt, so the installer finds that wording, leaves it
// where it is, and says which one it found. Every self-check is written down in
// the report, command and marker together, because a tool that reads nothing
// looks exactly like a quiet forge.
package kit

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// kitName is how the report names the tools together.
const kitName = "the route gate, the idler check and the stall watch"

// Tool is one tool the kit ships: the files it is made of, and the name the
// report uses for it.
type Tool struct {
	Subject string
	Files   []string
}

// Tools is the kit, in the order the report names it.
func Tools() []Tool {
	return []Tool{
		{Subject: "route gate", Files: []string{"route_card.sh", "route_card.bb"}},
		{Subject: "idler check", Files: []string{"role_health.sh", "role_health.bb"}},
		{Subject: "stall watch", Files: []string{"stall_watch.sh"}},
	}
}

// Report is what an install did, in the order it happened.
type Report struct {
	lines  []string
	failed bool
}

// String is the report whoever ran the installer reads.
func (r Report) String() string { return strings.Join(r.lines, "\n") }

// Failed reports whether any self-check found a tool that reads nothing.
func (r Report) Failed() bool { return r.failed }

func (r *Report) line(text string) { r.lines = append(r.lines, text) }

// Install copies the kit from kitDir into the forge's own scripts, writes the
// agent the watch would run, and self-checks every tool against the forge.
func Install(forgeRoot, kitDir string) (Report, error) {
	var report Report
	// Everything the report names, and every tool this runs, reads the forge by
	// its full path: a tool that resolves the root itself would otherwise name
	// a different place in the report than the one the installer worked on.
	absolute, err := filepath.Abs(forgeRoot)
	if err != nil {
		return report, err
	}
	forgeRoot = absolute
	info, err := os.Stat(forgeRoot)
	if err != nil {
		return report, fmt.Errorf("the forge %s is not there: %w", forgeRoot, err)
	}
	if !info.IsDir() {
		return report, fmt.Errorf("the forge %s is not a directory", forgeRoot)
	}
	scripts := filepath.Join(forgeRoot, "swarmforge", "scripts")
	if err := os.MkdirAll(scripts, 0o755); err != nil {
		return report, err
	}
	changed := false
	for _, tool := range Tools() {
		for _, file := range tool.Files {
			outcome, err := installFile(tool.Subject, filepath.Join(kitDir, file), filepath.Join(scripts, file))
			if err != nil {
				return report, err
			}
			report.line(outcome.line)
			changed = changed || outcome.changed
		}
	}
	if changed {
		report.line("installed " + kitName + " in " + scripts)
	} else {
		report.line("the kit was already current in " + scripts)
	}
	if err := installAgent(&report, scripts, forgeRoot); err != nil {
		return report, err
	}
	report.line("left alone loading the stall watch's agent: the machine's own step, and the agent it runs is written down")
	selfChecks(&report, scripts, forgeRoot)
	policy(&report, forgeRoot)
	return report, nil
}

// installOutcome is what copying one file did.
type installOutcome struct {
	line    string
	changed bool
}

// installFile puts one file of the kit into the forge's scripts, and says what
// it did: a file that already holds exactly what the kit ships is current, and
// anything else is replaced.
func installFile(subject, source, target string) (installOutcome, error) {
	wanted, err := os.ReadFile(source)
	if err != nil {
		return installOutcome{}, fmt.Errorf("the kit is missing %s: %w", filepath.Base(source), err)
	}
	existing, err := os.ReadFile(target)
	switch {
	case err == nil && bytes.Equal(existing, wanted):
		return installOutcome{line: "already current " + subject + " in " + target}, nil
	case err != nil && !os.IsNotExist(err):
		return installOutcome{}, err
	}
	mode := os.FileMode(0o644)
	if strings.HasSuffix(target, ".sh") {
		mode = 0o755
	}
	if err := os.WriteFile(target, wanted, mode); err != nil {
		return installOutcome{}, err
	}
	if err := os.Chmod(target, mode); err != nil {
		return installOutcome{}, err
	}
	return installOutcome{line: "changed " + subject + " in " + target, changed: true}, nil
}

// installAgent writes down the agent the installed watch would leave for the
// machine to run. Loading it is the machine's own step: the installer ships the
// schedule, it does not take over the one the machine already keeps.
func installAgent(report *Report, scripts, forgeRoot string) error {
	out, err := run(filepath.Join(scripts, "stall_watch.sh"), "print-agent", forgeRoot)
	if err != nil {
		return fmt.Errorf("the stall watch could not print the agent it would write: %w", err)
	}
	path := filepath.Join(forgeRoot, ".swarmforge", "stall-watch.plist")
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, []byte(out)) {
		report.line("already current the stall watch's agent " + path)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return err
	}
	report.line("wrote the stall watch's agent " + path)
	return nil
}

// selfChecks runs each installed tool against the forge and writes down what it
// ran and what it looked for. A reading that does not fit is a failure in the
// report, not silence.
func selfChecks(report *Report, scripts, forgeRoot string) {
	for _, subject := range []string{"route gate", "idler check", "stall watch"} {
		var line string
		var ok bool
		switch subject {
		case "route gate":
			line, ok = gateSelfCheck(scripts, forgeRoot)
		case "idler check":
			line, ok = idlerSelfCheck(scripts, forgeRoot)
		default:
			line, ok = watchSelfCheck(scripts, forgeRoot)
		}
		report.line(line)
		report.failed = report.failed || !ok
	}
}

// selfCheckProposal is the proposal the gate is asked about: one this forge
// does not hold.
const selfCheckProposal = "install-kit-self-check"

// gateSelfCheck asks the gate about a proposal the forge does not hold. A gate
// that reads this forge refuses it by name; a gate that reads nothing says
// nothing at all.
func gateSelfCheck(scripts, forgeRoot string) (string, bool) {
	command := "route_card.sh commit " + selfCheckProposal + " --forge-root " + forgeRoot
	marker := "No proposal record for " + selfCheckProposal
	out, _ := run(filepath.Join(scripts, "route_card.sh"), "commit", selfCheckProposal, "--forge-root", forgeRoot)
	if !strings.Contains(out, marker) {
		return fmt.Sprintf("self-check failed route gate: ran %q and never said %q, so it did not read this forge's proposal store", command, marker), false
	}
	return fmt.Sprintf("self-check route gate: ran %q, found marker %q", command, marker), true
}

// idlerSelfCheck runs the check over every project the forge serves and reports
// the first reading that shows it read the forge: a live pane, a board row and
// an inbox.
func idlerSelfCheck(scripts, forgeRoot string) (string, bool) {
	projects, err := projects(forgeRoot)
	if err != nil || len(projects) == 0 {
		return "self-check failed idler check: the forge serves no project to read", false
	}
	var first string
	for _, project := range projects {
		command := "role_health.sh " + project + " --forge-root " + forgeRoot
		out, _ := run(filepath.Join(scripts, "role_health.sh"), project, "--forge-root", forgeRoot)
		line, ok := idlerEvidence(project, command, out)
		if ok {
			return line, true
		}
		if first == "" {
			first = line
		}
	}
	return first, false
}

// idlerEvidence is what one run of the check read: the pane, the marker it
// found, the card and the mail. A run that read nothing says which of the three
// it could not read.
func idlerEvidence(project, command, output string) (string, bool) {
	notRunning, readings := parseIdlerReport(output)
	if notRunning {
		return fmt.Sprintf("self-check failed idler check: ran %q, the forge is not running", command), false
	}
	if len(readings) == 0 {
		return fmt.Sprintf("self-check failed idler check: ran %q, which read no role", command), false
	}
	var first string
	for _, reading := range readings {
		evidence := fmt.Sprintf("read pane %s, found marker %s", paneOf(project, reading.Role), reading.Verdict)
		switch {
		case reading.Verdict == "session-gone":
			if first == "" {
				first = fmt.Sprintf("self-check failed idler check: ran %q, %s, and there is no live pane behind it", command, evidence)
			}
		case reading.Card == "-" || reading.MailCount == 0:
			if first == "" {
				first = fmt.Sprintf("self-check failed idler check: ran %q, %s, with no board row or mail to read", command, evidence)
			}
		default:
			return fmt.Sprintf("self-check idler check: ran %q, %s, card %s, mail %s", command, evidence, reading.Card, reading.Mail), true
		}
	}
	return first, false
}

// reading is one role's line from the check's report: the verdict it reached,
// the card it saw on the board, and the mail it found.
type reading struct {
	Role      string
	Verdict   string
	Card      string
	Mail      string
	MailCount int
}

// parseIdlerReport reads the check's report: whether the forge has no sessions
// at all, and what it read for each role.
func parseIdlerReport(output string) (bool, []reading) {
	notRunning := false
	var readings []reading
	for _, line := range strings.Split(strings.TrimRight(output, "\n"), "\n") {
		columns := strings.Fields(line)
		if len(columns) < 2 {
			continue
		}
		if columns[1] == "not-running" {
			notRunning = true
			continue
		}
		found := reading{Role: columns[0], Verdict: columns[1], Card: "-"}
		if len(columns) > 2 && !strings.Contains(columns[2], "=") {
			found.Card = columns[2]
		}
		for _, column := range columns {
			count, ok := countOf(column, "new=")
			if ok && count > 0 {
				found.Mail, found.MailCount = "new="+strconv.Itoa(count), count
				continue
			}
			count, ok = countOf(column, "in_process=")
			if ok && count > 0 && found.MailCount == 0 {
				found.Mail, found.MailCount = "in_process="+strconv.Itoa(count), count
			}
		}
		readings = append(readings, found)
	}
	return notRunning, readings
}

// countOf reads one count column of a report line.
func countOf(column, prefix string) (int, bool) {
	if !strings.HasPrefix(column, prefix) {
		return 0, false
	}
	count, err := strconv.Atoi(strings.TrimPrefix(column, prefix))
	return count, err == nil
}

// watchSelfCheck asks the installed watch for the agent it would write: the
// marker is this forge's own root, so an agent that names another forge, or
// none, is a failure rather than a quiet pass.
func watchSelfCheck(scripts, forgeRoot string) (string, bool) {
	command := "stall_watch.sh print-agent " + forgeRoot
	marker := "<string>" + forgeRoot + "</string>"
	out, err := run(filepath.Join(scripts, "stall_watch.sh"), "print-agent", forgeRoot)
	if err != nil || !strings.Contains(out, marker) {
		return fmt.Sprintf("self-check failed stall watch: ran %q and never named this forge (%q)", command, marker), false
	}
	return fmt.Sprintf("self-check stall watch: ran %q, found marker %q", command, marker), true
}

// policy writes down the gate policy the installer found and left where it was.
func policy(report *Report, forgeRoot string) {
	path := filepath.Join(forgeRoot, "swarmforge", "roles", "lieutenant.prompt")
	data, err := os.ReadFile(path)
	if err != nil {
		report.line("left the gate policy alone: the lieutenant prompt " + path + " is not there, so this forge has not said how strict its gate is")
		return
	}
	report.line(fmt.Sprintf("left the gate policy in the lieutenant prompt alone, which asks: %s, at %s", firstLine(string(data)), path))
}

// firstLine is the wording a prompt opens with, which is what a report can name.
func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return "(written down, but empty)"
}

// projects are the projects a forge root serves, in name order.
func projects(forgeRoot string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(forgeRoot, "projects"))
	if err != nil {
		return nil, err
	}
	var found []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		project := filepath.Join(forgeRoot, "projects", entry.Name())
		if _, err := os.Stat(filepath.Join(project, ".swarmforge", "roles.tsv")); err != nil {
			continue
		}
		found = append(found, project)
	}
	sort.Strings(found)
	return found, nil
}

// paneOf is the pane one role of a project is served in, as the project's own
// roles file records it.
func paneOf(project, role string) string {
	data, err := os.ReadFile(filepath.Join(project, ".swarmforge", "roles.tsv"))
	if err != nil {
		return "-"
	}
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		columns := strings.Split(line, "\t")
		if len(columns) >= 4 && columns[0] == role {
			return columns[3]
		}
	}
	return "-"
}

// run runs one installed tool and keeps everything it said. A tool that refuses
// is often saying exactly what the self-check wants to read, so the exit status
// is the caller's to judge.
func run(command string, args ...string) (string, error) {
	out, err := exec.Command(command, args...).CombinedOutput()
	return string(out), err
}
