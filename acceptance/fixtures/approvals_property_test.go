//go:build property

package fixtures

import (
	"math/rand"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/quick"
)

// TestPropertyParseNamesTheHandover checks what the dashboard double reports
// about a handoff: who handed the work over and to whom, the card and its task
// id, and the changed files the handoff lists. The gate the operator reads is
// composed from those roles by the bridge's dashboard client, which pins that
// in its own property.
func TestPropertyParseNamesTheHandover(t *testing.T) {
	property := func(role, to, card, taskID string, artifacts []string) bool {
		handoff := strings.Join([]string{
			"id: approval-1",
			"to: " + to,
			"task_id: " + taskID,
			"task: " + card,
			"role: " + role,
			"artifacts: " + strings.Join(artifacts, ", "),
			"",
			"Re-read your role and constitution.",
			"",
		}, "\n")

		approval := parse(project, filepath.Join("/pending", "approval-1.handoff"), handoff)

		wantTaskID := taskID
		if wantTaskID == "" {
			wantTaskID = card
		}
		return approval.ID == "approval-1" &&
			approval.Project == project &&
			approval.From == strings.TrimSpace(role) &&
			approval.To == to &&
			approval.Card == card &&
			approval.TaskID == wantTaskID &&
			reflect.DeepEqual(approval.Artifacts, artifacts)
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 300,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randomName(rnd, true))
			values[1] = reflect.ValueOf(randomName(rnd, false))
			values[2] = reflect.ValueOf(randomName(rnd, false))
			values[3] = reflect.ValueOf(randomName(rnd, true))
			values[4] = reflect.ValueOf(randomArtifacts(rnd))
		},
	}); err != nil {
		t.Error(err)
	}
}

// TestPropertyParseIgnoresPaddedHeaders checks that padding in the handoff
// cannot reach the operator's phone: a handoff whose headers are padded reads
// exactly like the same handoff without the padding.
func TestPropertyParseIgnoresPaddedHeaders(t *testing.T) {
	property := func(role, to, card, artifacts string) bool {
		tidy := parse(project, approvalPath, handoff(role, to, card, artifacts))
		padded := parse(project, approvalPath, handoff(
			"  "+role+"  ", "  "+to+"  ", "  "+card+"  ", "  "+artifacts+"  "))
		return reflect.DeepEqual(tidy, padded)
	}
	if err := quick.Check(property, nil); err != nil {
		t.Error(err)
	}
}

// approvalPath is where the properties' handoffs sit.
var approvalPath = filepath.Join("/pending", "approval-1.handoff")

// handoff is a pending approval file with the given headers.
func handoff(role, to, card, artifacts string) string {
	return strings.Join([]string{
		"id: approval-1",
		"to: " + to,
		"task: " + card,
		"role: " + role,
		"artifacts: " + artifacts,
		"",
		"Re-read your role and constitution.",
		"",
	}, "\n")
}

// project is the project the properties' handoffs belong to.
const project = "forgelet-bridge"

// randomName is a role, a card, or a task id of the shape a handoff carries:
// one word, because a name with a comma in it is a list, not a name. It may be
// empty when the field is one a handoff can leave out.
func randomName(rnd *rand.Rand, mayBeEmpty bool) string {
	names := []string{"coder", "refactorer", "architect", "phone-approvals", "20260922T124152671989Z-phone-approvals"}
	if mayBeEmpty && rnd.Intn(3) == 0 {
		return ""
	}
	return names[rnd.Intn(len(names))]
}

// randomArtifacts is the changed-file list a handoff carries.
func randomArtifacts(rnd *rand.Rand) []string {
	files := []string{"internal/bridge/bridge.go", "internal/relay/relay.go", "README.md"}
	count := rnd.Intn(len(files) + 1)
	if count == 0 {
		return nil
	}
	return append([]string(nil), files[:count]...)
}
