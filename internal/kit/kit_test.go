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
		"board refund-card",
		"inbox in_process=1",
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

// A forge between cards is the other end of the same check: an empty board and
// an empty inbox are things the tool read, not things it could not read.
func TestIdlerEvidenceCountsAnEmptyBoardAndInboxAsReads(t *testing.T) {
	project := fixtureProject(t)
	report := "coder       idle-nothing-assigned   -                                  new=0 in_process=0 quiet=9m tool=codex\n"

	line, ok := idlerEvidence(project, "role_health.sh "+project, report)

	if !ok {
		t.Fatalf("a forge between cards has a live pane, an empty board and an empty inbox, so it installs: %s", line)
	}
	for _, want := range []string{"read pane fixture-coder", "board empty", "inbox empty"} {
		if !strings.Contains(line, want) {
			t.Errorf("the self-check line does not carry %q:\n%s", want, line)
		}
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
	cases := map[string]struct {
		prompt string
		want   string
	}{
		"the wording the forge wrote": {"Ask the operator before a card is created.\n", "Ask the operator before a card is created."},
		"a prompt with nothing in it": {"\n   \n", "(written down, but empty)"},
	}
	for what, testCase := range cases {
		forgeRoot := t.TempDir()
		prompt := filepath.Join(forgeRoot, "swarmforge", "roles", "lieutenant.prompt")
		if err := os.MkdirAll(filepath.Dir(prompt), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(prompt, []byte(testCase.prompt), 0o644); err != nil {
			t.Fatal(err)
		}
		var report Report

		policy(&report, forgeRoot)

		line := report.String()
		if !strings.Contains(line, "left the gate policy in the lieutenant prompt alone") {
			t.Errorf("%s: the report does not say it left the policy alone:\n%s", what, line)
		}
		if !strings.Contains(line, testCase.want) {
			t.Errorf("%s: the report does not name %q:\n%s", what, testCase.want, line)
		}
	}
}

func TestPolicySaysWhenTheForgeHasNotWrittenItsGateDown(t *testing.T) {
	var report Report

	policy(&report, t.TempDir())

	if line := report.String(); !strings.Contains(line, "left the gate policy alone") || !strings.Contains(line, "is not there") {
		t.Errorf("the report does not say the prompt is missing:\n%s", line)
	}
}

func TestParseIdlerReportSkipsLinesThatAreNotReadings(t *testing.T) {
	notRunning, readings := parseIdlerReport("forgelet-bridge\n\ncoder idle-holding-card refund-card new=0 in_process=1\n")

	if notRunning {
		t.Error("a forge with a reading was read as not running")
	}
	if len(readings) != 1 || readings[0].Role != "coder" {
		t.Errorf("readings = %+v, want only the role's line", readings)
	}

	// A line with two columns is a reading: the role and the verdict it
	// reached. Dropping it would lose a role the check did report.
	if _, two := parseIdlerReport("coder idle-holding-card\n"); len(two) != 1 || two[0].Verdict != "idle-holding-card" {
		t.Errorf("readings = %+v, want the role and its verdict read from a two-column line", two)
	}
}

func TestReadingFromReadsACardOnlyWhereACardSits(t *testing.T) {
	// A third column that is a count is not a card, and a line with no third
	// column has none either: reading either as a card is the self-check
	// seeing something the check never said.
	counts := readingFrom(strings.Fields("coder idle-holding-card new=2 in_process=0"))
	if counts.Card != "-" {
		t.Errorf("card = %q, want no card read from a count", counts.Card)
	}
	short := readingFrom(strings.Fields("coder idle-nothing-assigned"))
	if short.Card != "-" {
		t.Errorf("card = %q, want no card read from a two-column line", short.Card)
	}
}

func TestMailOfReadsTheMailALineNames(t *testing.T) {
	cases := map[string]struct {
		line string
		mail string
	}{
		"a new handoff":            {"coder idle-holding-card refund-card new=2 in_process=0", "new=2"},
		"a handoff in process":     {"coder idle-holding-card refund-card new=0 in_process=3", "in_process=3"},
		"new wins over in process": {"coder idle-holding-card refund-card new=1 in_process=2", "new=1"},
		"nothing waiting":          {"coder idle-nothing-assigned - new=0 in_process=0", ""},
	}
	for what, testCase := range cases {
		mail, count := mailOf(strings.Fields(testCase.line))
		if mail != testCase.mail || (testCase.mail == "") != (count == 0) {
			t.Errorf("%s: mailOf = (%q, %d), want %q", what, mail, count, testCase.mail)
		}
	}
}

func TestCountOfReadsOnlyItsOwnCount(t *testing.T) {
	if got, ok := countOf("in_process=2", "new="); ok || got != 0 {
		t.Errorf("countOf(in_process=2) as new = (%d, %v), want nothing read", got, ok)
	}
	if _, ok := countOf("new=two", "new="); ok {
		t.Error("countOf read a count that is not a number")
	}
	if got, ok := countOf("new=2", "new="); !ok || got != 2 {
		t.Errorf("countOf(new=2) = (%d, %v), want 2", got, ok)
	}
}

func TestPaneOfReadsTheRoleRowAndNothingElse(t *testing.T) {
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".swarmforge"), 0o755); err != nil {
		t.Fatal(err)
	}
	// The way a forge writes its roles file: the role and the lane it works in
	// are two fields, and the lane is not always the role's own name - this
	// forge's specifier works in the master lane. A row too short to name a
	// pane names none, and no row is read past the columns it has.
	roles := "specifier\tmaster\t" + project + "\tfixture-specifier\tSpecifier\tcodex\ttask\tforward-only\n" +
		"master\tmaster\tfixture-master\n" +
		"coder\tcoder\t" + project + "\tfixture-coder\n"
	if err := os.WriteFile(filepath.Join(project, ".swarmforge", "roles.tsv"), []byte(roles), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := paneOf(project, "specifier"); got != "fixture-specifier" {
		t.Errorf("paneOf(specifier) = %q, want the pane its row records, not the lane it works in", got)
	}
	if got := paneOf(project, "master"); got != "-" {
		t.Errorf("paneOf(master) = %q, want nothing: no role row is named master", got)
	}
	if got := paneOf(project, "coder"); got != "fixture-coder" {
		t.Errorf("paneOf(coder) = %q, want the pane its four-column row records", got)
	}
}

func TestPaneOfSaysNothingWhenTheProjectHasNoRolesFile(t *testing.T) {
	if got := paneOf(t.TempDir(), "coder"); got != "-" {
		t.Errorf("paneOf = %q, want nothing read from a project with no roles file", got)
	}
}

func TestPanePathNamesTheSocketTheProjectWrites(t *testing.T) {
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".swarmforge"), 0o755); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(project, ".swarmforge", "tmux-socket")
	if err := os.WriteFile(socket, []byte("/tmp/forge-socket\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := panePath(project); got != "/tmp/forge-socket" {
		t.Errorf("panePath = %q, want the socket the project writes", got)
	}
}

