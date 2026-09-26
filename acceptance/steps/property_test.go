//go:build property

package steps

import (
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"testing/quick"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
)

// TestPropertyThePanesReadingDoesNotDependOnTheWrapping is how a pane's own text
// is read: a pane wraps what it holds, so the same words at any width have to
// read the same, and reading a reading again has to change nothing. It is what
// lets a step look for the words of a ring rather than for its line breaks.
func TestPropertyThePanesReadingDoesNotDependOnTheWrapping(t *testing.T) {
	property := func(words []string) bool {
		joined := strings.Join(words, " ")
		// The same words where a pane could have broken the line instead.
		wrapped := strings.Join(words, "\n   ")
		if squashed(wrapped) != squashed(joined) {
			return false
		}
		if squashed(squashed(joined)) != squashed(joined) {
			return false
		}
		return strings.Contains(squashed(wrapped), squashed(joined))
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 300,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randWords(rnd))
		},
	}); err != nil {
		t.Error(err)
	}
}

// randWords is the words a reading is made of, wrapping and all.
func randWords(rnd *rand.Rand) []string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789.-\t "
	words := make([]string, rnd.Intn(8))
	for index := range words {
		word := make([]byte, 1+rnd.Intn(6))
		for at := range word {
			word[at] = alphabet[rnd.Intn(len(alphabet))]
		}
		words[index] = string(word)
	}
	return words
}

// TestPropertyTheComposerIsNotATurn is the reading the ring's landing turns on:
// a pane's screen is the turns it has taken and, under them, the words its
// composer is still holding - and only the turns above the composer's mark are
// turns. Text the pane has not submitted is not a session's reading of anything,
// whatever it holds, which is the false delivery the doorbell's composer check
// exists to stop. The screen here is drawn the way the fixture's terminal draws
// it, and the scenarios are what keep the two drawings the same.
func TestPropertyTheComposerIsNotATurn(t *testing.T) {
	property := func(turns []string, held string) bool {
		screen := drawnPane(turns, held)
		read := submittedTurns(screen)
		if len(read) != len(turns) {
			return false
		}
		for index, turn := range turns {
			if read[index] != turn {
				return false
			}
		}
		return composerOf(screen) == strings.TrimSpace(held)
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 300,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randTurns(rnd))
			values[1] = reflect.ValueOf(randHeld(rnd))
		},
	}); err != nil {
		t.Error(err)
	}
}

// drawnPane is the screen the fixture's terminal draws: every turn it has taken
// under the mark a turn is drawn with, then the composer's own mark and the
// words it is still holding.
func drawnPane(turns []string, held string) string {
	var screen strings.Builder
	for _, turn := range turns {
		screen.WriteString(turnMark + turn + "\n")
	}
	screen.WriteString(composerMark + held)
	return screen.String()
}

// randTurns is a few turns a pane could have taken, each of them a line or
// three: a turn runs to lines when the words a session sent did.
func randTurns(rnd *rand.Rand) []string {
	turns := make([]string, rnd.Intn(4))
	for index := range turns {
		lines := make([]string, 1+rnd.Intn(3))
		for at := range lines {
			lines[at] = randLine(rnd)
		}
		turns[index] = strings.Join(lines, "\n")
	}
	return turns
}

// randHeld is what a composer could still be holding: the text of one request
// or several lines of it, which is the whole of what has not been submitted.
func randHeld(rnd *rand.Rand) string {
	lines := make([]string, 1+rnd.Intn(3))
	for at := range lines {
		lines[at] = randLine(rnd)
	}
	return strings.Join(lines, "\n")
}

// randLine is one line of a pane's own drawing, with the words a person or an
// agent leaves in it and never a mark: a line that begins with one is the
// fixture's drawing rather than anything a session read, and a screen whose
// words wear the fixture's own marks is a different question than this one.
func randLine(rnd *rand.Rand) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789.-, "
	line := make([]byte, rnd.Intn(16))
	for at := range line {
		line[at] = alphabet[rnd.Intn(len(alphabet))]
	}
	return strings.TrimSpace(string(line))
}

