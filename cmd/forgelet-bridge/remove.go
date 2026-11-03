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

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-11-03T00:47:13+01:00","module_hash":"b501d1296dd9be035077e181391b7414bab6c3975329b1f12eae6b843645d5e1","functions":[{"id":"func/remove","name":"remove","line":22,"end_line":47,"hash":"ae58242e284218f9ecf08826f918bcaea139a84fd25642530e3a9347f338d998"},{"id":"func/prepareRemoval","name":"prepareRemoval","line":49,"end_line":70,"hash":"26e0820e71ece7348e2d9c0f3681d65faa3f47f960699f6429f29c840b4700df"},{"id":"func/executeRemoval","name":"executeRemoval","line":72,"end_line":84,"hash":"ce99e95c862ceb51a20136ce6da901876588d02b08779106c588eb4b8e0295c3"},{"id":"func/writeRemovalResult","name":"writeRemovalResult","line":86,"end_line":91,"hash":"ef1e7f2549030fefdf8093c1f0569688b44ec8e8a072976c9db4fb3a9973e022"}]}
// mutate4go-manifest-end
