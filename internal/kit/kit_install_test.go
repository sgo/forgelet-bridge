package kit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeScript writes an executable fixture file.
func writeScript(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// fixtureForge is a forge root that has been started - its own roles file is
// there, which is what the tools the kit ships read a forge root by - with one
// project the tools can read, and the lieutenant prompt that holds its gate
// policy.
func fixtureForge(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	forgeRoles := "lieutenant\tmaster\t" + root + "\tfixture-lieutenant\tLieutenant\tcodex\ttask\tforward-only\n"
	if err := os.MkdirAll(filepath.Join(root, ".swarmforge"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".swarmforge", "roles.tsv"), []byte(forgeRoles), 0o644); err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(root, "projects", "forgelet-bridge")
	if err := os.MkdirAll(filepath.Join(project, ".swarmforge"), 0o755); err != nil {
		t.Fatal(err)
	}
	roles := "master\tmaster\t" + project + "\tfixture-master\tMaster\tcodex\ttask\tforward-only\n" +
		"coder\tcoder\t" + filepath.Join(project, "worktrees", "coder") + "\tfixture-coder\tCoder\tcodex\ttask\tforward-only\n"
	if err := os.WriteFile(filepath.Join(project, ".swarmforge", "roles.tsv"), []byte(roles), 0o644); err != nil {
		t.Fatal(err)
	}
	prompt := filepath.Join(root, "swarmforge", "roles", "lieutenant.prompt")
	if err := os.MkdirAll(filepath.Dir(prompt), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(prompt, []byte("Ask the operator before a card is created.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// fixtureKit is a kit of small tools that answer the way the self-checks read
// them, each naming the forge it is run against.
func fixtureKit(t *testing.T, forgeRoot string) string {
	t.Helper()
	dir := t.TempDir()
	writeScript(t, filepath.Join(dir, "route_card.sh"), "#!/bin/sh\necho 'No proposal record for install-kit-self-check'\n")
	writeScript(t, filepath.Join(dir, "route_card.bb"), "# the route gate\n")
	writeScript(t, filepath.Join(dir, "role_health.sh"), "#!/bin/sh\necho 'coder idle-holding-card refund-card new=0 in_process=1'\n")
	writeScript(t, filepath.Join(dir, "role_health.bb"), "# the idler check\n")
	writeScript(t, filepath.Join(dir, "stall_watch.sh"), "#!/bin/sh\necho '<string>"+forgeRoot+"</string>'\n")
	writeScript(t, filepath.Join(dir, "forge_schedule.sh"), "#!/bin/sh\necho 'the forge schedule ran'\n")
	writeScript(t, filepath.Join(dir, "doorbell.sh"), fixtureDoorbell(true, readsThePane))
	writeScript(t, filepath.Join(dir, "doorbell.bb"), "# the doorbell\n")
	// The kit ships its .bb files beside the wrappers, and the wrappers are what
	// run: the .bb files are not executable here, as they are not in the kit the
	// project ships.
	for _, file := range []string{"route_card.bb", "role_health.bb", "doorbell.bb"} {
		if err := os.Chmod(filepath.Join(dir, file), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// The clauses the ring carries for the two notifications the bridge writes, as
// the kit's own doorbell says them: what a kit whose doorbell lost the clause
// does not say.
const (
	gateClauseWords   = "the gate is the operator's: Do not approve unless the operator says to."
	answerClauseWords = "the answer is the operator's to give: Do not answer the clarification request unless the operator says to."
)

// readsThePane is the line the doorbell's own pass opens with on a forge a
// session is up on, which is the reading the self-check looks for.
const readsThePane = "echo 'doorbell: read the pane fixture-master of the role master'"

// fixtureDoorbell is a doorbell that answers the way the self-check reads it: the
// line its own pass opens with, and the ring it would type for a request body -
// with the clauses its two gates carry, or without them, which is what a kit
// whose doorbell lost the clause looks like.
func fixtureDoorbell(withClauses bool, pass string) string {
	gate, answer := "nothing", "nothing"
	if withClauses {
		gate, answer = gateClauseWords, answerClauseWords
	}
	return "#!/bin/sh\n" +
		"case \"$1\" in\n" +
		"  print-ring)\n" +
		"    case \"$2\" in\n" +
		"      Approval*) echo \"" + gate + "\";;\n" +
		"      Clarification*) echo \"" + answer + "\";;\n" +
		"    esac\n" +
		"    ;;\n" +
		"  *) " + pass + ";;\n" +
		"esac\n"
}

func TestInstallCopiesTheKitAndSelfChecksIt(t *testing.T) {
	root := fixtureForge(t)

	report, err := Install(root, fixtureKit(t, root))
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if report.Failed() {
		t.Errorf("a kit whose tools all read the forge was reported as failed:\n%s", report)
	}
	for _, want := range []string{
		"changed route gate",
		"installed the route gate, the idler check, the stall watch and the doorbell",
		"self-check route gate: ran",
		"self-check idler check: ran",
		"self-check stall watch: ran",
		"self-check doorbell: ran",
		"wrote the stall watch's agent",
		"left alone loading the stall watch's agent",
		"left the gate policy in the lieutenant prompt alone",
	} {
		if !strings.Contains(report.String(), want) {
			t.Errorf("the report does not carry %q:\n%s", want, report)
		}
	}
	scripts := filepath.Join(root, "swarmforge", "scripts")
	for file, mode := range map[string]os.FileMode{
		"route_card.sh":  0o755,
		"route_card.bb":  0o644,
		"stall_watch.sh": 0o755,
	} {
		info, err := os.Stat(filepath.Join(scripts, file))
		if err != nil {
			t.Fatalf("%s was not installed: %v", file, err)
		}
		if info.Mode().Perm() != mode {
			t.Errorf("%s mode = %v, want %v", file, info.Mode().Perm(), mode)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".swarmforge", "stall-watch.plist")); err != nil {
		t.Errorf("the agent was not written: %v", err)
	}
}

func TestInstallLeavesACurrentKitAlone(t *testing.T) {
	root := fixtureForge(t)
	kitDir := fixtureKit(t, root)
	if _, err := Install(root, kitDir); err != nil {
		t.Fatalf("first Install: %v", err)
	}

	report, err := Install(root, kitDir)
	if err != nil {
		t.Fatalf("second Install: %v", err)
	}
	if report.Failed() {
		t.Errorf("a second install failed:\n%s", report)
	}
	for _, want := range []string{
		"already current route gate",
		"the kit was already current in ",
		"already current the stall watch's agent",
	} {
		if !strings.Contains(report.String(), want) {
			t.Errorf("the second report does not carry %q:\n%s", want, report)
		}
	}
}

func TestInstallReportsAToolThatReadsNothing(t *testing.T) {
	root := fixtureForge(t)
	kitDir := fixtureKit(t, root)
	// A gate that reads nothing says nothing at all.
	writeScript(t, filepath.Join(kitDir, "route_card.sh"), "#!/bin/sh\nexit 0\n")

	report, err := Install(root, kitDir)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !report.Failed() {
		t.Errorf("a gate that read nothing was not reported as failed:\n%s", report)
	}
	if !strings.Contains(report.String(), "self-check failed route gate") {
		t.Errorf("the report does not name the gate that read nothing:\n%s", report)
	}
}

func TestInstallReportsAWatchThatNamesNoForge(t *testing.T) {
	root := fixtureForge(t)
	kitDir := fixtureKit(t, root)
	writeScript(t, filepath.Join(kitDir, "stall_watch.sh"), "#!/bin/sh\necho '<string>/somewhere/else</string>'\n")

	report, err := Install(root, kitDir)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !report.Failed() || !strings.Contains(report.String(), "self-check failed stall watch") {
		t.Errorf("a watch that named another forge was not reported as failed:\n%s", report)
	}
}

func TestInstallRefusesAForgeThatIsNotThere(t *testing.T) {
	root := fixtureForge(t)
	missing := filepath.Join(t.TempDir(), "not-a-forge")

	_, err := Install(missing, fixtureKit(t, root))
	if err == nil || !strings.Contains(err.Error(), "is not there") {
		t.Fatalf("Install(missing) = %v, want the forge reported as not there", err)
	}
}

func TestInstallRefusesAForgeThatIsAFile(t *testing.T) {
	root := fixtureForge(t)
	file := filepath.Join(t.TempDir(), "a-file")
	if err := os.WriteFile(file, []byte("not a forge\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Install(file, fixtureKit(t, root))
	if err == nil || !strings.Contains(err.Error(), "is not a directory") {
		t.Fatalf("Install(a file) = %v, want the forge reported as not a directory", err)
	}
}

func TestIdlerSelfCheckFailsWhenTheForgeServesNoProject(t *testing.T) {
	root := t.TempDir()
	scripts := t.TempDir()
	writeScript(t, filepath.Join(scripts, "role_health.sh"), "#!/bin/sh\nexit 0\n")

	line, ok := idlerSelfCheck(scripts, root)
	if ok || !strings.Contains(line, "read no roles, no board and no inbox") {
		t.Errorf("idlerSelfCheck = (%q, %v), want a failure naming the empty forge", line, ok)
	}
}

func TestInstallFileReportsAMissingKitFile(t *testing.T) {
	_, err := installFile("route gate", filepath.Join(t.TempDir(), "absent.sh"), filepath.Join(t.TempDir(), "route_card.sh"))
	if err == nil || !strings.Contains(err.Error(), "the kit is missing") {
		t.Fatalf("installFile(missing source) = %v, want the missing kit file reported", err)
	}
}

func TestInstallWritesAStaleAgentAgain(t *testing.T) {
	root := fixtureForge(t)
	kitDir := fixtureKit(t, root)
	if _, err := Install(root, kitDir); err != nil {
		t.Fatalf("first Install: %v", err)
	}
	agent := filepath.Join(root, ".swarmforge", "stall-watch.plist")
	if err := os.WriteFile(agent, []byte("an agent from before the watch was installed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Install(root, kitDir)
	if err != nil {
		t.Fatalf("second Install: %v", err)
	}

	if !strings.Contains(report.String(), "wrote the stall watch's agent") {
		t.Errorf("the report does not say the stale agent was written again:\n%s", report)
	}
	want, _ := run(filepath.Join(root, "swarmforge", "scripts", "stall_watch.sh"), "print-agent", root)
	if got, _ := os.ReadFile(agent); string(got) != want {
		t.Errorf("the agent = %q, want what the installed watch prints", got)
	}
}

func TestIdlerSelfCheckFailsWhenNoProjectHasARolesFile(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "projects", "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	scripts := t.TempDir()
	writeScript(t, filepath.Join(scripts, "role_health.sh"), "#!/bin/sh\nexit 0\n")

	line, ok := idlerSelfCheck(scripts, root)
	if ok || !strings.Contains(line, "read no roles, no board and no inbox") {
		t.Errorf("idlerSelfCheck = (%q, %v), want a failure naming the forge with nothing to read", line, ok)
	}
}

func TestIdlerSelfCheckFailsWhenTheCheckReadsNoRole(t *testing.T) {
	root := fixtureForge(t)
	scripts := t.TempDir()
	// A check that prints nothing at all has read nothing: that is the failure
	// the self-check exists for, not a quiet pass.
	writeScript(t, filepath.Join(scripts, "role_health.sh"), "#!/bin/sh\nexit 0\n")

	line, ok := idlerSelfCheck(scripts, root)
	if ok || !strings.Contains(line, "which read no role") {
		t.Errorf("idlerSelfCheck = (%q, %v), want a failure saying the check read no role", line, ok)
	}
}

func TestInstallReportsAWatchThatCannotPrintItsAgent(t *testing.T) {
	root := fixtureForge(t)
	kitDir := fixtureKit(t, root)
	writeScript(t, filepath.Join(kitDir, "stall_watch.sh"), "#!/bin/sh\nexit 1\n")

	if _, err := Install(root, kitDir); err == nil || !strings.Contains(err.Error(), "could not print the agent") {
		t.Fatalf("Install = %v, want the watch that cannot print its agent reported", err)
	}
}

func TestInstallReportsAKitThatIsMissingAFile(t *testing.T) {
	root := fixtureForge(t)
	kitDir := fixtureKit(t, root)
	if err := os.Remove(filepath.Join(kitDir, "stall_watch.sh")); err != nil {
		t.Fatal(err)
	}

	if _, err := Install(root, kitDir); err == nil || !strings.Contains(err.Error(), "the kit is missing") {
		t.Fatalf("Install = %v, want the kit missing a file reported", err)
	}
}

func TestInstallFileReportsATargetItCannotRead(t *testing.T) {
	kitDir := t.TempDir()
	writeScript(t, filepath.Join(kitDir, "route_card.sh"), "#!/bin/sh\n")
	// A directory where the file goes cannot be read as the file it should be,
	// and overwriting it is not the installer's to do.
	scripts := t.TempDir()
	if err := os.MkdirAll(filepath.Join(scripts, "route_card.sh"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := installFile("route gate", filepath.Join(kitDir, "route_card.sh"), filepath.Join(scripts, "route_card.sh")); err == nil {
		t.Fatal("installFile reported success for a target it could not read")
	}
}

func TestInstallFileReportsATargetItCannotWrite(t *testing.T) {
	kitDir := t.TempDir()
	writeScript(t, filepath.Join(kitDir, "route_card.sh"), "#!/bin/sh\nthe kit's own gate\n")
	scripts := t.TempDir()
	target := filepath.Join(scripts, "route_card.sh")
	if err := os.WriteFile(target, []byte("#!/bin/sh\nthe forge's own gate\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(target, 0o644) })

	if _, err := installFile("route gate", filepath.Join(kitDir, "route_card.sh"), target); err == nil {
		t.Fatal("installFile reported success for a target it could not write")
	}
}

// What a tool is run with is the kit's own business: the kit ships the .bb
// beside the wrapper executable, and an installer that guessed the mode from
// the extension would lose that bit on every install.
func TestInstallKeepsTheModeTheKitsOwnCopyCarries(t *testing.T) {
	root := fixtureForge(t)
	kitDir := fixtureKit(t, root)
	wanted := map[string]os.FileMode{
		"route_card.sh":  0o755,
		"route_card.bb":  0o755,
		"role_health.sh": 0o750,
		"role_health.bb": 0o640,
		"doorbell.bb":    0o755,
	}
	for file, mode := range wanted {
		if err := os.Chmod(filepath.Join(kitDir, file), mode); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := Install(root, kitDir); err != nil {
		t.Fatalf("Install: %v", err)
	}

	scripts := filepath.Join(root, "swarmforge", "scripts")
	for file, mode := range wanted {
		info, err := os.Stat(filepath.Join(scripts, file))
		if err != nil {
			t.Fatalf("%s was not installed: %v", file, err)
		}
		if info.Mode().Perm() != mode {
			t.Errorf("%s mode = %v, want the kit's own %v", file, info.Mode().Perm(), mode)
		}
	}
}

// A second install writes nothing, so it cannot take a mode away either.
func TestInstallLeavesTheForgeCopyOfACurrentKitAlone(t *testing.T) {
	root := fixtureForge(t)
	kitDir := fixtureKit(t, root)
	if err := os.Chmod(filepath.Join(kitDir, "doorbell.bb"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(root, kitDir); err != nil {
		t.Fatalf("first Install: %v", err)
	}

	if _, err := Install(root, kitDir); err != nil {
		t.Fatalf("second Install: %v", err)
	}

	info, err := os.Stat(filepath.Join(root, "swarmforge", "scripts", "doorbell.bb"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("the reinstall rewrote the mode to %v, want the kit's own 0755", info.Mode().Perm())
	}
}

func TestDoorbellSelfCheckProvesTheClausesItsRingCarries(t *testing.T) {
	root := fixtureForge(t)
	scripts := t.TempDir()
	writeScript(t, filepath.Join(scripts, "doorbell.sh"), fixtureDoorbell(true, readsThePane))

	line, ok := doorbellSelfCheck(scripts, root)

	if !ok {
		t.Fatalf("a doorbell whose ring carries both clauses passed nothing: %s", line)
	}
	for _, want := range []string{
		"it rang an approval and found the gate clause",
		"Do not approve unless the operator says to",
		"it rang a clarification and found the answer clause",
		"Do not answer the clarification request unless the operator says to",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("the self-check does not carry %q:\n%s", want, line)
		}
	}
}

func TestDoorbellSelfCheckRefusesAKitWhoseDoorbellLostTheClause(t *testing.T) {
	root := fixtureForge(t)
	scripts := t.TempDir()
	writeScript(t, filepath.Join(scripts, "doorbell.sh"), fixtureDoorbell(false, readsThePane))

	line, ok := doorbellSelfCheck(scripts, root)

	if ok {
		t.Fatalf("a doorbell whose ring lost the clause passed its own install: %s", line)
	}
	if !strings.Contains(line, "could not find the clause") {
		t.Errorf("the self-check does not say the clause was not there:\n%s", line)
	}
}

func TestIdlerSelfCheckPassesOnAProjectWithNothingUp(t *testing.T) {
	cases := map[string]string{
		"a forge between sessions":      "forge not-running no role session is up",
		"a project whose session ended": "coder session-gone - new=0 in_process=1",
	}
	for what, output := range cases {
		root := fixtureForge(t)
		scripts := t.TempDir()
		writeScript(t, filepath.Join(scripts, "role_health.sh"), "#!/bin/sh\necho '"+output+"'\n")

		line, ok := idlerSelfCheck(scripts, root)
		if !ok {
			t.Errorf("%s: idlerSelfCheck failed on a project with nothing up: %s", what, line)
		}
		if !strings.Contains(line, "could not prove a pane") || !strings.Contains(line, "nothing is up on") {
			t.Errorf("%s: the self-check does not name what it could not prove:\n%s", what, line)
		}
	}
}

func TestIdlerSelfCheckProvesAReadingThatIsUp(t *testing.T) {
	root := fixtureForge(t)
	scripts := t.TempDir()
	writeScript(t, filepath.Join(scripts, "role_health.sh"), "#!/bin/sh\necho 'coder idle-holding-card refund-card new=0 in_process=1'\n")

	line, ok := idlerSelfCheck(scripts, root)
	if !ok {
		t.Fatalf("idlerSelfCheck failed on a reading with a pane and a card: %s", line)
	}
	if strings.Contains(line, "could not prove a pane") || !strings.Contains(line, "board refund-card") {
		t.Errorf("the self-check does not carry the reading it proved:\n%s", line)
	}
}

// TestIdlerSelfCheckReadsEveryProjectTheForgeServes is the pass over a forge
// with two started projects, one of which the check cannot read at all: the
// project it did read is the reading the report carries, and a project it could
// not read is a fault when no project proved one, whatever quiet projects sit
// beside it.
func TestIdlerSelfCheckReadsEveryProjectTheForgeServes(t *testing.T) {
	const (
		quiet    = "forge not-running no role session is up"
		aReading = "coder idle-holding-card refund-card new=0 in_process=1"
		nothing  = "exit 0"
	)
	cases := []struct {
		what    string
		second  string
		answers string
		wantOK  bool
		want    string
	}{
		{
			what:    "a project the check cannot read beside one between sessions",
			second:  "stopped-project",
			answers: "echo '" + quiet + "'",
			wantOK:  false,
			want:    "which read no role on a project that holds one",
		},
		{
			what:    "a reading beside a project the check cannot read",
			second:  "aa-unreadable",
			answers: "echo '" + aReading + "'",
			wantOK:  true,
			want:    "board refund-card",
		},
	}
	for _, tc := range cases {
		t.Run(tc.what, func(t *testing.T) {
			root := fixtureForge(t)
			servedProject(t, root, tc.second)
			scripts := t.TempDir()
			// The second project reads nothing; every other project answers
			// with what this case is about.
			writeScript(t, filepath.Join(scripts, "role_health.sh"),
				"#!/bin/sh\ncase \"$1\" in\n*"+tc.second+"*) "+nothing+";;\n*) "+tc.answers+";;\nesac\n")

			line, ok := idlerSelfCheck(scripts, root)

			if ok != tc.wantOK {
				t.Fatalf("idlerSelfCheck = %v, want %v:\n%s", ok, tc.wantOK, line)
			}
			if !strings.Contains(line, tc.want) {
				t.Errorf("the self-check does not carry %q:\n%s", tc.want, line)
			}
		})
	}
}

// TestIdlerProjectsReadFailsAPassHandedNoProject pins the shape the caller rules
// out before it gets here: a reading with no project to read is not a reading,
// and it fails rather than naming a pane that was never there.
func TestIdlerProjectsReadFailsAPassHandedNoProject(t *testing.T) {
	line, ok := idlerProjectsRead(t.TempDir(), t.TempDir(), nil)
	if ok || line != "" {
		t.Errorf("idlerProjectsRead(nil) = (%q, %v), want a pass with nothing to read to fail", line, ok)
	}
}

// Every pass proves the read, the one that put the tools there and the one that
// found them already current: a re-run is where a person checks what a forge
// looks like now, so it is the last pass that should go quiet.
func TestInstallProvesTheReadOnEveryPass(t *testing.T) {
	root := fixtureForge(t)
	kitDir := fixtureKit(t, root)

	first, err := Install(root, kitDir)
	if err != nil {
		t.Fatalf("first Install: %v", err)
	}
	if !strings.Contains(first.String(), "self-check idler check") {
		t.Errorf("the install does not carry the self-check it ran:\n%s", first)
	}

	second, err := Install(root, kitDir)
	if err != nil {
		t.Fatalf("second Install: %v", err)
	}
	if !strings.Contains(second.String(), "the kit was already current") {
		t.Errorf("the second install does not say the kit was already current:\n%s", second)
	}
	if !strings.Contains(second.String(), "self-check idler check") {
		t.Errorf("the second install proves nothing about the forge:\n%s", second)
	}
}
