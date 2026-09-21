//go:build property

package runtime

import (
	"math/rand"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"testing/quick"
)

// TestPropertyExpandFillsEveryPlaceholder checks the placeholder rule the
// generated steps depend on: every <name> is replaced by its example value and
// the rest of the text is left alone.
func TestPropertyExpandFillsEveryPlaceholder(t *testing.T) {
	property := func(pairs []string, values []string) bool {
		text, example, want, ok := phrase(pairs, values)
		if !ok {
			return true
		}
		got, err := Expand(text, example)
		return err == nil && got == want
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 300,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			names, filled := randomPhrase(rnd)
			values[0] = reflect.ValueOf(names)
			values[1] = reflect.ValueOf(filled)
		},
	}); err != nil {
		t.Error(err)
	}
}

// TestPropertyExpandLeavesTextWithoutPlaceholdersAlone checks that a step with
// no placeholders is passed through unchanged.
func TestPropertyExpandLeavesTextWithoutPlaceholdersAlone(t *testing.T) {
	property := func(text string) bool {
		if strings.ContainsAny(text, "<>") {
			return true
		}
		got, err := Expand(text, map[string]string{})
		return err == nil && got == text
	}
	if err := quick.Check(property, nil); err != nil {
		t.Error(err)
	}
}

// TestPropertyExpandNamesAMissingExampleValue checks that an example row
// missing a value fails loudly instead of expanding to something else.
func TestPropertyExpandNamesAMissingExampleValue(t *testing.T) {
	property := func(name string) bool {
		if name == "" || strings.ContainsAny(name, "<>") {
			return true
		}
		_, err := Expand("the forge holds <"+name+"> chat requests", map[string]string{})
		return err != nil && strings.Contains(err.Error(), "<"+name+">")
	}
	if err := quick.Check(property, nil); err != nil {
		t.Error(err)
	}
}

// phrase builds a step text with one placeholder per name, the example row
// that fills them, and the text Expand has to produce. It reports false when
// the names cannot be told apart in the example row.
func phrase(names, values []string) (string, map[string]string, string, bool) {
	if len(names) == 0 || len(names) != len(values) {
		return "", nil, "", false
	}
	example := map[string]string{}
	var text, want strings.Builder
	for index, name := range names {
		if name == "" || strings.ContainsAny(name, "<>") {
			return "", nil, "", false
		}
		if _, seen := example[name]; seen {
			return "", nil, "", false
		}
		example[name] = values[index]
		text.WriteString("word" + strconv.Itoa(index) + " <" + name + "> ")
		want.WriteString("word" + strconv.Itoa(index) + " " + values[index] + " ")
	}
	return text.String(), example, want.String(), true
}

// randomPhrase picks count distinct placeholder names and one value for each.
func randomPhrase(rnd *rand.Rand) ([]string, []string) {
	names := []string{"chat_requests", "n", "room", "forge", "message"}
	values := []string{"0", "1", "Chat", "forge-a", "is the build green?"}

	rnd.Shuffle(len(names), func(i, j int) { names[i], names[j] = names[j], names[i] })
	count := rnd.Intn(len(names) + 1)

	chosenNames := append([]string(nil), names[:count]...)
	chosenValues := make([]string, 0, count)
	for range count {
		chosenValues = append(chosenValues, values[rnd.Intn(len(values))])
	}
	return chosenNames, chosenValues
}
