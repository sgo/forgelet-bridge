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

// The two requests the doorbell's clause self-check rings, and the clauses their
// rings have to carry: an approval the operator forwarded is the operator's
// decision, and a clarification an agent is blocked on is the operator's answer.
// The bridge writes the first line of each notification, so both shapes are
// knowable rather than guessed at, and the ring - the one moment a session that
// never read its prompt meets the request - is where the words have to be.
const (
	selfCheckApproval      = "Approval for refund-card in forgelet-bridge"
	selfCheckClarification = "Clarification for forgelet-bridge from coder"
	gateClause             = "the gate is the operator's"
	gateRefusal            = "Do not approve unless the operator says to"
	answerClause           = "the answer is the operator's to give"
	answerRefusal          = "Do not answer it unless the operator says to"
)

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

// idlerProjectsRead reads every started project the forge serves in name order
// and says what it read: the first reading that proves the tool read the forge,
// or, when none proved it, every project whose pane could not be proved, which
// is not a fault - nothing is up there, and a later run can prove it.
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
	// A project the tool could not read at all is a fault when no project proved
	// a reading, whatever quiet projects sit beside it: a pane that is not up is
	// the only reading excused. Nothing to read and nothing unread is not a
	// shape a forge has, and it does not pass as one.
	if first != "" {
		return first, false
	}
	if len(unproved) == 0 {
		return "", false
	}
	return idlerUnprovedSummary(forgeRoot, unproved), true
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
// could not read, so a later run can prove it. The ring's own words are what a
// session meets its gate through, so a pass that proves the tool ran is not
// enough on its own: the check rings the two shapes the bridge writes and reads
// back the clause each one carries. That clause is the tool's own words rather
// than anything of the forge's, so it is proved whatever else the forge could
// prove - a kit whose doorbell lost it fails its own install on a forge that has
// never been started as surely as on one that has.
func doorbellSelfCheck(scripts, forgeRoot string) (string, bool) {
	command := "doorbell.sh " + forgeRoot
	marker := "read the pane"
	clauses, ok := doorbellClauseRead(scripts)
	if !ok {
		return clauses, false
	}
	out, _ := run(filepath.Join(scripts, "doorbell.sh"), forgeRoot)
	if !strings.Contains(out, marker) {
		if !started(forgeRoot) {
			return fmt.Sprintf("self-check doorbell: ran %q, and the pane it could not read: the doorbell said %q, so the forge has not been started and no socket is written at %s; %s",
				command, answered(out), filepath.Join(forgeRoot, ".swarmforge", "tmux-socket"), clauses), true
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
	return report + "; " + clauses, true
}

// clause is one gate the ring carries: the request the bridge writes it for, the
// name the report gives the clause, and the words the ring has to say.
type clause struct {
	subject string
	name    string
	request string
	words   []string
}

// clauses are the two notifications the bridge writes that carry a gate, in the
// order the report names them.
func clauses() []clause {
	return []clause{
		{subject: "an approval", name: "the gate clause", request: selfCheckApproval, words: []string{gateClause, gateRefusal}},
		{subject: "a clarification", name: "the answer clause", request: selfCheckClarification, words: []string{answerClause, answerRefusal}},
	}
}

// doorbellClauseRead asks the installed doorbell for the ring it would type for
// each of the two requests the bridge writes, and says whether the clause is in
// them. The ring's own words are what the check reads rather than a description
// of them, which is what makes a kit whose doorbell lost the clause fail its own
// install rather than shipping quietly.
func doorbellClauseRead(scripts string) (string, bool) {
	var found []string
	for _, gate := range clauses() {
		command := "doorbell.sh print-ring " + gate.request
		out, _ := run(filepath.Join(scripts, "doorbell.sh"), "print-ring", gate.request)
		for _, word := range gate.words {
			if !strings.Contains(out, word) {
				return fmt.Sprintf("self-check failed doorbell: ran %q and could not find the clause: the ring it would type for %s never says %q",
					command, gate.subject, word), false
			}
		}
		found = append(found, fmt.Sprintf("it rang %s and found %s (%s)",
			gate.subject, gate.name, quotedWords(gate.words)))
	}
	return strings.Join(found, ", and "), true
}

// quotedWords is the words one clause is made of, as the report quotes them.
func quotedWords(words []string) string {
	var quoted []string
	for _, word := range words {
		quoted = append(quoted, fmt.Sprintf("%q", word))
	}
	return strings.Join(quoted, ", ")
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
// {"version":1,"tested_at":"2026-09-25T22:56:13+02:00","module_hash":"8954a006a2ad7ee37a9b9e286bbf3a560fb63d03e8b66c681935692af73492f2","functions":[{"id":"func/selfChecks","name":"selfChecks","line":13,"end_line":19,"hash":"3d50ddfbf8fa4d850cb4bc024e9212c0ed9d9da9038346672db495b0ad253e60"},{"id":"func/gateSelfCheck","name":"gateSelfCheck","line":46,"end_line":58,"hash":"4e0abca726ae840a692d39b48eba1e3a40805c310db4e30814659550593e4355"},{"id":"func/idlerSelfCheck","name":"idlerSelfCheck","line":69,"end_line":75,"hash":"5ded6d084da208e4911ab7cb2c54234f8bf73c70952483d430214c527fea87c0"},{"id":"func/idlerNothingStarted","name":"idlerNothingStarted","line":83,"end_line":91,"hash":"4fd54e5463585af41d1f7f40400c09382722e54c5392f987c8bb9ebc7ed2c38e"},{"id":"func/idlerProjectsRead","name":"idlerProjectsRead","line":97,"end_line":124,"hash":"e3bd834d36c03e8503270b07595c324dccdc74bd22b691ec4f40571d1d4d4333"},{"id":"func/idlerUnstartedRead","name":"idlerUnstartedRead","line":131,"end_line":141,"hash":"435b92f0953e24ee237a55d2ee5952c48d83a5cbbd16cc2491e0d327d65a2bce"},{"id":"func/projectsRead","name":"projectsRead","line":145,"end_line":150,"hash":"b6240d6e5a516fd14781cd28d8fc28bba0485bb45da4c5c44829aa163ed40341"},{"id":"func/plural","name":"plural","line":153,"end_line":158,"hash":"9e69b96fbd931b64063f7d8152a7700ae8f043bce14b2f7121c4df62270cbb65"},{"id":"func/idlerProjectRead","name":"idlerProjectRead","line":164,"end_line":176,"hash":"4795cb1006101abb3045316ee06ab7564e1c3c7421c22af76c56f86ae27889f2"},{"id":"func/idlerUnprovedSummary","name":"idlerUnprovedSummary","line":181,"end_line":185,"hash":"1dfeb139a94f5b979af1b697dbd84f46a3f156fde6d99094a26619757ceffd7a"},{"id":"func/allSessionsGone","name":"allSessionsGone","line":189,"end_line":196,"hash":"77d7bb47ccd76367f8c088752a0c84dd09c4e03229e24cfaf348e2bc3e652066"},{"id":"func/panePath","name":"panePath","line":200,"end_line":207,"hash":"7422b4755c59d8535f04bb2d8ae2b2e58e0b580543ef0c14aecf48b1fdca62ff"},{"id":"func/watchSelfCheck","name":"watchSelfCheck","line":212,"end_line":220,"hash":"fcc24096c9e693c1344e2f84fa45870c3e4a78594ead6186d5a2fc3b32103762"},{"id":"func/doorbellSelfCheck","name":"doorbellSelfCheck","line":234,"end_line":258,"hash":"a6bd6df4b182f22ea824eabe035c10632535bb22c733445c115670dd8169f62b"},{"id":"func/clauses","name":"clauses","line":271,"end_line":276,"hash":"e24e77394a1e9c370203d46cdd6c8f6f8a38659b14a76520016cbf4ea20c9538"},{"id":"func/doorbellClauseRead","name":"doorbellClauseRead","line":283,"end_line":298,"hash":"9c300f8957c5dca2ee04c6890d98d1cad75bc32bf756f4500b657339ef024a76"},{"id":"func/quotedWords","name":"quotedWords","line":301,"end_line":307,"hash":"5d012175d6df61335f9cd394236819e442b6510c65ab1bb04fe8c224d56bf8b8"},{"id":"func/answered","name":"answered","line":312,"end_line":317,"hash":"314e8b572f0c8ab4da8cefb1f9522e0e297304a15877714ebf603e629b130934"},{"id":"func/doorbellRang","name":"doorbellRang","line":320,"end_line":333,"hash":"f3a49277529cc51598dba6042ef9222e25299cfa5945ed1fff3ce04835dd7732"},{"id":"func/doorbellRead","name":"doorbellRead","line":337,"end_line":344,"hash":"441093ea34681efcb6c72418a22d812498bb637aac4d855bd71342f3242eb018"}]}
// mutate4go-manifest-end
