package kit

import (
	"fmt"
	"strconv"
	"strings"
)

// reading is one role's line from the check's report: the verdict it reached,
// the card it saw on the board, and the mail it found.
type reading struct {
	Role      string
	Verdict   string
	Card      string
	Mail      string
	MailCount int
}

// parseIdlerReport reads the check's report: whether the forge has no sessions
// at all, and what it read for each role.
func parseIdlerReport(output string) (bool, []reading) {
	notRunning := false
	var readings []reading
	for _, line := range strings.Split(strings.TrimRight(output, "\n"), "\n") {
		columns := strings.Fields(line)
		if len(columns) < 2 {
			continue
		}
		if columns[1] == "not-running" {
			notRunning = true
			continue
		}
		readings = append(readings, readingFrom(columns))
	}
	return notRunning, readings
}

// readingFrom is one role's line of the check's report: the role, its verdict,
// the card the board showed, and the mail.
func readingFrom(columns []string) reading {
	found := reading{Role: columns[0], Verdict: columns[1], Card: "-"}
	if len(columns) > 2 && !strings.Contains(columns[2], "=") {
		found.Card = columns[2]
	}
	found.Mail, found.MailCount = mailOf(columns)
	return found
}

// mailOf is the mail a report line names: a new one first, then the one still
// in process.
func mailOf(columns []string) (string, int) {
	mail, count := "", 0
	for _, column := range columns {
		if got, ok := countOf(column, "new="); ok && got > 0 {
			mail, count = "new="+strconv.Itoa(got), got
			continue
		}
		if got, ok := countOf(column, "in_process="); ok && got > 0 && count == 0 {
			mail, count = "in_process="+strconv.Itoa(got), got
		}
	}
	return mail, count
}

// countOf reads one count column of a report line.
func countOf(column, prefix string) (int, bool) {
	if !strings.HasPrefix(column, prefix) {
		return 0, false
	}
	count, err := strconv.Atoi(strings.TrimPrefix(column, prefix))
	return count, err == nil
}

// idlerEvidence is what one run of the check read: the pane it looked at, the
// marker it found, the board and the inbox. An empty board and an empty inbox
// are reads the tool made - a forge between cards is a forge the check can
// read - so only reading nothing at all fails: no pane, no role, or no live
// session behind the one it names.
func idlerEvidence(project, command, output string) (string, bool) {
	notRunning, readings := parseIdlerReport(output)
	if notRunning {
		return fmt.Sprintf("self-check failed idler check: ran %q, the forge is not running", command), false
	}
	if len(readings) == 0 {
		return fmt.Sprintf("self-check failed idler check: ran %q, which read no role", command), false
	}
	var first string
	var quiet string
	for _, reading := range readings {
		evidence := fmt.Sprintf("read pane %s, found marker %s", paneOf(project, reading.Role), reading.Verdict)
		if reading.Verdict == "session-gone" {
			if first == "" {
				first = fmt.Sprintf("self-check failed idler check: ran %q, %s, and there is no live pane behind it", command, evidence)
			}
			continue
		}
		line := fmt.Sprintf("self-check idler check: ran %q, %s, board %s, inbox %s",
			command, evidence, boardRead(reading), inboxRead(reading))
		// A role with something in flight is the reading that says the most, so
		// it is the one the report carries when there is one.
		if boardRead(reading) != "empty" || inboxRead(reading) != "empty" {
			return line, true
		}
		if quiet == "" {
			quiet = line
		}
	}
	if quiet != "" {
		// Every role read was empty, and an empty board and inbox are reads the
		// tool made: a forge between cards is a forge the check can read.
		return quiet, true
	}
	return first, false
}

// boardRead is what the check found in the card's lane. Nothing there is a
// reading too: the tool looked and the board held no card.
func boardRead(found reading) string {
	if found.Card == "" || found.Card == "-" {
		return "empty"
	}
	return found.Card
}

// inboxRead is what the check found in the role's own inbox: the mail waiting
// there, or nothing, which is still something the tool read.
func inboxRead(found reading) string {
	if found.MailCount == 0 {
		return "empty"
	}
	return found.Mail
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-24T16:28:26+02:00","module_hash":"6e739b3b86f1533f0dc0389b9072eae720278843a4f2265a664624203b3a039d","functions":[{"id":"func/parseIdlerReport","name":"parseIdlerReport","line":21,"end_line":36,"hash":"d1317252a05b64fbc31b286c504434c8d4c16a6096260f00597f947e6c8408ef"},{"id":"func/readingFrom","name":"readingFrom","line":40,"end_line":47,"hash":"02e45455e68fbaff4be2de1912ed5ad454512d5e11df952143b056157fe0c7e9"},{"id":"func/mailOf","name":"mailOf","line":51,"end_line":63,"hash":"0ec62c2244e0e2727d0d4e98dbc61f786a1e014bdeb01f39a01966b2c3a2f93c"},{"id":"func/countOf","name":"countOf","line":66,"end_line":72,"hash":"0d2f4363a3d949886a923f10779ae39233aa333ebce3c1e23437cf0cbc090ed5"},{"id":"func/idlerEvidence","name":"idlerEvidence","line":79,"end_line":114,"hash":"31d695bf3ff9e9a7e21ddb3dab9fa0c392bb80407fa84e5642112fc781fd8b37"},{"id":"func/boardRead","name":"boardRead","line":118,"end_line":123,"hash":"5de4d9e8c53b8b5620a333ff5d5c606cd01ad165738abe7b9af687c6c1ee8fca"},{"id":"func/inboxRead","name":"inboxRead","line":127,"end_line":132,"hash":"c8dd4e11c9f804a29376794afd8d110ff50d4d332aadbbd82a56cd2bbe2b99f0"}]}
// mutate4go-manifest-end