// TestPropertyTheFinishingStepNeverRewritesWhatOriginAlreadyHad is the promise
// the step makes about a shared remote, however far the remote has moved on: a
// push that cannot land leaves every commit origin held exactly where it was,
// and a push that can land puts the card's commit there. What another checkout
// put on origin is never this step's to discard.
func TestPropertyTheFinishingStepNeverRewritesWhatOriginAlreadyHad(t *testing.T) {
	base := t.TempDir()
	cases := 0
	property := func(ahead int) bool {
		cases++
		fixture, theirs, err := finishableFixture(base, cases, ahead)
		if err != nil {
			return false
		}
		ctx, cancel := stepContext()
		defer cancel()

		fixture.runHook(ctx, pushCard)

		held, err := fixture.originHolds(fixture.branch)
		if err != nil {
			return false
		}
		if ahead > 0 {
			// The remote had work of its own: the step refuses rather than
			// forcing, and everything origin held is still there.
			return fixture.err != nil && held == theirs &&
				strings.Contains(fixture.output, "the push failed")
		}
		// The remote had none: the card's commit is the work the step owes it.
		return fixture.err == nil && held == fixture.commit &&
			strings.Contains(fixture.output, "pushed")
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 12,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(rnd.Intn(4))
		},
	}); err != nil {
		t.Error(err)
	}
}

// finishableFixture lays a fixture out under base, in a directory of its own: a
// checkout of the bridge with the card's work landed and an origin, moved on by
// however many commits another checkout has already put there. It answers the
// fixture and the commit the origin holds before the step runs, which is empty
// when that origin held nothing at all.
func finishableFixture(base string, caseNumber, ahead int) (*cardCompleteFixture, string, error) {
	dir := filepath.Join(base, strconv.Itoa(caseNumber))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, "", err
	}
	w := newWorld()
	w.workDir = dir
	fixture, err := w.cardCompleteProject()
	if err != nil {
		return nil, "", err
	}
	if err := fixture.withOrigin(); err != nil {
		return nil, "", err
	}
	if err := fixture.landCard(pushCard); err != nil {
		return nil, "", err
	}
	if ahead == 0 {
		return fixture, "", nil
	}
	theirs, err := fixture.moveOriginOnBy(ahead)
	return fixture, theirs, err
}

// TestPropertySameDeviceAcceptsTheListingItWasGiven checks the restart rule
// against itself: the device listing the operator saw before the restart always
// passes as the listing after it. An empty listing is not a device listing, so
// it is out of the property's domain — the step reports it instead.
func TestPropertySameDeviceAcceptsTheListingItWasGiven(t *testing.T) {
	property := func(before []fixtures.DeviceKey) bool {
		if len(before) == 0 {
			return true
		}
		return sameDevice(before, before) == nil
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 300,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randomDevices(rnd))
		},
	}); err != nil {
		t.Error(err)
	}
}

// TestPropertySameDeviceIgnoresTheOrderOfTheListing checks that the verdict
// does not depend on the order the homeserver happened to answer in.
func TestPropertySameDeviceIgnoresTheOrderOfTheListing(t *testing.T) {
	property := func(before []fixtures.DeviceKey, seed int64) bool {
		shuffled := append([]fixtures.DeviceKey(nil), before...)
		rnd := rand.New(rand.NewSource(seed))
		rnd.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })

		return (sameDevice(before, shuffled) == nil) == (sameDevice(before, before) == nil)
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 300,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randomDevices(rnd))
			values[1] = reflect.ValueOf(rnd.Int63())
		},
	}); err != nil {
		t.Error(err)
	}
}

// TestPropertySameDeviceRejectsADifferentListing checks the rule bites: a
// listing that gains a device, loses one, or renames an identity key is never
// the same as the one before the restart.
func TestPropertySameDeviceRejectsADifferentListing(t *testing.T) {
	property := func(before []fixtures.DeviceKey) bool {
		if len(before) == 0 {
			return true
		}

		gained := append(append([]fixtures.DeviceKey(nil), before...), fixtures.DeviceKey{DeviceID: "FORGELETBRIDGE-NEW", Ed25519: "key-new"})
		lost := before[:len(before)-1]
		renamed := append([]fixtures.DeviceKey(nil), before...)
		renamed[0].Ed25519 += "-other"

		return sameDevice(before, gained) != nil &&
			sameDevice(before, lost) != nil &&
			sameDevice(before, renamed) != nil
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 300,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randomDevices(rnd))
		},
	}); err != nil {
		t.Error(err)
	}
}

// randomDevices builds the device listing a client would answer with: distinct
// device ids, sorted the way the fixture sorts them.
func randomDevices(rnd *rand.Rand) []fixtures.DeviceKey {
	ids := []string{"FORGELETBRIDGE", "FORGELETBRIDGE2", "AAAABBBB", "device with spaces"}
	count := rnd.Intn(4)
	devices := make([]fixtures.DeviceKey, 0, count)
	for _, deviceID := range ids[:count] {
		devices = append(devices, fixtures.DeviceKey{
			DeviceID:   deviceID,
			Ed25519:    "key-" + deviceID,
			Curve25519: "curve-" + deviceID,
		})
	}
	if len(devices) == 0 {
		return nil
	}
	return devices
}
