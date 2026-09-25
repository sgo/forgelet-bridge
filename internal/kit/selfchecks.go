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
// nothing at all. A forge that has been composed and never started has no
// proposal store to read, and installing into one is the order most people
// choose: there the report names the store it could not read, so a later run
// can prove it.
func gateSelfCheck(scripts, forgeRoot string) (string, bool) {
	command := "route_card.sh commit " + selfCheckProposal + " --forge-root " + forgeRoot
	marker := "No proposal record for " + selfCheckProposal
	out, _ := run(filepath.Join(scripts, "route_card.sh"), "commit", selfCheckProposal, "--forge-root", forgeRoot)
	if !strings.Contains(out, marker) {
		if !started(forgeRoot) {
			return fmt.Sprintf("self-check route gate: ran %q, and the proposal store it could not read: the gate said %q, so the forge has not been started",
				command, answered(out)), true
		}
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
		return idlerNothingStarted(scripts, forgeRoot)
	}
	return idlerProjectsRead(scripts, forgeRoot, projects)
}

// idlerNothingStarted is what the check says about a forge that serves no
// started project. A forge that has been composed and never started is the far
// end of the same check: nothing has been started there, so there is no pane to
// prove and the project it read is the reading the report carries. Reading
// nothing at all where no project exists is still a wrong path or a wrong
// forge, and fails.
func idlerNothingStarted(scripts, forgeRoot string) (string, bool) {
	if !started(forgeRoot) {
		if line, ok := idlerUnstartedRead(scripts, forgeRoot); ok {
			return line, true
		}
	}
	return fmt.Sprintf("self-check failed idler check: %s holds no project with a roles file, so it read no roles, no board and no inbox",
		filepath.Join(forgeRoot, "projects")), false
}

