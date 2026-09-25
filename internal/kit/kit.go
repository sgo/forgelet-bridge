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
	"path/filepath"
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
		// The watch's cadence travels with it: the agent the installer writes
		// runs the forge schedule, which makes the watch's pass and then the
		// doorbell's, so the script it names has to be installed beside it.
		{Subject: "stall watch", Files: []string{"stall_watch.sh", "forge_schedule.sh"}, Check: watchSelfCheck},
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

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-25T17:18:34+02:00","module_hash":"0e1b753b5a57a453cb937e37d92ba6b6756eced310ee33097dcfdb51dcc0e905","functions":[{"id":"func/Tools","name":"Tools","line":33,"end_line":43,"hash":"dea286c970ee3563323571271885f9f002494507d4dff689337f60c2bd5a1881"},{"id":"func/Report.String","name":"Report.String","line":52,"end_line":52,"hash":"863e712f9451c5c9557839b1f63d51b90cbca25f344ed8be78f4bb1db698109d"},{"id":"func/Report.Failed","name":"Report.Failed","line":55,"end_line":55,"hash":"6571c27262847925ba3f9d9228894e28a4e8f64e756fd26f5a9fe2dedeca6219"},{"id":"func/Report.line","name":"Report.line","line":57,"end_line":57,"hash":"f139de20d2b5e0c47987f48ff54a63a8fd8abfef4ee9f0f72709daeca5b4fb77"},{"id":"func/Install","name":"Install","line":61,"end_line":87,"hash":"2cb72b6421c7585c406a13ad8f40ee7a5c817a5da9ad37f7b029b7caf937629c"},{"id":"func/installTools","name":"installTools","line":91,"end_line":104,"hash":"e44299b0e64ad015e26023a978f63d25a36ce7e77b1a03c3e53cd8b0d2e5b814"},{"id":"func/installFile","name":"installFile","line":115,"end_line":138,"hash":"9b58bb5480e63c7f14ffb5eb27be651e9641336c60044d95ff8678d3f6cdf188"},{"id":"func/installAgent","name":"installAgent","line":143,"end_line":161,"hash":"53718d85a8f28a22c9ee2214d32751afaf26d55ea889f20db7265d8610516640"},{"id":"func/policy","name":"policy","line":164,"end_line":172,"hash":"0c82487d751f8aa51afd36223b39b7b5e7dc093624363312b499929f3be2eb83"},{"id":"func/firstLine","name":"firstLine","line":175,"end_line":182,"hash":"55405d13f50d5c0b8bed4113c52ff9d3314748ddade7e4dc38afc668638a0cae"}]}
// mutate4go-manifest-end
