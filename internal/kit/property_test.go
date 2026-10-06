//go:build property

package kit

import (
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"testing/quick"
)

// projectState is one directory under a forge's projects/: whether the forge
// composed a project in it, and whether that project has been started. A started
// project was composed first, so Started implies Composed.
type projectState struct {
	Composed bool
	Started  bool
}

// randomForge is a forge of a few project directories, each composed, started,
// both or neither.
func randomForge(rnd *rand.Rand) []projectState {
	states := make([]projectState, rnd.Intn(6))
	for index := range states {
		started := rnd.Intn(2) == 0
		states[index] = projectState{Composed: started || rnd.Intn(2) == 0, Started: started}
	}
	return states
}

// layOutForge writes one forge's projects/ out, and answers the projects a
// reader should find started and the ones it should find composed and unstarted,
// both in name order.
func layOutForge(projectsDir string, states []projectState) (served, held []string, err error) {
	if err := os.RemoveAll(projectsDir); err != nil {
		return nil, nil, err
	}
	if err := os.MkdirAll(projectsDir, 0o755); err != nil {
		return nil, nil, err
	}
	for index, state := range states {
		project := filepath.Join(projectsDir, "p"+strconv.Itoa(index))
		if err := os.MkdirAll(project, 0o755); err != nil {
			return nil, nil, err
		}
		if state.Composed {
			if err := os.MkdirAll(filepath.Join(project, "swarmforge"), 0o755); err != nil {
				return nil, nil, err
			}
		}
		switch {
		case state.Started:
			if err := os.MkdirAll(filepath.Join(project, ".swarmforge"), 0o755); err != nil {
				return nil, nil, err
			}
			if err := os.WriteFile(filepath.Join(project, ".swarmforge", "roles.tsv"), []byte("role\n"), 0o644); err != nil {
				return nil, nil, err
			}
			served = append(served, project)
		case state.Composed:
			held = append(held, project)
		}
	}
	sort.Strings(served)
	sort.Strings(held)
	return served, held, nil
}

// TestPropertyStartedAndUnstartedProjectsSplitTheComposedOnes is the forge's own
// reading: every project the forge composed is either served - it has been
// started - or held and not started, never both and never neither, and both
// readings come back in name order. It is what lets an install name the projects
// of a forge that has never run.
func TestPropertyStartedAndUnstartedProjectsSplitTheComposedOnes(t *testing.T) {
	root := t.TempDir()
	projectsDir := filepath.Join(root, "projects")
	property := func(states []projectState) bool {
		wantServed, wantHeld, err := layOutForge(projectsDir, states)
		if err != nil {
			return false
		}
		served, err := projects(root)
		if err != nil {
			return false
		}
		return reflect.DeepEqual(served, wantServed) &&
			reflect.DeepEqual(unstartedProjects(root), wantHeld)
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 200,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randomForge(rnd))
		},
	}); err != nil {
		t.Error(err)
	}
}

// TestPropertyAnInstalledFileIsCurrentTheSecondTime is the installer's own
// promise: a file that already holds exactly what the kit ships is left alone
// however it was written, and whatever its name and whatever bytes it holds.
func TestPropertyAnInstalledFileIsCurrentTheSecondTime(t *testing.T) {
	root := t.TempDir()
	// Each case gets its own directory: a name and a body may come round again,
	// and a file that already holds what the kit ships is not this case's first
	// install.
	cases := 0
	property := func(name string, body []byte) bool {
		cases++
		dir := filepath.Join(root, strconv.Itoa(cases))
		name = plainFileName(name)
		source := filepath.Join(dir, "kit", name)
		target := filepath.Join(dir, "forge", name)
		// Install writes into scripts the installer made first, so lay both
		// directories out as a forge root does.
		for _, laidOut := range []string{filepath.Dir(source), filepath.Dir(target)} {
			if err := os.MkdirAll(laidOut, 0o755); err != nil {
				return false
			}
		}
		if err := os.WriteFile(source, body, 0o644); err != nil {
			return false
		}
		first, err := installFile("a tool", source, target)
		if err != nil || !first.changed {
			return false
		}
		second, err := installFile("a tool", source, target)
		if err != nil || second.changed {
			return false
		}
		if second.line != "already current a tool in "+target {
			return false
		}
		return true
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 200,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(plainFileName(randString(rnd)))
			values[1] = reflect.ValueOf(randBytes(rnd))
		},
	}); err != nil {
		t.Error(err)
	}
}

