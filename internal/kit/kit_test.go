package kit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureProject is a project of a fixture forge, with the roles file the
// check's report is read against.
func fixtureProject(t *testing.T) string {
	t.Helper()
	project := filepath.Join(t.TempDir(), "projects", "forgelet-bridge")
	if err := os.MkdirAll(filepath.Join(project, ".swarmforge"), 0o755); err != nil {
		t.Fatal(err)
	}
	roles := "master\tmaster\t" + project + "\tfixture-master\tMaster\tcodex\ttask\tforward-only\n" +
		"coder\tcoder\t" + filepath.Join(project, "worktrees", "coder") + "\tfixture-coder\tCoder\tcodex\ttask\tforward-only\n"
	if err := os.WriteFile(filepath.Join(project, ".swarmforge", "roles.tsv"), []byte(roles), 0o644); err != nil {
		t.Fatal(err)
	}
	return project
}

const idlerReportWithACard = "master      idle-nothing-assigned    -                                  new=0 in_process=0 quiet=4m tool=codex\n" +
	"coder       idle-holding-card      refund-card                        new=0 in_process=1 quiet=9m tool=codex\n"

func TestIdlerEvidenceNamesThePaneTheMarkerTheCardAndTheMail(t *testing.T) {
	project := fixtureProject(t)

	line, ok := idlerEvidence(project, "role_health.sh "+project, idlerReportWithACard)

	if !ok {
		t.Fatalf("the check read a live pane, a board row and an inbox, but the self-check failed: %s", line)
	}
	for _, want := range []string{
		"ran \"role_health.sh " + project + "\"",
		"read pane fixture-coder",
		"found marker idle-holding-card",
		"card refund-card",
		"mail in_process=1",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("the self-check line does not carry %q:\n%s", want, line)
		}
	}
}

func TestIdlerEvidenceReportsAForgeThatIsNotRunning(t *testing.T) {
	project := fixtureProject(t)
	report := "forge       not-running              no role session is up (cards waiting: refund-card)\n"

	line, ok := idlerEvidence(project, "role_health.sh "+project, report)

	if ok {
		t.Fatalf("a forge with no sessions was read as a pass: %s", line)
	}
	if !strings.Contains(line, "self-check failed") || !strings.Contains(line, "the forge is not running") {
		t.Errorf("the failure does not say the forge is not running:\n%s", line)
	}
}

func TestIdlerEvidenceFailsOnAMailboxWithNothingInIt(t *testing.T) {
	project := fixtureProject(t)
	report := "coder       assigned-not-taken       refund-card                        new=0 in_process=0 quiet=9m tool=codex\n"

	line, ok := idlerEvidence(project, "role_health.sh "+project, report)

	if ok {
		t.Fatalf("a role with no mail was read as a pass: %s", line)
	}
	if !strings.Contains(line, "no board row or mail to read") {
		t.Errorf("the failure does not name what it could not read:\n%s", line)
	}
}

func TestIdlerEvidenceFailsOnASessionThatIsGone(t *testing.T) {
	project := fixtureProject(t)
	report := "coder       session-gone             refund-card                        new=0 in_process=1 quiet=9m tool=codex\n"

	line, ok := idlerEvidence(project, "role_health.sh "+project, report)

	if ok {
		t.Fatalf("a pane with no session was read as a pass: %s", line)
	}
	if !strings.Contains(line, "no live pane") {
		t.Errorf("the failure does not name the pane it could not read:\n%s", line)
	}
}

func TestInstallFileReplacesWhatTheKitDoesNotShipAndKeepsWhatItDoes(t *testing.T) {
	kitDir := t.TempDir()
	target := t.TempDir()
	source := filepath.Join(kitDir, "route_card.sh")
	if err := os.WriteFile(source, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	installed := filepath.Join(target, "route_card.sh")
	if err := os.WriteFile(installed, []byte("an older gate\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	outcome, err := installFile("route gate", source, installed)
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.changed || !strings.HasPrefix(outcome.line, "changed route gate in ") {
		t.Errorf("outcome = %+v, want the stale file replaced", outcome)
	}
	again, err := installFile("route gate", source, installed)
	if err != nil {
		t.Fatal(err)
	}
	if again.changed || !strings.HasPrefix(again.line, "already current route gate in ") {
		t.Errorf("outcome = %+v, want the installed file already current", again)
	}
	data, err := os.ReadFile(installed)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "#!/bin/sh\n" {
		t.Errorf("installed file = %q, want what the kit ships", data)
	}
}

func TestPolicyNamesTheWordingItLeftWhereItWas(t *testing.T) {
	forgeRoot := t.TempDir()
	prompt := filepath.Join(forgeRoot, "swarmforge", "roles", "lieutenant.prompt")
	if err := os.MkdirAll(filepath.Dir(prompt), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(prompt, []byte("Ask the operator before a card is created.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var report Report

	policy(&report, forgeRoot)

	line := report.String()
	if !strings.Contains(line, "left the gate policy in the lieutenant prompt alone") {
		t.Errorf("the report does not say it left the policy alone:\n%s", line)
	}
	if !strings.Contains(line, "Ask the operator before a card is created.") {
		t.Errorf("the report does not name the policy it found:\n%s", line)
	}
}

func TestPolicySaysWhenTheForgeHasNotWrittenItsGateDown(t *testing.T) {
	var report Report

	policy(&report, t.TempDir())

	if line := report.String(); !strings.Contains(line, "left the gate policy alone") || !strings.Contains(line, "is not there") {
		t.Errorf("the report does not say the prompt is missing:\n%s", line)
	}
}
