//go:build property

package rules

import (
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"
)

// TestPropertyAGoodBlockParsesBackToItself is the round trip the installer's
// markers rest on: a block written the way the installer writes it parses back
// to its subject and to the very block it came from, so what is read back is
// byte for byte what was installed.
func TestPropertyAGoodBlockParsesBackToItself(t *testing.T) {
	property := func(subject, body string) bool {
		block := blockFor(subject, body)
		rule, err := Parse(block)
		if err != nil {
			return false
		}
		return rule.Subject == subject && rule.Block == block
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 300,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(pick(rnd, ruleSubjects))
			values[1] = reflect.ValueOf(pick(rnd, ruleBodies))
		},
	}); err != nil {
		t.Error(err)
	}
}

// TestPropertyInstallingTwiceChangesNothing is the promise the operator can run
// it again on: once a rule is installed, installing the same rules a second
// time changes nothing and reports every one of them as already current, so
// drift showing up as a diff means something really drifted.
func TestPropertyInstallingTwiceChangesNothing(t *testing.T) {
	property := func(subjects []string) bool {
		root := t.TempDir()
		loaded := make([]Rule, 0, len(subjects))
		for _, subject := range subjects {
			block := blockFor(subject, "A rule body.")
			rule, err := Parse(block)
			if err != nil {
				return false
			}
			loaded = append(loaded, rule)
		}

		first, err := Install(root, loaded)
		if err != nil {
			return false
		}
		if len(first.Changed) != len(loaded) || len(first.Current) != 0 {
			return false
		}

		second, err := Install(root, loaded)
		if err != nil {
			return false
		}
		return len(second.Changed) == 0 && len(second.Current) == len(loaded)
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 300,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			subjects := []string{"stopping-without-finishing", "asking-before-deleting", "rule-three"}
			// The subjects are distinct: installing the same subject twice in
			// one run is the second install seeing its own work, not two rules.
			rnd.Shuffle(len(subjects), func(i, j int) { subjects[i], subjects[j] = subjects[j], subjects[i] })
			chosen := subjects[:rnd.Intn(len(subjects)+1)]
			values[0] = reflect.ValueOf(chosen)
		},
	}); err != nil {
		t.Error(err)
	}
}

// blockFor writes a rule block the way the installer writes one.
func blockFor(subject, body string) string {
	return markerPrefix + subject + " -->\n" + body + "\n" + markerEnd + subject + " -->\n"
}

// ruleSubjects are the rule names the features install; ruleBodies are the
// wording a rule block can carry, including a block with nothing said.
var ruleSubjects = []string{"stopping-without-finishing", "asking-before-deleting", "rule-two", "rule-three"}

var ruleBodies = []string{
	"",
	"A rule body.",
	"a role that stops while holding a card raises a clarification",
	"line one\nline two",
	"  spaced  ",
}

// pick is one of a set, at random.
func pick[T any](rnd *rand.Rand, options []T) T {
	return options[rnd.Intn(len(options))]
}
