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
// report uses for it. The self-check travels with the tool, so a tool the kit
// ships is a tool the installer checks.
type Tool struct {
	Subject string
	Files   []string
	Check   func(scripts, forgeRoot string) (string, bool)
}

// Tools is the kit, in the order the report names it.
func Tools() []Tool {
	return []Tool{
		{Subject: "route gate", Files: []string{"route_card.sh", "route_card.bb"}, Check: gateSelfCheck},
		{Subject: "idler check", Files: []string{"role_health.sh", "role_health.bb"}, Check: idlerSelfCheck},
		{Subject: "stall watch", Files: []string{"stall_watch.sh"}, Check: watchSelfCheck},
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
	forgeRoot, err := forgeDir(forgeRoot)
	if err != nil {
		return report, err
	}
	scripts := filepath.Join(forgeRoot, "swarmforge", "scripts")
	if err := os.MkdirAll(scripts, 0o755); err != nil {
		return report, err
	}
	changed, err := installTools(&report, kitDir, scripts)
	if err != nil {
		return report, err
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

// forgeDir is the forge root as a full path that is a directory. Everything the
// report names, and every tool the installer runs, reads the forge by its full
// path: a tool that resolves the root itself would otherwise name a different
// place in the report than the one the installer worked on.
func forgeDir(forgeRoot string) (string, error) {
	absolute, err := filepath.Abs(forgeRoot)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", fmt.Errorf("the forge %s is not there: %w", absolute, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("the forge %s is not a directory", absolute)
	}
	return absolute, nil
}

// installTools copies every file of the kit into the forge's scripts, and says
// whether any of them changed.
func installTools(report *Report, kitDir, scripts string) (bool, error) {
	changed := false
	for _, tool := range Tools() {
		for _, file := range tool.Files {
			outcome, err := installFile(tool.Subject, filepath.Join(kitDir, file), filepath.Join(scripts, file))
			if err != nil {
				return changed, err
			}
			report.line(outcome.line)
			changed = changed || outcome.changed
		}
	}
	return changed, nil
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
		command := "role_health.sh " + project + " --forge-root " + forgeRoot
		out, _ := run(filepath.Join(scripts, "role_health.sh"), project, "--forge-root", forgeRoot)
		notRunning, readings := parseIdlerReport(out)
		if notRunning || allSessionsGone(readings) {
			unproved = append(unproved, project)
			continue
		}
		if len(readings) == 0 {
			if first == "" {
				first = fmt.Sprintf("self-check failed idler check: ran %q, which read no role on a project that holds one", command)
			}
			continue
		}
		line, ok := idlerEvidence(project, command, out)
		if ok {
			return line, true
		}
		if first == "" {
			first = line
		}
	}
	if len(unproved) > 0 {
		command := "role_health.sh " + unproved[0] + " --forge-root " + forgeRoot
		return fmt.Sprintf("self-check idler check: ran %q, read the projects %s and could not prove a pane: nothing is up on %s",
			command, strings.Join(unproved, ", "), panePath(unproved[0])), true
	}
	return first, false
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

// idlerEvidence is what one run of the check read: the pane it looked at, the
// marker it found, the board and the inbox. An empty board and an empty inbox
// are reads the tool made - a forge between cards is a forge the check can
// read - so only reading nothing at all fails: no pane, no role, or no live
// session behind the one it names.
func idlerEvidence(project, command, output string) (string, bool) {
	notRunning, readings := parseIdlerReport(output)
	if notRunning {
		return fmt.Sprintf("self-check failed idler check: ran %q, the forge is not running", command), false
	}
	if len(readings) == 0 {
		return fmt.Sprintf("self-check failed idler check: ran %q, which read no role", command), false
	}
	var first string
	var quiet string
	for _, reading := range readings {
		evidence := fmt.Sprintf("read pane %s, found marker %s", paneOf(project, reading.Role), reading.Verdict)
		if reading.Verdict == "session-gone" {
			if first == "" {
				first = fmt.Sprintf("self-check failed idler check: ran %q, %s, and there is no live pane behind it", command, evidence)
			}
			continue
		}
		line := fmt.Sprintf("self-check idler check: ran %q, %s, board %s, inbox %s",
			command, evidence, boardRead(reading), inboxRead(reading))
		// A role with something in flight is the reading that says the most, so
		// it is the one the report carries when there is one.
		if boardRead(reading) != "empty" || inboxRead(reading) != "empty" {
			return line, true
		}
		if quiet == "" {
			quiet = line
		}
	}
	if quiet != "" {
		// Every role read was empty, and an empty board and inbox are reads the
		// tool made: a forge between cards is a forge the check can read.
		return quiet, true
	}
	return first, false
}

// boardRead is what the check found in the card's lane. Nothing there is a
// reading too: the tool looked and the board held no card.
func boardRead(found reading) string {
	if found.Card == "" || found.Card == "-" {
		return "empty"
	}
	return found.Card
}

// inboxRead is what the check found in the role's own inbox: the mail waiting
// there, or nothing, which is still something the tool read.
func inboxRead(found reading) string {
	if found.MailCount == 0 {
		return "empty"
	}
	return found.Mail
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
		readings = append(readings, readingFrom(columns))
	}
	return notRunning, readings
}

// readingFrom is one role's line of the check's report: the role, its verdict,
// the card the board showed, and the mail.
func readingFrom(columns []string) reading {
	found := reading{Role: columns[0], Verdict: columns[1], Card: "-"}
	if len(columns) > 2 && !strings.Contains(columns[2], "=") {
		found.Card = columns[2]
	}
	found.Mail, found.MailCount = mailOf(columns)
	return found
}

// mailOf is the mail a report line names: a new one first, then the one still
// in process.
func mailOf(columns []string) (string, int) {
	mail, count := "", 0
	for _, column := range columns {
		if got, ok := countOf(column, "new="); ok && got > 0 {
			mail, count = "new="+strconv.Itoa(got), got
			continue
		}
		if got, ok := countOf(column, "in_process="); ok && got > 0 && count == 0 {
			mail, count = "in_process="+strconv.Itoa(got), got
		}
	}
	return mail, count
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

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-24T12:22:36+02:00","module_hash":"16e9238c6aa8677bd858a3dacc8b041a10c87c91b364451f902568cfb7a0c4a5","functions":[{"id":"func/Tools","name":"Tools","line":36,"end_line":42,"hash":"e04178bc850c9e20648be62d7e984d0c2498989ee72be4daf19c90453d4620b0"},{"id":"func/Report.String","name":"Report.String","line":51,"end_line":51,"hash":"863e712f9451c5c9557839b1f63d51b90cbca25f344ed8be78f4bb1db698109d"},{"id":"func/Report.Failed","name":"Report.Failed","line":54,"end_line":54,"hash":"6571c27262847925ba3f9d9228894e28a4e8f64e756fd26f5a9fe2dedeca6219"},{"id":"func/Report.line","name":"Report.line","line":56,"end_line":56,"hash":"f139de20d2b5e0c47987f48ff54a63a8fd8abfef4ee9f0f72709daeca5b4fb77"},{"id":"func/Install","name":"Install","line":60,"end_line":86,"hash":"05c2c63d18bef9c127cf28f7ca77fdb0a31cc540dbdbe0280288b31f4041992b"},{"id":"func/forgeDir","name":"forgeDir","line":92,"end_line":105,"hash":"3201b616212bcf7f5a30837dec2b783d9659270662cb1db0f7ac25a6ad3d2f64"},{"id":"func/installTools","name":"installTools","line":109,"end_line":126,"hash":"0ebf91f9d8633f0aa3b93105c96a1c70a74d0f2439f11e1bc217fac5e1204295"},{"id":"func/installFile","name":"installFile","line":137,"end_line":160,"hash":"9b58bb5480e63c7f14ffb5eb27be651e9641336c60044d95ff8678d3f6cdf188"},{"id":"func/installAgent","name":"installAgent","line":165,"end_line":183,"hash":"53718d85a8f28a22c9ee2214d32751afaf26d55ea889f20db7265d8610516640"},{"id":"func/selfChecks","name":"selfChecks","line":190,"end_line":204,"hash":"f4d3c650c36069877cc65791a4b6bf3320f0ea9132f6a4a3adcd1f89641777f5"},{"id":"func/gateSelfCheck","name":"gateSelfCheck","line":213,"end_line":221,"hash":"1e619c76e525fca762a817046990f848b0debe2aa7fbc215f84bde379a0a3976"},{"id":"func/idlerSelfCheck","name":"idlerSelfCheck","line":226,"end_line":244,"hash":"871972403f17e874d6ac62b5d04963ad3c1ab2dbf89056a2c520655422fefb87"},{"id":"func/idlerEvidence","name":"idlerEvidence","line":251,"end_line":286,"hash":"31d695bf3ff9e9a7e21ddb3dab9fa0c392bb80407fa84e5642112fc781fd8b37"},{"id":"func/boardRead","name":"boardRead","line":290,"end_line":295,"hash":"5de4d9e8c53b8b5620a333ff5d5c606cd01ad165738abe7b9af687c6c1ee8fca"},{"id":"func/inboxRead","name":"inboxRead","line":299,"end_line":304,"hash":"c8dd4e11c9f804a29376794afd8d110ff50d4d332aadbbd82a56cd2bbe2b99f0"},{"id":"func/parseIdlerReport","name":"parseIdlerReport","line":318,"end_line":333,"hash":"d1317252a05b64fbc31b286c504434c8d4c16a6096260f00597f947e6c8408ef"},{"id":"func/readingFrom","name":"readingFrom","line":337,"end_line":344,"hash":"02e45455e68fbaff4be2de1912ed5ad454512d5e11df952143b056157fe0c7e9"},{"id":"func/mailOf","name":"mailOf","line":348,"end_line":360,"hash":"0ec62c2244e0e2727d0d4e98dbc61f786a1e014bdeb01f39a01966b2c3a2f93c"},{"id":"func/countOf","name":"countOf","line":363,"end_line":369,"hash":"0d2f4363a3d949886a923f10779ae39233aa333ebce3c1e23437cf0cbc090ed5"},{"id":"func/watchSelfCheck","name":"watchSelfCheck","line":374,"end_line":382,"hash":"fcc24096c9e693c1344e2f84fa45870c3e4a78594ead6186d5a2fc3b32103762"},{"id":"func/policy","name":"policy","line":385,"end_line":393,"hash":"0c82487d751f8aa51afd36223b39b7b5e7dc093624363312b499929f3be2eb83"},{"id":"func/firstLine","name":"firstLine","line":396,"end_line":403,"hash":"55405d13f50d5c0b8bed4113c52ff9d3314748ddade7e4dc38afc668638a0cae"},{"id":"func/projects","name":"projects","line":406,"end_line":424,"hash":"7c4517b40f84cd057b6fe50d69cca68cf19ec7baeada4860a4ebae2e489315b1"},{"id":"func/paneOf","name":"paneOf","line":428,"end_line":440,"hash":"dbb60542d466b85f6528d132ec38702d9715958b37e117f38eefb9c0cb950043"},{"id":"func/run","name":"run","line":445,"end_line":448,"hash":"b0ba526783b3c7b75bbd753ee0cc95ab7a15ecd7af00ccd08a10f9bd95c098ae"}]}
// mutate4go-manifest-end
