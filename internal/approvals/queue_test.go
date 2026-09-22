package approvals

import (
	"reflect"
	"testing"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

func TestQueuePresentsPendingApprovalsInRelayForm(t *testing.T) {
	store := newStore(t, "forgelet-bridge")
	seed(t, store, "forgelet-bridge", "approval-1", pending)

	pendingApprovals, err := Queue{Store: store}.Pending()
	if err != nil {
		t.Fatalf("Pending: %v", err)
	}
	if len(pendingApprovals) != 1 {
		t.Fatalf("pending = %+v, want the one approval", pendingApprovals)
	}

	want := []relay.Approval{{
		Key:       Key("forgelet-bridge", "approval-1"),
		Project:   "forgelet-bridge",
		ID:        "approval-1",
		Card:      "phone-approvals",
		Gate:      "coder → refactorer",
		Artifacts: []string{"internal/bridge/bridge.go", "internal/relay/relay.go"},
	}}
	if !reflect.DeepEqual(pendingApprovals, want) {
		t.Errorf("pending = %+v, want %+v", pendingApprovals, want)
	}
}

func TestQueueDecidesThroughTheStore(t *testing.T) {
	store := newStore(t, "forgelet-bridge")
	seed(t, store, "forgelet-bridge", "approval-1", pending)
	queue := Queue{Store: store}

	if err := queue.Approve("forgelet-bridge", "approval-1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if remaining, err := store.Pending(); err != nil || len(remaining) != 0 {
		t.Errorf("pending = %+v, %v, want the approval approved", remaining, err)
	}

	seed(t, store, "forgelet-bridge", "approval-2", pending)
	if err := queue.SendBack("forgelet-bridge", "approval-2", "the total is wrong"); err != nil {
		t.Fatalf("SendBack: %v", err)
	}
	if remaining, err := store.Pending(); err != nil || len(remaining) != 0 {
		t.Errorf("pending = %+v, %v, want the approval sent back", remaining, err)
	}
}

func TestKeyKeepsProjectsApart(t *testing.T) {
	first := Key("forgelet-bridge", "approval-1")
	second := Key("saibill", "approval-1")

	if first == second {
		t.Errorf("key = %q, the same for two projects", first)
	}
	if first != "forgelet-bridge/approval-1" {
		t.Errorf("key = %q, want the project and the handoff", first)
	}
}
