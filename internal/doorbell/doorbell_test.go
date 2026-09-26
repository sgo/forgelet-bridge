// Package doorbell holds the tests that run the doorbell tool - the script the
// kit ships - against a tmux that keeps one pane's own state: the turns the pane
// has taken, and the text its composer is still holding.
//
// The pane's own terminal is the acceptance suite's business. What is pinned
// here is the doorbell's reading of it, which is the difference between a ring
// that landed and a ring that is sitting in the composer with nothing having
// seen it, and the ledger that difference is written down in.
package doorbell

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// projectRoot is the checkout the tool under test lives in.
func projectRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("the test cannot say where it is")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

// fixture is one forge root, the pane the doorbell rings, and the tmux that
// keeps that pane: what it has taken, and what its composer holds.
type fixture struct {
	t       *testing.T
	root    string
	state   string
	pane    string
	id      string
	body    string
	scripts string
}

const (
	fixturePane = "fixture-pane"
	fixtureID   = "req-1"
	fixtureBody = "Clarification for forgelet-bridge from coder\nQuestion:\nis the build green?\nAnswer:\nyes, it is"
)

func newFixture(t *testing.T) *fixture {
	t.Helper()
	dir := t.TempDir()
	f := &fixture{
		t:       t,
		root:    filepath.Join(dir, "forge"),
		state:   filepath.Join(dir, "pane"),
		pane:    fixturePane,
		id:      fixtureID,
		body:    fixtureBody,
		scripts: filepath.Join(dir, "bin"),
	}
	for _, where := range []string{filepath.Join(f.root, ".swarmforge", "dashboard", "requests", "pending"), f.state, f.scripts} {
		if err := os.MkdirAll(where, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	roles := "coder\tmaster\t" + f.root + "\t" + fixturePane + "\tCoder\tcodex\ttask\tforward-only\n"
	f.write(filepath.Join(f.root, ".swarmforge", "roles.tsv"), roles)
	f.write(filepath.Join(f.root, ".swarmforge", "tmux-socket"), f.state+"\n")
	request := "id: " + fixtureID + "\nstatus: pending\ncreated_at: 2026-09-26T12:00:00Z\n\n" + fixtureBody + "\n"
	f.write(filepath.Join(f.root, ".swarmforge", "dashboard", "requests", "pending", fixtureID+".request"), request)
	f.write(filepath.Join(f.state, "loses-enter"), "no")
	f.write(filepath.Join(f.state, "loses-cursor"), "no")
	f.write(filepath.Join(f.state, "turns"), "")
	f.write(filepath.Join(f.state, "calls"), "")
	f.write(filepath.Join(f.state, "composer"), "")
	f.write(filepath.Join(f.state, "screen"), "> ")
	f.write(filepath.Join(f.state, "cursor"), "0\n")
	if err := os.WriteFile(filepath.Join(f.scripts, "tmux"), []byte(tmuxStub), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.scripts, "idler.sh"), []byte(idlerStub), 0o755); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *fixture) write(path, body string) {
	f.t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

// loses is one answer this fixture's pane does not give, named the way the stub
// tmux's own state file names it: the Enter a ring sends - which a terminal too
// busy taking the paste leaves in the composer - or where its cursor sits, which
// a session that has gone, or a tmux too old to say, cannot answer at all. A
// pane that does not answer has not said yes.
func (f *fixture) loses(answer string) {
	f.t.Helper()
	f.write(filepath.Join(f.state, "loses-"+answer), "yes")
}

// run runs the doorbell the kit ships against the fixture forge.
func (f *fixture) run() string {
	f.t.Helper()
	command := exec.Command(filepath.Join(projectRoot(f.t), "swarmforge", "scripts", "doorbell.sh"),
		f.root, "--idler", filepath.Join(f.scripts, "idler.sh"))
	command.Env = append(os.Environ(), "PATH="+f.scripts+":"+os.Getenv("PATH"))
	out, err := command.CombinedOutput()
	if err != nil {
		f.t.Fatalf("the doorbell did not finish: %v\n%s", err, out)
	}
	return string(out)
}

// calls is everything the doorbell asked the pane's tmux to do.
func (f *fixture) calls() string {
	f.t.Helper()
	data, err := os.ReadFile(filepath.Join(f.state, "calls"))
	if err != nil {
		f.t.Fatal(err)
	}
	return string(data)
}

// ledger is the doorbell's own record of what it rang and what it left owed.
func (f *fixture) ledger() string {
	f.t.Helper()
	data, err := os.ReadFile(filepath.Join(f.root, ".swarmforge", "doorbell.edn"))
	if err != nil {
		f.t.Fatalf("the doorbell kept no ledger: %v", err)
	}
	return string(data)
}

// TestTheRingGoesInAsOnePasteAndOneEnter pins the mechanism the ring is: the
// whole request in as one paste, which a body of many lines cannot submit on its
// own newlines, and then one Enter - a pane still taking the paste loses an
// Enter that follows it, and the ring that stays in the composer is the failure
// a second Enter would hide.
func TestTheRingGoesInAsOnePasteAndOneEnter(t *testing.T) {
	f := newFixture(t)
	f.loses("enter")

	f.run()

	calls := f.calls()
	for _, want := range []string{
		"set-buffer -b",
		"paste-buffer -d -p -b",
		"send-keys -t " + fixturePane + " C-m",
	} {
		if !strings.Contains(calls, want) {
			t.Errorf("the ring does not go in as %q:\n%s", want, calls)
		}
	}
	if strings.Contains(calls, "send-keys -t "+fixturePane+" -l") {
		t.Errorf("the ring is typed rather than pasted:\n%s", calls)
	}
	if enters := strings.Count(calls, "send-keys -t "+fixturePane+" C-m"); enters != 1 {
		t.Errorf("the ring sends %d Enters, want one:\n%s", enters, calls)
	}
	if strings.Contains(calls, "C-j") {
		t.Errorf("the ring sends a line feed after the Enter:\n%s", calls)
	}
}

// TestARingThePaneStillHoldsIsNotADelivery is the failure this tool exists to
// prevent: a ring whose Enter was lost sits in the composer, and the pass must
// not write the request down as delivered or rung.
func TestARingThePaneStillHoldsIsNotADelivery(t *testing.T) {
	f := newFixture(t)
	f.loses("enter")

	out := f.run()

	if !strings.Contains(out, `the chat request "`+fixtureBody+`" was rung and the ring did not land`) {
		t.Errorf("the doorbell does not say the ring did not land:\n%s", out)
	}
	ledger := f.ledger()
	if !strings.Contains(ledger, `:owed ["`+fixtureID+`"]`) {
		t.Errorf("the ledger does not leave the request owed:\n%s", ledger)
	}
	if strings.Contains(ledger, `:delivered ["`+fixtureID+`"]`) {
		t.Errorf("the ledger writes a ring the pane is still holding down as delivered:\n%s", ledger)
	}
	if strings.Contains(ledger, `:rung ["`+fixtureID+`"]`) {
		t.Errorf("the ledger writes a ring that did not land down as rung:\n%s", ledger)
	}
}

// TestARingThatLandedIsWrittenDownAsRung is the other half: a pane that took the
// turn is a request the session has seen, and the ledger says so.
func TestARingThatLandedIsWrittenDownAsRung(t *testing.T) {
	f := newFixture(t)

	out := f.run()

	if !strings.Contains(out, `the chat request "`+fixtureBody+`" was never delivered and rung into `+fixturePane) {
		t.Errorf("the doorbell does not say the ring landed:\n%s", out)
	}
	ledger := f.ledger()
	if !strings.Contains(ledger, `:rung ["`+fixtureID+`"]`) {
		t.Errorf("the ledger does not say the request was rung:\n%s", ledger)
	}
	if strings.Contains(ledger, `:owed ["`+fixtureID+`"]`) {
		t.Errorf("the ledger leaves a request the pane took owed:\n%s", ledger)
	}
}

// TestAPaneThatCannotSayWhereItsCursorIsIsNotALanding is the other silence a
// ring can meet: a pane whose composer cannot be read at all - the session has
// gone, or its tmux is too old to say where the cursor sits - has not proved
// that it took the ring, so the request stays owed rather than being written
// down as a delivery. Taking no answer for a yes is the same false delivery the
// composer check exists to stop.
func TestAPaneThatCannotSayWhereItsCursorIsIsNotALanding(t *testing.T) {
	f := newFixture(t)
	f.loses("cursor")

	out := f.run()

	if !strings.Contains(out, `the chat request "`+fixtureBody+`" was rung and the ring did not land`) {
		t.Errorf("the doorbell does not say the ring did not land:\n%s", out)
	}
	ledger := f.ledger()
	if !strings.Contains(ledger, `:owed ["`+fixtureID+`"]`) {
		t.Errorf("the ledger does not leave the request owed:\n%s", ledger)
	}
	if strings.Contains(ledger, `:rung ["`+fixtureID+`"]`) || strings.Contains(ledger, `:delivered ["`+fixtureID+`"]`) {
		t.Errorf("the ledger writes a ring a pane never proved it took down as delivered or rung:\n%s", ledger)
	}
}

// TestAPassOverARingStillInTheComposerRingsNothingMore pins the reading the next
// pass has of a ring the pane never took: the composer still holds the doorbell's
// own words, so its copy of the request proves nothing - the request stays owed,
// and ringing it again would only pile the same text into the same composer.
func TestAPassOverARingStillInTheComposerRingsNothingMore(t *testing.T) {
	f := newFixture(t)
	f.loses("enter")
	f.run()
	afterFirst := strings.Count(f.calls(), "paste-buffer")

	out := f.run()

	if !strings.Contains(out, `the chat request "`+fixtureBody+`" is still owed`) {
		t.Errorf("the second pass does not say the request is still owed:\n%s", out)
	}
	if pasted := strings.Count(f.calls(), "paste-buffer") - afterFirst; pasted != 0 {
		t.Errorf("the second pass typed the ring into the composer again (%d more pastes):\n%s", pasted, f.calls())
	}
	ledger := f.ledger()
	if !strings.Contains(ledger, `:owed ["`+fixtureID+`"]`) || strings.Contains(ledger, `:rung ["`+fixtureID+`"]`) {
		t.Errorf("the second pass does not leave the request owed and unrung:\n%s", ledger)
	}
}

// tmuxStub is a tmux that keeps one pane's own state: the turns it has taken,
// the text its composer is holding, and the line its cursor sits on, which is
// where a terminal draws the composer. It is given the state directory in the
// place of the socket.
const tmuxStub = `#!/bin/sh
set -eu
state="$2"; shift 2
cmd="$1"; shift

render() {
  { cat "$state/turns"; printf '> '; cat "$state/composer" 2>/dev/null || true; } > "$state/screen"
  awk 'END { print NR - 1 }' "$state/screen" > "$state/cursor"
}

case "$cmd" in
  display-message)
    if [ "$(cat "$state/loses-cursor")" = "yes" ]; then exit 1; fi
    cat "$state/cursor" ;;
  capture-pane) cat "$state/screen" ;;
  set-buffer)
    printf '%s\n' "$cmd $*" >> "$state/calls"
    while [ "$#" -gt 0 ]; do
      case "$1" in
        -b) shift 2 ;;
        *) printf '%s' "$1" > "$state/buffer"; shift ;;
      esac
    done
    ;;
  paste-buffer)
    printf '%s\n' "$cmd $*" >> "$state/calls"
    cat "$state/buffer" >> "$state/composer"
    render
    ;;
  send-keys)
    printf '%s\n' "$cmd $*" >> "$state/calls"
    if [ "$(cat "$state/loses-enter")" = "yes" ]; then exit 0; fi
    { printf '> '; cat "$state/composer"; printf '\n'; } >> "$state/turns"
    : > "$state/composer"
    render
    ;;
esac
`

// idlerStub is the check the doorbell asks whether the role is mid-turn: the
// pane is quiet, so a ring has a window to go in through.
const idlerStub = `#!/bin/sh
echo "coder       idle-nothing-assigned    -    new=0 in_process=0 quiet=4m tool=codex"
`
