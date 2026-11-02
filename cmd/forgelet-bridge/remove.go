package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

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
	service, inspection, refusal := prepareRemoval(configPath, root)
	if refusal.Status != "" {
		return writeRemovalResult(output, refusal)
	}
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
	return writeRemovalResult(output, executeRemoval(service, inspection))
}

func prepareRemoval(configPath, root string) (removal.Service, removal.Inspection, removal.Result) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return removal.Service{}, removal.Inspection{}, removal.Result{
			Status: removal.Refused,
			Reason: "configuration_unavailable",
		}
	}
	service := removal.Service{
		StatePath: filepath.Join(cfg.StateDir, "bridge-state.json"),
		Operator:  cfg.Operator,
		Config:    cfg,
	}
	loaded, err := service.Load()
	if err != nil {
		return removal.Service{}, removal.Inspection{}, removal.Result{
			Status: removal.Refused,
			Reason: "incomplete_state",
		}
	}
	return service, removal.Inspect(cfg, loaded, root), removal.Result{}
}

func executeRemoval(service removal.Service, inspection removal.Inspection) removal.Result {
	ctx := context.Background()
	client, err := matrix.Connect(ctx, service.Config, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))
	if err != nil {
		return removal.Result{
			Status: removal.Retryable,
			Reason: "matrix_unavailable",
		}
	}
	defer client.Close()
	service.Cleaner = removalCleaner(client)
	return service.Remove(ctx, inspection)
}

func writeRemovalResult(output io.Writer, result removal.Result) int {
	if err := removal.WriteResult(output, result); err != nil {
		return 1
	}
	return 0
}
