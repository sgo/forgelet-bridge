//go:build property

package board

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"testing/quick"
)

// TestPropertyParseRowReadsTheRowsTheBoardWrites checks the one rule every
// reader of a board file shares: the row the board writes for a card in a lane
// reads back as that card in that lane, whatever the rest of the columns hold.
func TestPropertyParseRowReadsTheRowsTheBoardWrites(t *testing.T) {
	property := func(card, lane string, columns []string) bool {
		if strings.TrimSpace(card) == "" || strings.TrimSpace(lane) == "" {
			return true // a row without a card or a lane is not a row
		}
		line := strings.Join(append([]string{card, lane}, columns...), "\t")

		name, gotLane, ok := ParseRow(line)
		return ok && name == strings.TrimSpace(card) && gotLane == strings.TrimSpace(lane)
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 300,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randomColumn(rnd))
			values[1] = reflect.ValueOf(randomColumn(rnd))
			values[2] = reflect.ValueOf(randomColumns(rnd, 0))
		},
	}); err != nil {
		t.Error(err)
	}
}

// TestPropertyParseRowIgnoresWhatIsNotARow checks the other half of that rule:
// a line without a card and a lane names no card, so it never reaches the
// operator as one.
func TestPropertyParseRowIgnoresWhatIsNotARow(t *testing.T) {
	property := func(columns []string) bool {
		if len(columns) >= 2 && strings.TrimSpace(columns[0]) != "" && strings.TrimSpace(columns[1]) != "" {
			return true // this line is a row, which the other property covers
		}
		_, _, ok := ParseRow(strings.Join(columns, "\t"))
		return !ok
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 300,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randomColumns(rnd, 0))
		},
	}); err != nil {
		t.Error(err)
	}
}

// randomColumn is one column of a board row: a card name, a lane, or one of the
// bookkeeping columns the board writes beside them.
func randomColumn(rnd *rand.Rand) string {
	columns := []string{
		"",
		" ",
		"card-activity-feed",
		" specifier ",
		"2026-09-22T15:00:00Z",
		"task-1",
		"0",
	}
	return columns[rnd.Intn(len(columns))]
}

// randomColumns is a whole row's columns, up to most of a row.
func randomColumns(rnd *rand.Rand, atLeast int) []string {
	count := atLeast + rnd.Intn(4)
	columns := make([]string, 0, count)
	for range count {
		columns = append(columns, randomColumn(rnd))
	}
	return columns
}
