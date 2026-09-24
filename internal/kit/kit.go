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
	"strings"
)

// kitName is how the report names the tools together.
const kitName = "the route gate, the idler check, the stall watch and the doorbell"

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
		{Subject: "doorbell", Files: []string{"doorbell.sh", "doorbell.bb"}, Check: doorbellSelfCheck},
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
// {"version":1,"tested_at":"2026-09-24T14:19:24+02:00","module_hash":"f663d219df574f6ab04e3396b7636233aa95a695f7e307ad63a76884a1450254","functions":[{"id":"func/Tools","name":"Tools","line":36,"end_line":42,"hash":"e04178bc850c9e20648be62d7e984d0c2498989ee72be4daf19c90453d4620b0"},{"id":"func/Report.String","name":"Report.String","line":51,"end_line":51,"hash":"863e712f9451c5c9557839b1f63d51b90cbca25f344ed8be78f4bb1db698109d"},{"id":"func/Report.Failed","name":"Report.Failed","line":54,"end_line":54,"hash":"6571c27262847925ba3f9d9228894e28a4e8f64e756fd26f5a9fe2dedeca6219"},{"id":"func/Report.line","name":"Report.line","line":56,"end_line":56,"hash":"f139de20d2b5e0c47987f48ff54a63a8fd8abfef4ee9f0f72709daeca5b4fb77"},{"id":"func/Install","name":"Install","line":60,"end_line":86,"hash":"2cb72b6421c7585c406a13ad8f40ee7a5c817a5da9ad37f7b029b7caf937629c"},{"id":"func/forgeDir","name":"forgeDir","line":92,"end_line":105,"hash":"3201b616212bcf7f5a30837dec2b783d9659270662cb1db0f7ac25a6ad3d2f64"},{"id":"func/installTools","name":"installTools","line":109,"end_line":122,"hash":"e44299b0e64ad015e26023a978f63d25a36ce7e77b1a03c3e53cd8b0d2e5b814"},{"id":"func/installFile","name":"installFile","line":133,"end_line":156,"hash":"9b58bb5480e63c7f14ffb5eb27be651e9641336c60044d95ff8678d3f6cdf188"},{"id":"func/installAgent","name":"installAgent","line":161,"end_line":179,"hash":"53718d85a8f28a22c9ee2214d32751afaf26d55ea889f20db7265d8610516640"},{"id":"func/selfChecks","name":"selfChecks","line":184,"end_line":190,"hash":"3d50ddfbf8fa4d850cb4bc024e9212c0ed9d9da9038346672db495b0ad253e60"},{"id":"func/gateSelfCheck","name":"gateSelfCheck","line":199,"end_line":207,"hash":"1e619c76e525fca762a817046990f848b0debe2aa7fbc215f84bde379a0a3976"},{"id":"func/idlerSelfCheck","name":"idlerSelfCheck","line":218,"end_line":245,"hash":"6bcb44297ad17825e70054d9dc5d2abdde9a2de785106e1abae1dbcb596313b9"},{"id":"func/idlerProjectRead","name":"idlerProjectRead","line":251,"end_line":263,"hash":"4795cb1006101abb3045316ee06ab7564e1c3c7421c22af76c56f86ae27889f2"},{"id":"func/idlerUnprovedSummary","name":"idlerUnprovedSummary","line":268,"end_line":272,"hash":"1dfeb139a94f5b979af1b697dbd84f46a3f156fde6d99094a26619757ceffd7a"},{"id":"func/allSessionsGone","name":"allSessionsGone","line":276,"end_line":283,"hash":"77d7bb47ccd76367f8c088752a0c84dd09c4e03229e24cfaf348e2bc3e652066"},{"id":"func/panePath","name":"panePath","line":287,"end_line":294,"hash":"7422b4755c59d8535f04bb2d8ae2b2e58e0b580543ef0c14aecf48b1fdca62ff"},{"id":"func/idlerEvidence","name":"idlerEvidence","line":301,"end_line":336,"hash":"31d695bf3ff9e9a7e21ddb3dab9fa0c392bb80407fa84e5642112fc781fd8b37"},{"id":"func/boardRead","name":"boardRead","line":340,"end_line":345,"hash":"5de4d9e8c53b8b5620a333ff5d5c606cd01ad165738abe7b9af687c6c1ee8fca"},{"id":"func/inboxRead","name":"inboxRead","line":349,"end_line":354,"hash":"c8dd4e11c9f804a29376794afd8d110ff50d4d332aadbbd82a56cd2bbe2b99f0"},{"id":"func/parseIdlerReport","name":"parseIdlerReport","line":368,"end_line":383,"hash":"d1317252a05b64fbc31b286c504434c8d4c16a6096260f00597f947e6c8408ef"},{"id":"func/readingFrom","name":"readingFrom","line":387,"end_line":394,"hash":"02e45455e68fbaff4be2de1912ed5ad454512d5e11df952143b056157fe0c7e9"},{"id":"func/mailOf","name":"mailOf","line":398,"end_line":410,"hash":"0ec62c2244e0e2727d0d4e98dbc61f786a1e014bdeb01f39a01966b2c3a2f93c"},{"id":"func/countOf","name":"countOf","line":413,"end_line":419,"hash":"0d2f4363a3d949886a923f10779ae39233aa333ebce3c1e23437cf0cbc090ed5"},{"id":"func/watchSelfCheck","name":"watchSelfCheck","line":424,"end_line":432,"hash":"fcc24096c9e693c1344e2f84fa45870c3e4a78594ead6186d5a2fc3b32103762"},{"id":"func/policy","name":"policy","line":435,"end_line":443,"hash":"0c82487d751f8aa51afd36223b39b7b5e7dc093624363312b499929f3be2eb83"},{"id":"func/firstLine","name":"firstLine","line":446,"end_line":453,"hash":"55405d13f50d5c0b8bed4113c52ff9d3314748ddade7e4dc38afc668638a0cae"},{"id":"func/projects","name":"projects","line":456,"end_line":474,"hash":"7c4517b40f84cd057b6fe50d69cca68cf19ec7baeada4860a4ebae2e489315b1"},{"id":"func/paneOf","name":"paneOf","line":478,"end_line":490,"hash":"dbb60542d466b85f6528d132ec38702d9715958b37e117f38eefb9c0cb950043"},{"id":"func/run","name":"run","line":495,"end_line":498,"hash":"b0ba526783b3c7b75bbd753ee0cc95ab7a15ecd7af00ccd08a10f9bd95c098ae"}]}
// mutate4go-manifest-end
