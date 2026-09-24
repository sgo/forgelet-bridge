package kit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// selfChecks runs every tool the kit ships against the forge and writes down
// what it ran and what it looked for. A reading that does not fit is a failure
// in the report, not silence.
func selfChecks(report *Report, scripts, forgeRoot string) {
	for _, tool := range Tools() {
		line, ok := tool.Check(scripts, forgeRoot)
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
// what it read. The pane is the reading most worth proving - it is the one that
// broke when Claude Code started reporting its version instead of its name - but
// a forge that is not running is not a fault: installing the tools before
// starting the forge is the order most people choose. Such a pass succeeds and
// names the projects it read and the pane it could not prove, so a later run can
// prove it. What still fails is a forge with no structure to read at all: no
// roles file, no board and no inbox, which is a wrong path or a wrong forge
// rather than a quiet one.
func idlerSelfCheck(scripts, forgeRoot string) (string, bool) {
	projects, err := projects(forgeRoot)
	if err != nil || len(projects) == 0 {
		return fmt.Sprintf("self-check failed idler check: %s holds no project with a roles file, so it read no roles, no board and no inbox",
			filepath.Join(forgeRoot, "projects")), false
	}
	var unproved []string
	var first string
	for _, project := range projects {
		line, proved, skipped := idlerProjectRead(scripts, forgeRoot, project)
		if skipped {
			unproved = append(unproved, project)
			continue
		}
		if proved {
			return line, true
		}
		if first == "" {
			first = line
		}
	}
	// A project the tool could not read at all is a fault whatever else the
	// forge holds: a pane that is not up is the only reading excused.
	if len(unproved) > 0 && first == "" {
		return idlerUnprovedSummary(forgeRoot, unproved), true
	}
	return first, false
}

// idlerProjectRead runs the check over one project and says what it read: a
// reading that proves the tool read this forge, a reading that did not, or
// nothing to prove because no session is up on the project, which is not a
// fault.
func idlerProjectRead(scripts, forgeRoot, project string) (line string, proved, unproved bool) {
	command := "role_health.sh " + project + " --forge-root " + forgeRoot
	out, _ := run(filepath.Join(scripts, "role_health.sh"), project, "--forge-root", forgeRoot)
	notRunning, readings := parseIdlerReport(out)
	if notRunning || allSessionsGone(readings) {
		return "", false, true
	}
	if len(readings) == 0 {
		return fmt.Sprintf("self-check failed idler check: ran %q, which read no role on a project that holds one", command), false, false
	}
	line, proved = idlerEvidence(project, command, out)
	return line, proved, false
}

// idlerUnprovedSummary is what a pass says when every project it read could not
// prove a pane: nothing to fail on, and the projects it read and the pane it
// could not prove named, so a later run can prove it.
func idlerUnprovedSummary(forgeRoot string, unproved []string) string {
	command := "role_health.sh " + unproved[0] + " --forge-root " + forgeRoot
	return fmt.Sprintf("self-check idler check: ran %q, read the projects %s and could not prove a pane: nothing is up on %s",
		command, strings.Join(unproved, ", "), panePath(unproved[0]))
}

// allSessionsGone reports whether every reading names a pane whose session has
// ended. A role the check read and judged is a read, whatever it judged.
func allSessionsGone(readings []reading) bool {
	for _, found := range readings {
		if found.Verdict != "session-gone" {
			return false
		}
	}
	return len(readings) > 0
}

// panePath is where a project's panes live: the socket the check reads them on.
// A forge with no socket at all says so rather than naming nothing.
func panePath(project string) string {
	path := filepath.Join(project, ".swarmforge", "tmux-socket")
	data, err := os.ReadFile(path)
	if err != nil {
		return path + " (no socket written yet)"
	}
	return strings.TrimSpace(string(data))
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

// doorbellSelfCheck runs the doorbell over the forge and reads the line it opens
// with: which pane, of which role, it looked at. A doorbell that cannot name the
// pane it would ring has nothing to say about a request that went missing.
func doorbellSelfCheck(scripts, forgeRoot string) (string, bool) {
	command := "doorbell.sh " + forgeRoot
	marker := "read the pane"
	out, _ := run(filepath.Join(scripts, "doorbell.sh"), forgeRoot)
	if !strings.Contains(out, marker) {
		return fmt.Sprintf("self-check failed doorbell: ran %q and never said what it read (%q)", command, marker), false
	}
	return fmt.Sprintf("self-check doorbell: ran %q, and the doorbell said: %s", command, doorbellRead(out)), true
}

// doorbellRead is what the doorbell said it read, which is what its first line
// carries.
func doorbellRead(out string) string {
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "doorbell: ") {
			return strings.TrimPrefix(line, "doorbell: ")
		}
	}
	return "nothing"
}
