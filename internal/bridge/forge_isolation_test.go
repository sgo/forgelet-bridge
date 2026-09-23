package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

// statusOf is the status the bridge last wrote.
func statusOf(t *testing.T, built *Bridge) Status {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(filepath.Dir(built.statePath), StatusName))
	if err != nil {
		t.Fatalf("read the bridge's status: %v", err)
	}
	var status Status
	if err := json.Unmarshal(data, &status); err != nil {
		t.Fatalf("parse the bridge's status: %v", err)
	}
	return status
}

// twoForges is a bridge that serves two forges, one of which is broken.
func twoForges(t *testing.T) (*Bridge, *fakeRooms, *fakeStore, *fakeStore) {
	t.Helper()
	healthy := &fakeStore{requests: []relay.Request{{ID: "req-a", Body: "is the build green?"}}}
	sick := &fakeStore{readErr: errors.New("the forge is not there")}
	rooms := &fakeRooms{}
	built, _ := newTestBridge(t, rooms, map[string]ForgeStore{
		"/forges/forge-a": healthy,
		"/forges/forge-b": sick,
	}, "/forges/forge-a", "/forges/forge-b")
	return built, rooms, healthy, sick
}

func TestAForgeThatCannotBeReachedDoesNotQuietTheOthers(t *testing.T) {
	built, rooms, _, _ := twoForges(t)

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	if !sentBodyTo(rooms, "!room-forge-a", "is the build green?") {
		t.Errorf("sent = %+v, want the healthy forge's chat carried into its room", rooms.sentMessages())
	}
	status := statusOf(t, built)
	if len(status.UnhappyForges) != 1 || status.UnhappyForges[0] != "forge-b" {
		t.Errorf("unhappy forges = %v, want only the forge that could not be reached", status.UnhappyForges)
	}
	if len(status.ReachedForges) != 1 || status.ReachedForges[0] != "forge-a" {
		t.Errorf("reached forges = %v, want the forge still being carried", status.ReachedForges)
	}
	if !strings.Contains(status.LastError, "the forge is not there") {
		t.Errorf("last error = %q, want the reason the sick forge is unhappy", status.LastError)
	}
}

func TestAForgeThatIsServedAgainStopsBeingNamed(t *testing.T) {
	built, rooms, _, sick := twoForges(t)
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick: %v", err)
	}

	sick.mu.Lock()
	sick.readErr = nil
	sick.requests = []relay.Request{{ID: "req-b", Body: "retry the invoice"}}
	sick.mu.Unlock()
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick: %v", err)
	}

	if !sentBodyTo(rooms, "!room-forge-b", "retry the invoice") {
		t.Errorf("sent = %+v, want the healed forge's chat carried at last", rooms.sentMessages())
	}
	if status := statusOf(t, built); len(status.UnhappyForges) != 0 {
		t.Errorf("unhappy forges = %v, want none once the forge is served again", status.UnhappyForges)
	}
	if status := statusOf(t, built); len(status.ReachedForges) != 2 {
		t.Errorf("reached forges = %v, want every configured forge once both are served", status.ReachedForges)
	}
}

func TestAForgeThatRefusesAnActionIsNamedWhileTheOtherWorks(t *testing.T) {
	approvals := &fakeApprovals{
		pending: []relay.Approval{isolatedApproval()},
		errors:  map[string]error{"forgelet-bridge/approval-1": errors.New("the forge is not there")},
	}
	healthy := &fakeStore{requests: []relay.Request{{ID: "req-a", Body: "is the build green?"}}}
	rooms := &fakeRooms{}
	built, _ := newTestBridgeWithStores(t, rooms,
		map[string]ForgeStore{"/forges/forge-a": healthy, "/forges/forge-b": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": &fakeApprovals{}, "/forges/forge-b": approvals},
		map[string]BoardStore{"/forges/forge-a": &fakeBoard{}, "/forges/forge-b": &fakeBoard{}},
		"/forges/forge-a", "/forges/forge-b")
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick: %v", err)
	}
	approval := built.State().Relay.Approvals[isolatedApproval().Key].MessageID
	rooms.pushReaction(relay.Reaction{
		RoomID: "!approvals-forge-b", Sender: operator, Key: relay.ApproveReaction, TargetEventID: approval,
	})

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick: %v", err)
	}

	status := statusOf(t, built)
	if len(status.UnhappyForges) != 1 || status.UnhappyForges[0] != "forge-b" {
		t.Errorf("unhappy forges = %v, want only the forge that refused the action", status.UnhappyForges)
	}
	if !sentBodyTo(rooms, "!room-forge-a", "is the build green?") {
		t.Errorf("sent = %+v, want the other forge still working", rooms.sentMessages())
	}
}

// isolatedApproval is one approval waiting in a forge the bridge can reach but
// which refuses to carry it out. Its key carries the forge, the way the forge's
// own dashboard names it, so the other forge's bookkeeping cannot be mistaken
// for it.
func isolatedApproval() relay.Approval {
	return relay.Approval{
		Key:       "/forges/forge-b/forgelet-bridge/approval-1",
		Project:   "forgelet-bridge",
		ID:        "approval-1",
		Card:      "one-sick-forge-does-not-quiet-the-others",
		Gate:      "coder → refactorer",
		Artifacts: []string{"internal/bridge/bridge.go"},
	}
}

// sentBodyTo reports whether the bridge sent a body into a room.
func sentBodyTo(rooms *fakeRooms, roomID, body string) bool {
	for _, sent := range rooms.sentMessages() {
		if sent.roomID == roomID && sent.body == body {
			return true
		}
	}
	return false
}

func TestTheStartupReportIsWrittenOnce(t *testing.T) {
	// The report belongs to startup: a tick that named the forges again every
	// time would fill the log the operator and the adapter read.
	var logged bytes.Buffer
	rooms := &fakeRooms{}
	cfg := newTestConfig(t, "/forges/forge-a")
	built, err := New(cfg, rooms,
		map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": &fakeApprovals{}},
		map[string]ClarificationStore{"/forges/forge-a": &fakeClarifications{}},
		map[string]BoardStore{"/forges/forge-a": &fakeBoard{}},
		slog.New(slog.NewTextHandler(&logged, nil)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	report := "the forges the bridge serves"
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick: %v", err)
	}
	if got := strings.Count(logged.String(), report); got != 1 {
		t.Errorf("the startup report was written %d times on the first tick, want once", got)
	}
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick: %v", err)
	}
	if got := strings.Count(logged.String(), report); got != 1 {
		t.Errorf("the startup report was written %d times over two ticks, want once", got)
	}
}
