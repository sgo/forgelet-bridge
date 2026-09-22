//go:build property

package bridge

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"testing/quick"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

// TestPropertyAPendingActionIsVisible checks what the phone and the dashboard
// read while the bridge is stuck: work the forge refused is counted, and a tick
// with work still waiting is never called idle, so a stuck bridge does not look
// like a quiet one.
func TestPropertyAPendingActionIsVisible(t *testing.T) {
	property := func(refuse bool) bool {
		store := &fakeApprovals{pending: []relay.Approval{approvalNamed("approval-1", "card-one")}, errors: map[string]error{}}
		if refuse {
			store.errors["forgelet-bridge/approval-1"] = fmt.Errorf("the forge is not there")
		}
		rooms := &fakeRooms{}
		built, cfg := newTestBridgeWithStores(t, rooms,
			map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
			map[string]ApprovalStore{"/forges/forge-a": store},
			map[string]BoardStore{"/forges/forge-a": &fakeBoard{}}, "/forges/forge-a")
		if err := built.Tick(context.Background()); err != nil {
			return false
		}
		messageID := built.State().Relay.Approvals["forgelet-bridge/approval-1"].MessageID
		rooms.pushReaction(relay.Reaction{
			RoomID: "!approvals-forge-a", Sender: operator, Key: relay.ApproveReaction, TargetEventID: messageID,
		})
		if err := built.Tick(context.Background()); err != nil {
			return false
		}

		status := readStatus(t, filepath.Join(cfg.StateDir, StatusName))
		if refuse {
			// The work is still owed, so the tick that could not carry it out
			// is not idle: a stuck bridge does not look like a quiet one.
			return status.Pending > 0 && !status.Idle
		}
		return status.Pending == 0
	}
	if err := quick.Check(property, &quick.Config{MaxCount: 4}); err != nil {
		t.Error(err)
	}
}

// approvalNamed is a waiting approval of one card. The cards differ between
// approvals because the fake room names a message after its text, so two
// approvals of one card would share a message id.
func approvalNamed(id, card string) relay.Approval {
	return relay.Approval{
		Key:       "forgelet-bridge/" + id,
		Project:   "forgelet-bridge",
		ID:        id,
		Card:      card,
		Gate:      "coder → refactorer",
		Artifacts: []string{"internal/bridge/bridge.go"},
	}
}
