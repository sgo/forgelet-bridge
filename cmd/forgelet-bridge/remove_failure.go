package main

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/unclebob/forgelet-bridge/internal/removal"
)

// removalCleaner keeps the acceptance suite's failure boundary injectable
// without putting a fake Matrix server in the normal removal path.
func removalCleaner(client removal.Cleaner) removal.Cleaner {
	stage, roomID, _ := strings.Cut(os.Getenv("FORGELET_BRIDGE_REMOVE_TEST_FAILURE"), "=")
	switch stage {
	case "relay_cleanup_failed", "operator_removal_failed", "bridge_membership_failed":
		return failingCleaner{Cleaner: client, stage: stage, roomID: roomID}
	default:
		return client
	}
}

type failingCleaner struct {
	removal.Cleaner
	stage  string
	roomID string
}

func (f failingCleaner) ClearFacts(ctx context.Context, roomID string) error {
	return f.perform("relay_cleanup_failed", roomID, func() error {
		return f.Cleaner.ClearFacts(ctx, roomID)
	})
}

func (f failingCleaner) RemoveOperator(ctx context.Context, roomID, operator string) error {
	return f.perform("operator_removal_failed", roomID, func() error {
		return f.Cleaner.RemoveOperator(ctx, roomID, operator)
	})
}

func (f failingCleaner) LeaveAndForget(ctx context.Context, roomID string) error {
	return f.perform("bridge_membership_failed", roomID, func() error {
		return f.Cleaner.LeaveAndForget(ctx, roomID)
	})
}

func (f failingCleaner) perform(stage, roomID string, call func() error) error {
	if f.failsAt(stage, roomID) {
		return errors.New("injected removal failure")
	}
	return call()
}

func (f failingCleaner) failsAt(stage, roomID string) bool {
	return f.stage == stage && (f.roomID == "" || f.roomID == roomID)
}
