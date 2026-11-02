package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/unclebob/forgelet-bridge/internal/config"
	"github.com/unclebob/forgelet-bridge/internal/matrix"
	"github.com/unclebob/forgelet-bridge/internal/removal"
)

const removalUnset = "\x00"

// remove implements the adapter's two removal phases. Planning validates the
// configuration and saved ids before the bridge stops. Carrying it out makes
// the same validation again, connects Matrix, and returns one sanitized JSON
// object either way.
func remove(configPath, root string, plan bool, extra []string, output io.Writer) int {
	if len(extra) != 0 {
		return writeRemovalResult(output, removal.Result{
			Status: removal.Refused,
			Reason: "unsafe_root",
		})
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return writeRemovalResult(output, removal.Result{
			Status: removal.Refused,
			Reason: "configuration_unavailable",
		})
	}
	service := removal.Service{
		StatePath: filepath.Join(cfg.StateDir, "bridge-state.json"),
		Operator:  cfg.Operator,
		Config:    cfg,
	}
	loaded, err := service.Load()
	if err != nil {
		return writeRemovalResult(output, removal.Result{
			Status: removal.Refused,
			Reason: "incomplete_state",
		})
	}
	inspection := removal.Inspect(cfg, loaded, root)
	if plan {
		if inspection.Result.Status == removal.Ready {
			_, err := fmt.Fprintln(output, "ready")
			if err != nil {
				return 1
			}
			return 0
		}
		return writeRemovalResult(output, inspection.Result)
	}
	if inspection.Result.Status != removal.Ready {
		return writeRemovalResult(output, inspection.Result)
	}

	ctx := context.Background()
	client, err := matrix.Connect(ctx, cfg, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))
	if err != nil {
		return writeRemovalResult(output, removal.Result{
			Status: removal.Retryable,
			Reason: "matrix_unavailable",
		})
	}
	defer client.Close()
	service.Cleaner = removalCleaner(client)
	return writeRemovalResult(output, service.Remove(ctx, inspection))
}

// removalCleaner keeps the acceptance suite's failure boundary injectable
// without putting a fake Matrix server in the normal removal path.
func removalCleaner(client removal.Cleaner) removal.Cleaner {
	failure := os.Getenv("FORGELET_BRIDGE_REMOVE_TEST_FAILURE")
	stage, roomID, _ := strings.Cut(failure, "=")
	switch stage {
	case "relay_cleanup_failed":
		return failingCleaner{Cleaner: client, stage: stage, roomID: roomID}
	case "operator_removal_failed":
		return failingCleaner{Cleaner: client, stage: stage, roomID: roomID}
	case "bridge_membership_failed":
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
	if f.stage == "relay_cleanup_failed" && f.matches(roomID) {
		return errors.New("injected relay failure")
	}
	return f.Cleaner.ClearFacts(ctx, roomID)
}

func (f failingCleaner) RemoveOperator(ctx context.Context, roomID, operator string) error {
	if f.stage == "operator_removal_failed" && f.matches(roomID) {
		return errors.New("injected operator failure")
	}
	return f.Cleaner.RemoveOperator(ctx, roomID, operator)
}

func (f failingCleaner) LeaveAndForget(ctx context.Context, roomID string) error {
	if f.stage == "bridge_membership_failed" && f.matches(roomID) {
		return errors.New("injected membership failure")
	}
	return f.Cleaner.LeaveAndForget(ctx, roomID)
}

func (f failingCleaner) matches(roomID string) bool {
	return f.roomID == "" || f.roomID == roomID
}

func writeRemovalResult(output io.Writer, result removal.Result) int {
	if err := removal.WriteResult(output, result); err != nil {
		return 1
	}
	return 0
}
