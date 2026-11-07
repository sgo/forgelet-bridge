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

func projectRemovalCleaner(client removal.ProjectCleaner) removal.ProjectCleaner {
	stage, roomID, _ := strings.Cut(os.Getenv("FORGELET_BRIDGE_REMOVE_TEST_FAILURE"), "=")
	switch stage {
	case "relay_cleanup_failed", "operator_removal_failed", "bridge_membership_failed":
		return failingProjectCleaner{ProjectCleaner: client, stage: stage, roomID: roomID}
	default:
		return client
	}
}

type failingCleaner struct {
	removal.Cleaner
	stage  string
	roomID string
}

type failingProjectCleaner struct {
	removal.ProjectCleaner
	stage  string
	roomID string
}

func (f failingProjectCleaner) ClearProjectFacts(ctx context.Context, projectRoomID, activityRoomID, approvalsRoomID, project string) error {
	if f.failsAt("relay_cleanup_failed", projectRoomID) {
		return errors.New("injected project removal failure")
	}
	return f.ProjectCleaner.ClearProjectFacts(ctx, projectRoomID, activityRoomID, approvalsRoomID, project)
}

func (f failingProjectCleaner) RemoveOperator(ctx context.Context, roomID, operator string) error {
	if f.failsAt("operator_removal_failed", roomID) {
		return errors.New("injected project removal failure")
	}
	return f.ProjectCleaner.RemoveOperator(ctx, roomID, operator)
}

func (f failingProjectCleaner) LeaveAndForget(ctx context.Context, roomID string) error {
	if f.failsAt("bridge_membership_failed", roomID) {
		return errors.New("injected project removal failure")
	}
	return f.ProjectCleaner.LeaveAndForget(ctx, roomID)
}

func (f failingProjectCleaner) failsAt(stage, roomID string) bool {
	return f.stage == stage && (f.roomID == "" || f.roomID == roomID)
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

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-11-03T00:48:20+01:00","module_hash":"0c58abaf9e89dfd5ed6d99b1047628eb3075fc5f802d2badfc38e5cc860a073e","functions":[{"id":"func/removalCleaner","name":"removalCleaner","line":14,"end_line":22,"hash":"595ae850e46c6ee86652cd418585ea7c36afddaadf353bd13c7934bca33732c3"},{"id":"func/failingCleaner.ClearFacts","name":"failingCleaner.ClearFacts","line":30,"end_line":34,"hash":"92c58f8c14b8afe9841aa3bf3ddf0eed2116d7a256bca81229f21e0a87562ae9"},{"id":"func/failingCleaner.RemoveOperator","name":"failingCleaner.RemoveOperator","line":36,"end_line":40,"hash":"24c436c1fe6ee2bf541a6fae40a010440d0d64f329fa6161cd217d1c5042295c"},{"id":"func/failingCleaner.LeaveAndForget","name":"failingCleaner.LeaveAndForget","line":42,"end_line":46,"hash":"5254fc2f95a7ba4b917e8a33b343a5fb6077a2077753ccb030ebe058ffcf5f17"},{"id":"func/failingCleaner.perform","name":"failingCleaner.perform","line":48,"end_line":53,"hash":"bac3f247588b038417dcb4bd86296e9f4085b676e3c60a771cc8211e2521a8dd"},{"id":"func/failingCleaner.failsAt","name":"failingCleaner.failsAt","line":55,"end_line":57,"hash":"055e3f64e519d121f5f9a73d8154c982d3afa7b35906d846e118d19a20da6597"}]}
// mutate4go-manifest-end
