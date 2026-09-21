//go:build property

package dashboard

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"testing/quick"
)

// TestPropertyRenderThenParseKeepsTheRequest checks the dashboard request file
// survives a round trip through the reader every tool uses.
func TestPropertyRenderThenParseKeepsTheRequest(t *testing.T) {
	property := func(request Request) bool {
		return Parse(render(request)) == request
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 500,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randomRequest(rnd))
		},
	}); err != nil {
		t.Error(err)
	}
}

// TestPropertyRenderIsStable checks that reading a request and writing it
// again leaves the file the dashboard reads exactly as it was.
func TestPropertyRenderIsStable(t *testing.T) {
	property := func(request Request) bool {
		written := render(request)
		return render(Parse(written)) == written
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 500,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randomRequest(rnd))
		},
	}); err != nil {
		t.Error(err)
	}
}

// randomRequest builds a request of the shape the dashboard really writes:
// header fields are single lines without edge whitespace, a response is a
// single line, and a body is one or more lines with no blank line inside.
func randomRequest(rnd *rand.Rand) Request {
	request := Request{
		ID:        randomHeaderValue(rnd),
		Status:    randomHeaderValue(rnd),
		Role:      randomHeaderValue(rnd),
		Body:      randomBody(rnd),
		CreatedAt: randomHeaderValue(rnd),
		UpdatedAt: randomHeaderValue(rnd),
	}
	if rnd.Intn(2) == 0 {
		request.Response = randomResponse(rnd)
	}
	return request
}

// randomHeaderValue is a single line without edge whitespace, the way the
// dashboard writes its header fields.
func randomHeaderValue(rnd *rand.Rand) string {
	values := []string{"", "req-1", "pending", "done", "2026-09-21T20:04:32.588996Z", "a b c", "x:y"}
	return values[rnd.Intn(len(values))]
}

// randomResponse is one or more lines of the shape a request file can carry:
// the format escapes newlines with a backslash, so a literal backslash is out
// of its domain, and the dashboard trims an answer before it writes one.
func randomResponse(rnd *rand.Rand) string {
	var lines []string
	for count := 1 + rnd.Intn(3); count > 0; count-- {
		lines = append(lines, strings.TrimSpace(randomBodyLine(rnd)))
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

// randomBody is an empty body, or one or more non-empty lines with no blank
// line inside and no trailing newline: the blank line is what separates the
// headers from the body, so the format cannot carry one.
func randomBody(rnd *rand.Rand) string {
	if rnd.Intn(4) == 0 {
		return ""
	}
	var lines []string
	for count := 1 + rnd.Intn(3); count > 0; count-- {
		lines = append(lines, randomBodyLine(rnd))
	}
	return strings.Join(lines, "\n")
}

func randomBodyLine(rnd *rand.Rand) string {
	values := []string{"is the build green?", "yes, the build is green", "  indented  ", "id: not a header"}
	return values[rnd.Intn(len(values))]
}
