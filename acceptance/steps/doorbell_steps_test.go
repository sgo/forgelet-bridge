package steps

import (
	"strings"
	"testing"
)

// TestThePaneReadingCountsTheTurnsAboveTheComposer pins the reading a ring's
// landing is judged by: the turns a pane has taken are the ones above its
// composer's mark, a turn runs on for the lines its own text wrapped onto, and
// the words under the composer's mark are what the pane is still holding rather
// than a turn it took.
func TestThePaneReadingCountsTheTurnsAboveTheComposer(t *testing.T) {
	screen := strings.Join([]string{
		turnMark + "[req-1] hello",
		"a line the pane wrapped",
		turnMark + "another turn",
		composerMark + "held words",
		"still held",
		"",
	}, "\n")

	turns := submittedTurns(screen)
	if len(turns) != 2 || turns[0] != "[req-1] hello\na line the pane wrapped" || turns[1] != "another turn" {
		t.Errorf("the pane's turns read as %q, and it took two", turns)
	}
	if held := composerOf(screen); held != "held words\nstill held" {
		t.Errorf("the composer reads as %q, and the pane is holding two lines", held)
	}
}

// TestTheComposerReadsAsEmptyWhenThePaneIsHoldingNothing is the other half the
// step reads: a pane that took the turn is holding nothing, and nothing is what
// its own drawing has to say after the composer's mark.
func TestTheComposerReadsAsEmptyWhenThePaneIsHoldingNothing(t *testing.T) {
	screen := turnMark + "[req-1] hello\n" + composerMark

	if held := composerOf(screen); held != "" {
		t.Errorf("the composer reads as %q, and the pane is holding nothing", held)
	}
}