func TestDoorbellRangNamesOnlyTheRequestsThePassRang(t *testing.T) {
	out := "doorbell: read the pane fixture-coder of the role coder for /forges/forge-a\n" +
		"the chat request \"already there\" was already delivered from the screen and left alone\n" +
		"the chat request \"never arrived\" was never delivered and rung into fixture-coder\n" +
		"the chat request \"busy one\" was not rung because the role coder was busy\n"

	rang := doorbellRang(out)

	if len(rang) != 1 || rang[0] != `"never arrived"` {
		t.Errorf("doorbellRang = %v, want only the request the pass rang", rang)
	}
}

func TestIdlerEvidenceCarriesAReadingWithSomethingInFlight(t *testing.T) {
	project := fixtureProject(t)
	// One role has an empty lane and an empty inbox, the next holds a card with
	// nothing in its inbox: the card is the reading that says the most, so it is
	// the one the report carries rather than the emptiness read first.
	report := "master      idle-nothing-assigned   -             new=0 in_process=0 quiet=9m tool=codex\n" +
		"coder       idle-holding-card      refund-card   new=0 in_process=0 quiet=9m tool=codex\n"

	line, ok := idlerEvidence(project, "role_health.sh "+project, report)

	if !ok {
		t.Fatalf("a card was read as nothing to read: %s", line)
	}
	if !strings.Contains(line, "board refund-card") {
		t.Errorf("the self-check line does not carry the reading with something in flight:\n%s", line)
	}
}
