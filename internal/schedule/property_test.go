//go:build property

package schedule

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/quick"
	"time"
)

// The bound the sweep goes by, and the room a case keeps away from it: what a
// pass calls stale is a tree nobody has touched for longer than the week it
// keeps, and a case that sat on the boundary would be asking which second the
// pass read rather than which rule it follows.
const (
	bound     = 7 * 24 * time.Hour
	wellStale = 8 * 24 * time.Hour
	wellFresh = 5 * 24 * time.Hour
)

// tmpEntry is one thing a case leaves in the forge's own tmp: a scratch
// directory a run left, with the time its own row was touched and the time the
// work inside it was - which is the reading the age rule takes - or a loose file
// somebody kept there.
type tmpEntry struct {
	name    string
	scratch bool
	own     time.Time
	inside  time.Time
}

// tmpCase is what one pass meets: the things a forge's own tmp holds, and the
// days of the schedule's own log it holds with them.
type tmpCase struct {
	entries []tmpEntry
	days    []int
}

// TestPropertyThePassTakesWhatIsPastItsBoundAndNamesIt is the rule the card is
// loudest about, over what a forge really holds: what a pass takes from the
// forge's own tmp is the scratch nobody has touched for longer than the week it
// keeps, whatever the entry is called, and what it takes there is a directory
// rather than the file a person left; what it takes from its own log is the
// days outside that same week, read from the day in the name, while the day
// sitting on the bound is inside it; and every removal it makes it names. A case
// is a whole pass, so a case carries all of that rather than the property being
// run over one thing at a time.
func TestPropertyThePassTakesWhatIsPastItsBoundAndNamesIt(t *testing.T) {
	property := func(one tmpCase) bool {
		f := newFixture(t)
		cutoff := time.Now().UTC().Add(-bound)
		logs := map[string]int{}
		for _, days := range one.days {
			logs[f.log(time.Duration(days)*24*time.Hour)] = days
		}
		wantGone := map[string]bool{}
		wantKept := map[string]bool{}
		for index, entry := range one.entries {
			// A case carries entries whose names could come round again; the
			// index is what keeps two of them apart.
			name := fmt.Sprintf("%s-%d", entry.name, index)
			if !entry.scratch {
				path := filepath.Join(f.root, "tmp", name)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					return false
				}
				if err := os.WriteFile(path, []byte("the answer a person gave\n"), 0o644); err != nil {
					return false
				}
				if err := os.Chtimes(path, entry.own, entry.own); err != nil {
					return false
				}
				wantKept[path] = true
				continue
			}
			path := f.scratch(name, entry.own)
			if err := os.Chtimes(filepath.Join(path, "inner", "work.txt"), entry.inside, entry.inside); err != nil {
				return false
			}
			newest := entry.own
			if entry.inside.After(newest) {
				newest = entry.inside
			}
			if newest.Before(cutoff) {
				wantGone[path] = true
			} else {
				wantKept[path] = true
			}
		}

		out := f.run()

		for path := range wantGone {
			if !f.gone(path) {
				return false
			}
			// A removal nobody can read about is the silent cleanup the pass
			// exists not to be.
			if !strings.Contains(out, path) {
				return false
			}
		}
		for path := range wantKept {
			if f.gone(path) {
				return false
			}
		}
		for path, days := range logs {
			if days > int(bound/(24*time.Hour)) {
				if !f.gone(path) || !strings.Contains(out, path) {
					return false
				}
				continue
			}
			if f.gone(path) {
				return false
			}
		}
		return len(wantGone)+len(wantKept) == len(one.entries)
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 3,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randCase(rnd))
		},
	}); err != nil {
		t.Error(err)
	}
}

// randCase is what one case's pass meets: the four things the rule tells apart
// in a forge's own tmp, the day sitting on the bound of its own log and the day
// just past it, and then whatever else the draw adds. A case is a whole pass, so
// a case that happened to hold none of them would spend one proving nothing.
func randCase(rnd *rand.Rand) tmpCase {
	week := int(bound / (24 * time.Hour))
	days := []int{week, week + 1}
	for extra := rnd.Intn(3); extra > 0; extra-- {
		if rnd.Intn(2) == 0 {
			days = append(days, rnd.Intn(week))
			continue
		}
		days = append(days, week+1+rnd.Intn(33))
	}
	return tmpCase{entries: randTmp(rnd), days: days}
}

// randTmp is what one case's pass meets in the forge's own tmp: the four things
// the rule has to tell apart - scratch nobody has touched, the scratch a run
// left just now, a scratch a run is still filling, and the file a person kept
// there - and then whatever else the draw adds. A case is a whole pass, so a
// case that happens to hold none of the four would spend one proving nothing.
func randTmp(rnd *rand.Rand) []tmpEntry {
	now := time.Now().UTC()
	stale := now.Add(-(wellStale + time.Duration(rnd.Intn(32))*24*time.Hour))
	fresh := now.Add(-time.Duration(rnd.Intn(int(wellFresh/(24*time.Hour)))) * 24 * time.Hour)
	entries := []tmpEntry{
		{name: randName(rnd), scratch: true, own: stale, inside: stale},
		{name: randName(rnd), scratch: true, own: fresh, inside: fresh},
		{name: randName(rnd), scratch: true, own: stale, inside: fresh},
		{name: randName(rnd), scratch: false, own: stale, inside: stale},
	}
	for extra := rnd.Intn(3); extra > 0; extra-- {
		entries = append(entries, tmpEntry{
			name:    randName(rnd),
			scratch: rnd.Intn(4) > 0,
			own:     randTouched(rnd, now),
			inside:  randTouched(rnd, now),
		})
	}
	return entries
}

// randTouched is when an entry was last touched: outside the bound the pass
// keeps, or well inside it, and never on it.
func randTouched(rnd *rand.Rand, now time.Time) time.Time {
	if rnd.Intn(2) == 0 {
		return now.Add(-(wellStale + time.Duration(rnd.Intn(32))*24*time.Hour))
	}
	return now.Add(-time.Duration(rnd.Intn(int(wellFresh/(24*time.Hour)))) * 24 * time.Hour)
}

// randName is what a run, or a person, could call something left in a forge's
// own tmp: the characters a shell cares about, and a first character that is
// neither a dot - the sweep's glob is the plain one, and a dot entry is not a
// name any of these runs leaves - nor a slash, which would make the name a path.
func randName(rnd *rand.Rand) string {
	const alphabet = "abcXYZ019-_ .'\"$&()[]#!%+@^~"
	const first = "abcXYZ019"
	var name strings.Builder
	name.WriteByte(first[rnd.Intn(len(first))])
	for length := rnd.Intn(12); length > 0; length-- {
		name.WriteByte(alphabet[rnd.Intn(len(alphabet))])
	}
	return name.String()
}