// TestPropertyALostCharacterInAClauseFailsTheSelfCheck is the clause reading's
// own promise: the ring proves a clause by carrying its words whole, so a ring
// that lost any one character of any word - a copy of the sentence that predates
// the clarification the refusal names, or any other drift - is read as a kit
// whose doorbell lost the clause, and its own install fails rather than passing
// quietly. Every word and every position in it is a case, because "the whole
// phrase" is exactly the range this reading has to be exact over.
func TestPropertyALostCharacterInAClauseFailsTheSelfCheck(t *testing.T) {
	scripts := t.TempDir()
	doorbell := filepath.Join(scripts, "doorbell.sh")
	ring := gateClause + ": " + gateRefusal + ". " + answerClause + ": " + answerRefusal + "."
	cases := 0
	for _, gate := range clauses() {
		for _, word := range gate.words {
			for position := range word {
				cases++
				lost := strings.Replace(ring, word, word[:position]+word[position+1:], 1)
				writeScript(t, doorbell, "#!/bin/sh\necho \""+lost+"\"\n")
				line, ok := doorbellClauseRead(scripts)
				if ok {
					t.Fatalf("a ring that lost a character of %s passed its own self-check: %s",
						strconv.Quote(word), line)
				}
				if !strings.Contains(line, strconv.Quote(word)) {
					t.Fatalf("the self-check does not name the whole word it wanted (%s): %s",
						strconv.Quote(word), line)
				}
			}
		}
	}
	if want := len(gateClause) + len(gateRefusal) + len(answerClause) + len(answerRefusal); cases != want {
		t.Fatalf("the reading was proved over %d characters of the clauses, and they hold %d", cases, want)
	}
}

// TestPropertyTheForgeCarriesTheModeTheKitsCopyCarries is what a tool is run
// with: whatever mode the kit's own copy of a file carries, and whatever the
// name suggests it is, the forge's copy carries the kit's mode rather than one
// the installer guessed - an install that rewrote a tool's bit would lose it
// again on the next one.
func TestPropertyTheForgeCarriesTheModeTheKitsCopyCarries(t *testing.T) {
	root := t.TempDir()
	cases := 0
	property := func(name string, mode uint32, script bool) bool {
		cases++
		dir := filepath.Join(root, strconv.Itoa(cases))
		name = plainFileName(name)
		if script {
			name += ".sh"
		}
		source := filepath.Join(dir, "kit", name)
		target := filepath.Join(dir, "forge", name)
		for _, laidOut := range []string{filepath.Dir(source), filepath.Dir(target)} {
			if err := os.MkdirAll(laidOut, 0o755); err != nil {
				return false
			}
		}
		// The owner keeps read and write, so the file the kit ships can be read
		// and replaced; everything else about the mode is the kit's business.
		shipped := os.FileMode(mode&0o777) | 0o600
		if err := os.WriteFile(source, []byte("a tool\n"), shipped); err != nil {
			return false
		}
		carried, err := os.Stat(source)
		if err != nil {
			return false
		}
		if _, err := installFile("a tool", source, target); err != nil {
			return false
		}
		installed, err := os.Stat(target)
		if err != nil {
			return false
		}
		return installed.Mode().Perm() == carried.Mode().Perm()
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 200,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(plainFileName(randString(rnd)))
			values[1] = reflect.ValueOf(uint32(rnd.Intn(0o777)))
			values[2] = reflect.ValueOf(rnd.Intn(2) == 0)
		},
	}); err != nil {
		t.Error(err)
	}
}

// plainFileName is a name a file may have: the generated one with anything that
// is not part of a plain file name replaced, and never a name that would walk
// out of its directory.
func plainFileName(name string) string {
	plain := strings.Map(func(r rune) rune {
		if r == '/' || r == os.PathSeparator || r == 0 {
			return '-'
		}
		return r
	}, name)
	if strings.Trim(plain, ".") == "" {
		return "file" + plain
	}
	return plain
}

// randString is a name of bytes a file name might be made of, which the
// generator's own strings are not.
func randString(rnd *rand.Rand) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._- /"
	length := 1 + rnd.Intn(12)
	var name strings.Builder
	for index := 0; index < length; index++ {
		name.WriteByte(alphabet[rnd.Intn(len(alphabet))])
	}
	return name.String()
}

// randBytes is a file's body, of the length a body may have.
func randBytes(rnd *rand.Rand) []byte {
	body := make([]byte, rnd.Intn(64))
	for index := range body {
		body[index] = byte(rnd.Intn(256))
	}
	return body
}

// TestPropertyTheReportReadsBackWhateverVerdictItPrinted is the report parser's
// own promise, and the reason a verdict can be added without touching it: for
// any role and any verdict the check prints - the standing ones and the agent
// that has gone - a reader gets that role and that verdict back, and reads no
// card where the check wrote none. A column carrying a key ("new=",
// "in_process=") is the check's count, never the card a role holds.
func TestPropertyTheReportReadsBackWhateverVerdictItPrinted(t *testing.T) {
	property := func(role, verdict string) bool {
		// "not-running" is the forge's own line rather than a role's verdict,
		// and the check names a role or a verdict with no space in it.
		if role == "" || verdict == "" || verdict == "not-running" {
			return true
		}
		notRunning, readings := parseIdlerReport(role + " " + verdict)
		if notRunning || len(readings) != 1 {
			return false
		}
		found := readings[0]
		return found.Role == role && found.Verdict == verdict &&
			found.Card == "-" && found.MailCount == 0
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 300,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(plainReportWord(rnd))
			values[1] = reflect.ValueOf(plainReportWord(rnd))
		},
	}); err != nil {
		t.Error(err)
	}
}

// plainReportWord is one word of a report line as the check writes it: letters
// and dashes, never a space and never the "=" that would make a column a count
// rather than a name.
func plainReportWord(rnd *rand.Rand) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz-"
	word := make([]byte, 1+rnd.Intn(8))
	for index := range word {
		word[index] = alphabet[rnd.Intn(len(alphabet))]
	}
	return string(word)
}
