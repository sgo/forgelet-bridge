//go:build property

package prune

import (
	"fmt"
	"math/rand"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"testing/quick"
	"time"
)

// The window a run in flight is read within, and the room a case keeps away from
// it: a case that sat on the boundary would be asking what minute the prune
// rounded to rather than which rule it follows.
const (
	inFlight  = 15 * time.Minute
	wellLive  = 10 * time.Minute
	wellQuiet = 25 * time.Minute
)

// burst is what one prune meets: a directory of mutant runs, each with how long
// ago it last wrote anything - the whole of what the prune reads - and how many
// of them the rule is asked to keep.
type burst struct {
	keep int
	ages []time.Duration
}

// TestPropertyThePruneNeverTakesARunSomethingIsWritingIn is the rule the card
// was folded to hold, over a burst of mutant runs rather than a shape chosen by
// hand: a directory of runs something wrote in within the window is left whole -
// whoever the long run in it is, and however many short ones the burst put beside
// it - and a directory nothing has written in for longer than that window is the
// debris the count exists for, so the newest few are kept and the rest go. A
// case is a whole prune, so it carries every reading at once.
func TestPropertyThePruneNeverTakesARunSomethingIsWritingIn(t *testing.T) {
	property := func(one burst) bool {
		f := newFixture(t)
		mutations := f.mutationDir("forge-schedule")
		now := time.Now()

		var runs []string
		live := false
		for index, age := range one.ages {
			when := now.Add(-age)
			runs = append(runs, f.mutantRun(mutations, fmt.Sprintf("m%d", index+1), when))
			live = live || age < inFlight
		}
		f.touch(mutations, now.Add(-oldest(one.ages)))

		out := f.ran("--keep-mutants", strconv.Itoa(one.keep))

		left := len(f.stillHere(runs))
		due := len(runs) - one.keep // what the count alone would take
		if live {
			// A run in flight is the one thing the count may not take, so a
			// directory holding one is left whole - and a prune that was due
			// under the count says which directory it left alone.
			if left != len(runs) {
				return false
			}
			return due <= 0 || strings.Contains(out, "left the runs under")
		}
		if left != min(one.keep, len(runs)) {
			return false
		}
		return strings.Contains(out, fmt.Sprintf("pruned %d directories", max(due, 0)))
	}
	// Half the cases are drawn with a run in flight and half without, so the
	// property spends itself on both readings rather than on whichever the draw
	// happens to hand it.
	drawn := 0
	if err := quick.Check(property, &quick.Config{
		MaxCount: 4,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			drawn++
			values[0] = reflect.ValueOf(randBurst(rnd, drawn%2 == 1))
		},
	}); err != nil {
		t.Error(err)
	}
}

// randBurst is what one prune meets: a run something may be writing in right
// now when the case asks for one, the runs a burst of short ones left beside it,
// and how many of them the rule keeps. Every case carries a run that went quiet
// outside the window, so a case with nothing in flight is the debris the count
// exists for rather than an empty draw.
func randBurst(rnd *rand.Rand, inFlightRun bool) burst {
	ages := []time.Duration{}
	if inFlightRun {
		ages = append(ages, time.Minute+time.Duration(rnd.Intn(int(wellLive/time.Minute)))*time.Minute)
	}
	for extra := 1 + rnd.Intn(4); extra > 0; extra-- {
		ages = append(ages, wellQuiet+time.Duration(rnd.Intn(30))*time.Minute)
	}
	return burst{keep: 1 + rnd.Intn(3), ages: ages}
}

// oldest is the longest ago anything in a burst wrote: what the directory
// holding the runs has to read as, since the directory is one a pass left then.
func oldest(ages []time.Duration) time.Duration {
	oldest := time.Duration(0)
	for _, age := range ages {
		if age > oldest {
			oldest = age
		}
	}
	return oldest
}
