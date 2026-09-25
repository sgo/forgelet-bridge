package kit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// notAForgeRoot is what the tools the kit ships answer when the forge root
// holds no state of its own: the words a forge that has never been started
// makes them say.
const notAForgeRoot = "Not a forge root (no .swarmforge/roles.tsv): "

// composedButNotStarted is a forge that has been composed and never started:
// its own composition and the project it serves are there, and nothing a start
// would write down is - no roles file, no board, no inbox and no socket. That
// is the shape a Forgelet forge arrives in, where the kit is installed before
// anything is started.
func composedButNotStarted(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "projects", "forgelet-bridge", "swarmforge"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "swarmforge", "roles"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// start is what starting a forge writes down at the forge root, which is what
// the tools read a forge root by.
func start(t *testing.T, root string) {
	t.Helper()
	roles := "lieutenant\tmaster\t" + root + "\tfixture-lieutenant\tLieutenant\tcodex\ttask\tforward-only\n"
	if err := os.MkdirAll(filepath.Join(root, ".swarmforge"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".swarmforge", "roles.tsv"), []byte(roles), 0o644); err != nil {
		t.Fatal(err)
	}
}

// toolThatReadsTheForge is one of the tools whose self-check reads the forge's
// own store: the script it is made of, and the names the report uses for the
// tool and for the thing it could not read.
type toolThatReadsTheForge struct {
	script  string
	check   func(scripts, forgeRoot string) (string, bool)
	subject string
	thing   string
	exit    string
}

// toolsThatReadTheForge are those tools: the gate reads the proposal store and
// the doorbell reads the pane.
func toolsThatReadTheForge() []toolThatReadsTheForge {
	return []toolThatReadsTheForge{
		{script: "route_card.sh", check: gateSelfCheck, subject: "route gate", thing: "proposal store", exit: "1"},
		{script: "doorbell.sh", check: doorbellSelfCheck, subject: "doorbell", thing: "pane", exit: "2"},
	}
}

// saysItCouldNotReadTheForge writes one tool's stand-in: a script that says the
// forge root holds no state of its own and refuses, the way a tool says it could
// not read the thing it works on. The doorbell's stand-in still answers the ring
// the self-check asks it for: the pass is what cannot read this forge, and the
// ring is the tool's own words, which are provable there too.
func saysItCouldNotReadTheForge(t *testing.T, scripts, script, exit, root string) {
	t.Helper()
	ring := ""
	if script == "doorbell.sh" {
		ring = "case \"$1\" in\n  print-ring) echo \"" + gateClauseWords + " " + answerClauseWords + "\";;\nesac\n"
	}
	writeScript(t, filepath.Join(scripts, script),
		"#!/bin/sh\n"+ring+"echo '"+notAForgeRoot+root+"' >&2\nexit "+exit+"\n")
}

func TestSelfCheckReadsWhatItCouldNotReadOnAForgeThatHasNotBeenStarted(t *testing.T) {
	for _, tool := range toolsThatReadTheForge() {
		t.Run(tool.subject, func(t *testing.T) {
			root := composedButNotStarted(t)
			scripts := t.TempDir()
			saysItCouldNotReadTheForge(t, scripts, tool.script, tool.exit, root)

			line, ok := tool.check(scripts, root)

			if !ok {
				t.Fatalf("a forge that has not been started holds no %s to read, so it installs: %s", tool.thing, line)
			}
			if !strings.Contains(line, "the "+tool.thing+" it could not read") {
				t.Errorf("the self-check does not say which %s it could not read:\n%s", tool.thing, line)
			}
			if !strings.Contains(line, "has not been started") {
				t.Errorf("the self-check does not say why the %s was not there:\n%s", tool.thing, line)
			}
		})
	}
}

func TestSelfCheckFailsAStartedForgeWhoseToolReadNothing(t *testing.T) {
	for _, tool := range toolsThatReadTheForge() {
		t.Run(tool.subject, func(t *testing.T) {
			root := composedButNotStarted(t)
			start(t, root)
			scripts := t.TempDir()
			// A tool that reads nothing says nothing at all on its own pass, on a
			// forge that has been started: that is the silence the self-check
			// exists to catch. The doorbell's ring still answers the clause, so
			// what fails here is the pass rather than the words.
			silent := "#!/bin/sh\nexit 0\n"
			if tool.script == "doorbell.sh" {
				silent = fixtureDoorbell(true, "exit 0")
			}
			writeScript(t, filepath.Join(scripts, tool.script), silent)

			line, ok := tool.check(scripts, root)

			if ok {
				t.Fatalf("a %s that read nothing passed on a forge that has been started: %s", tool.subject, line)
			}
			if !strings.Contains(line, "self-check failed "+tool.subject) {
				t.Errorf("the failure does not name the %s:\n%s", tool.subject, line)
			}
		})
	}
}

func TestIdlerSelfCheckReadsAProjectThatHasNotBeenStarted(t *testing.T) {
	root := composedButNotStarted(t)
	project := filepath.Join(root, "projects", "forgelet-bridge")
	scripts := t.TempDir()
	writeScript(t, filepath.Join(scripts, "role_health.sh"), "#!/bin/sh\necho 'Give a project root: role_health.sh <project-root> [--ask]' >&2\nexit 2\n")

	line, ok := idlerSelfCheck(scripts, root)

	if !ok {
		t.Fatalf("a forge nothing has been started in has no pane to prove, and installs: %s", line)
	}
	for _, want := range []string{
		"read the project " + project,
		"which has not been started",
		"could not prove a pane",
		filepath.Join(project, ".swarmforge", "tmux-socket"),
	} {
		if !strings.Contains(line, want) {
			t.Errorf("the self-check does not carry %q:\n%s", want, line)
		}
	}
}

func TestIdlerSelfCheckFailsAStartedForgeWhoseProjectItCannotRead(t *testing.T) {
	root := composedButNotStarted(t)
	start(t, root)
	scripts := t.TempDir()
	writeScript(t, filepath.Join(scripts, "role_health.sh"), "#!/bin/sh\nexit 0\n")

	line, ok := idlerSelfCheck(scripts, root)

	if ok {
		t.Fatalf("a started forge whose project holds no roles file passed: %s", line)
	}
	if !strings.Contains(line, "read no roles, no board and no inbox") {
		t.Errorf("the failure does not say the check read nothing:\n%s", line)
	}
}

func TestIdlerSelfCheckNamesEveryProjectOfAForgeThatHasNotBeenStarted(t *testing.T) {
	root := composedButNotStarted(t)
	if err := os.MkdirAll(filepath.Join(root, "projects", "forgelet-app", "swarmforge"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A directory the forge did not compose is not a project it serves.
	if err := os.MkdirAll(filepath.Join(root, "projects", "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	scripts := t.TempDir()
	writeScript(t, filepath.Join(scripts, "role_health.sh"), "#!/bin/sh\nexit 2\n")

	line, ok := idlerSelfCheck(scripts, root)

	if !ok {
		t.Fatalf("a forge nothing has been started in installs: %s", line)
	}
	for _, want := range []string{
		"read the projects " + filepath.Join(root, "projects", "forgelet-app") + ", " + filepath.Join(root, "projects", "forgelet-bridge"),
		"which have not been started",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("the self-check does not carry %q:\n%s", want, line)
		}
	}
	if strings.Contains(line, "notes") {
		t.Errorf("the self-check named a directory the forge never composed:\n%s", line)
	}
}

func TestIdlerSelfCheckFailsAForgeThatHoldsNoProjectToRead(t *testing.T) {
	// Nothing has been started and there is no project either: that is a wrong
	// path or a wrong forge, not a forge that is merely unstarted.
	root := t.TempDir()
	scripts := t.TempDir()
	writeScript(t, filepath.Join(scripts, "role_health.sh"), "#!/bin/sh\nexit 0\n")

	line, ok := idlerSelfCheck(scripts, root)

	if ok {
		t.Fatalf("a forge with no project at all passed: %s", line)
	}
	if !strings.Contains(line, "read no roles, no board and no inbox") {
		t.Errorf("the failure does not say the check read nothing:\n%s", line)
	}
}

func TestInstallInstallsIntoAForgeThatHasNotBeenStarted(t *testing.T) {
	root := composedButNotStarted(t)
	kitDir := t.TempDir()
	writeScript(t, filepath.Join(kitDir, "route_card.sh"), "#!/bin/sh\necho '"+notAForgeRoot+root+"' >&2\nexit 1\n")
	writeScript(t, filepath.Join(kitDir, "route_card.bb"), "# the route gate\n")
	writeScript(t, filepath.Join(kitDir, "role_health.sh"), "#!/bin/sh\necho 'Give a project root: role_health.sh <project-root> [--ask]' >&2\nexit 2\n")
	writeScript(t, filepath.Join(kitDir, "role_health.bb"), "# the idler check\n")
	writeScript(t, filepath.Join(kitDir, "stall_watch.sh"), "#!/bin/sh\necho '<string>"+root+"</string>'\n")
	writeScript(t, filepath.Join(kitDir, "forge_schedule.sh"), "#!/bin/sh\necho 'the forge schedule ran'\n")
	writeScript(t, filepath.Join(kitDir, "doorbell.sh"),
		fixtureDoorbell(true, "echo '"+notAForgeRoot+root+"' >&2; exit 2"))
	writeScript(t, filepath.Join(kitDir, "doorbell.bb"), "# the doorbell\n")

	report, err := Install(root, kitDir)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	if report.Failed() {
		t.Errorf("a forge that has been composed and never started failed the install:\n%s", report)
	}
	for _, want := range []string{
		"installed the route gate, the idler check, the stall watch and the doorbell",
		"the proposal store it could not read",
		"which has not been started",
		"could not prove a pane",
		"the pane it could not read",
		"it rang an approval and found the gate clause",
		"it rang a clarification and found the answer clause",
	} {
		if !strings.Contains(report.String(), want) {
			t.Errorf("the report does not carry %q:\n%s", want, report)
		}
	}
}

// TestDoorbellSelfCheckProvesTheClauseOnAForgeThatHasNotBeenStarted is the other
// side of the pass over an unstarted forge: the pane is the forge's and stands
// down, and the ring's clauses are the tool's own words, which are provable
// there - a kit whose doorbell lost them ships quietly nowhere.
func TestDoorbellSelfCheckProvesTheClauseOnAForgeThatHasNotBeenStarted(t *testing.T) {
	root := composedButNotStarted(t)
	scripts := t.TempDir()
	pass := "echo '" + notAForgeRoot + root + "' >&2; exit 2"
	writeScript(t, filepath.Join(scripts, "doorbell.sh"), fixtureDoorbell(true, pass))

	line, ok := doorbellSelfCheck(scripts, root)

	if !ok {
		t.Fatalf("a doorbell whose ring carries both clauses failed on a forge that has not been started: %s", line)
	}
	for _, want := range []string{
		"the pane it could not read",
		"it rang an approval and found the gate clause",
		"it rang a clarification and found the answer clause",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("the self-check does not carry %q:\n%s", want, line)
		}
	}
}

// TestDoorbellSelfCheckRefusesAKitThatLostTheClauseOnAForgeThatHasNotBeenStarted
// is the failure the clause check exists for, on the forge a composition
// installs into: the kit's own words are read whatever the forge has to prove,
// so a doorbell that lost them fails its own install here too.
func TestDoorbellSelfCheckRefusesAKitThatLostTheClauseOnAForgeThatHasNotBeenStarted(t *testing.T) {
	root := composedButNotStarted(t)
	scripts := t.TempDir()
	pass := "echo '" + notAForgeRoot + root + "' >&2; exit 2"
	writeScript(t, filepath.Join(scripts, "doorbell.sh"), fixtureDoorbell(false, pass))

	line, ok := doorbellSelfCheck(scripts, root)

	if ok {
		t.Fatalf("a doorbell whose ring lost the clause passed on a forge that has not been started: %s", line)
	}
	if !strings.Contains(line, "could not find the clause") {
		t.Errorf("the self-check does not say the clause was not there:\n%s", line)
	}
}