// idlerProjectsRead reads every started project the forge serves and says what
// it read: the first reading that proves the tool read the forge, the first that
// did not when none proved it, or every project whose pane could not be proved,
// which is not a fault - nothing is up there, and a later run can prove it.
func idlerProjectsRead(scripts, forgeRoot string, projects []string) (string, bool) {
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

// idlerUnstartedRead is what the check says about a forge that has been
// composed and never started: the projects the forge holds are the reading, the
// pane it could not prove is named, and there is nothing to fail on, so a later
// run can prove what this one could not. A forge that holds no project at all
// is not this: that is a wrong path or a wrong forge.
func idlerUnstartedRead(scripts, forgeRoot string) (string, bool) {
	unstarted := unstartedProjects(forgeRoot)
	if len(unstarted) == 0 {
		return "", false
	}
	project := unstarted[0]
	command := "role_health.sh " + project + " --forge-root " + forgeRoot
	out, _ := run(filepath.Join(scripts, "role_health.sh"), project, "--forge-root", forgeRoot)
	return fmt.Sprintf("self-check idler check: ran %q, read %s, which %s not been started, and could not prove a pane: the check said %q and nothing is up on %s",
		command, projectsRead(unstarted), plural(len(unstarted), "has", "have"), answered(out), panePath(project)), true
}

// projectsRead names the projects a reading covered, the way the report says
// them: one project, or the projects it read when the forge holds several.
func projectsRead(projects []string) string {
	if len(projects) == 1 {
		return "the project " + projects[0]
	}
	return "the projects " + strings.Join(projects, ", ")
}

// plural is the wording one reading takes for one thing or for several.
func plural(count int, one, many string) string {
	if count == 1 {
		return one
	}
	return many
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
// pane it would ring has nothing to say about a request that went missing. A
// forge that has been composed and never started holds no pane to read, which is
// the shape a Forgelet forge arrives in: there the report names the pane it
// could not read, so a later run can prove it.
func doorbellSelfCheck(scripts, forgeRoot string) (string, bool) {
	command := "doorbell.sh " + forgeRoot
	marker := "read the pane"
	out, _ := run(filepath.Join(scripts, "doorbell.sh"), forgeRoot)
	if !strings.Contains(out, marker) {
		if !started(forgeRoot) {
			return fmt.Sprintf("self-check doorbell: ran %q, and the pane it could not read: the doorbell said %q, so the forge has not been started and no socket is written at %s",
				command, answered(out), filepath.Join(forgeRoot, ".swarmforge", "tmux-socket")), true
		}
		return fmt.Sprintf("self-check failed doorbell: ran %q and never said what it read (%q)", command, marker), false
	}
	report := fmt.Sprintf("self-check doorbell: ran %q, and the doorbell said: %s", command, doorbellRead(out))
	// The self-check runs the doorbell's pass, so installing into a forge with a
	// request that never arrived rings it. Those rings are right - such a request
	// really was never delivered - and installing is a moment when somebody is
	// reading, so the report says which requests it repaired on the way in.
	for _, body := range doorbellRang(out) {
		report += fmt.Sprintf("; it rang the chat request %s that had never been delivered", body)
	}
	return report, true
}

// answered is a tool's own words, as a self-check quotes them: the first line it
// said, or "nothing" when it said nothing at all. The report carries the tool's
// words rather than the installer's guess at them.
func answered(out string) string {
	if strings.TrimSpace(out) == "" {
		return "nothing"
	}
	return firstLine(out)
}

// doorbellRang is the requests one pass rang, as the doorbell named them.
func doorbellRang(out string) []string {
	var rang []string
	for _, line := range strings.Split(out, "\n") {
		const prefix = `the chat request "`
		if !strings.HasPrefix(line, prefix) || !strings.Contains(line, "was never delivered and rung") {
			continue
		}
		body, _, found := strings.Cut(strings.TrimPrefix(line, prefix), `"`)
		if found && body != "" {
			rang = append(rang, `"`+body+`"`)
		}
	}
	return rang
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

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-24T19:20:34+02:00","module_hash":"c91c2eb9988248d5149ac78bebc8d2c6c1fa7a28633c79ebc95c0a59ec005558","functions":[{"id":"func/selfChecks","name":"selfChecks","line":13,"end_line":19,"hash":"3d50ddfbf8fa4d850cb4bc024e9212c0ed9d9da9038346672db495b0ad253e60"},{"id":"func/gateSelfCheck","name":"gateSelfCheck","line":28,"end_line":36,"hash":"1e619c76e525fca762a817046990f848b0debe2aa7fbc215f84bde379a0a3976"},{"id":"func/idlerSelfCheck","name":"idlerSelfCheck","line":47,"end_line":74,"hash":"6bcb44297ad17825e70054d9dc5d2abdde9a2de785106e1abae1dbcb596313b9"},{"id":"func/idlerProjectRead","name":"idlerProjectRead","line":80,"end_line":92,"hash":"4795cb1006101abb3045316ee06ab7564e1c3c7421c22af76c56f86ae27889f2"},{"id":"func/idlerUnprovedSummary","name":"idlerUnprovedSummary","line":97,"end_line":101,"hash":"1dfeb139a94f5b979af1b697dbd84f46a3f156fde6d99094a26619757ceffd7a"},{"id":"func/allSessionsGone","name":"allSessionsGone","line":105,"end_line":112,"hash":"77d7bb47ccd76367f8c088752a0c84dd09c4e03229e24cfaf348e2bc3e652066"},{"id":"func/panePath","name":"panePath","line":116,"end_line":123,"hash":"7422b4755c59d8535f04bb2d8ae2b2e58e0b580543ef0c14aecf48b1fdca62ff"},{"id":"func/watchSelfCheck","name":"watchSelfCheck","line":128,"end_line":136,"hash":"fcc24096c9e693c1344e2f84fa45870c3e4a78594ead6186d5a2fc3b32103762"},{"id":"func/doorbellSelfCheck","name":"doorbellSelfCheck","line":141,"end_line":157,"hash":"e56d8375cca5072525a670ce76edb207b452d573359cc0fe87222c90f15b5a8a"},{"id":"func/doorbellRang","name":"doorbellRang","line":160,"end_line":173,"hash":"f3a49277529cc51598dba6042ef9222e25299cfa5945ed1fff3ce04835dd7732"},{"id":"func/doorbellRead","name":"doorbellRead","line":177,"end_line":184,"hash":"441093ea34681efcb6c72418a22d812498bb637aac4d855bd71342f3242eb018"}]}
// mutate4go-manifest-end
