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

// removeProjectCommand implements the adapter's planning and apply phases for
// one managed project. Planning validates configuration, filesystem identity
// and saved room ids before a running bridge may be stopped.
func removeProjectCommand(configPath, root, project string, plan bool, extra []string, output io.Writer) int {
	if len(extra) != 0 {
		return writeProjectRefusal(output, "unsafe_root")
	}
	service, inspection := prepareProjectRemoval(configPath, root, project)
	if plan {
		return writeProjectPlan(output, inspection.Result)
	}
	if inspection.Result.Status != removal.Ready {
		return writeRemovalResult(output, inspection.Result)
	}
	return writeRemovalResult(output, executeProjectRemoval(service, inspection))
}

func writeProjectPlan(output io.Writer, result removal.Result) int {
	if result.Status != removal.Ready {
		return writeRemovalResult(output, result)
	}
	if _, err := fmt.Fprintln(output, "ready"); err != nil {
		return 1
	}
	return 0
}

func writeProjectRefusal(output io.Writer, reason string) int {
	return writeRemovalResult(output, removal.Result{Status: removal.Refused, Reason: reason})
}

func prepareProjectRemoval(configPath, root, project string) (removal.Service, removal.ProjectInspection) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return removal.Service{}, removal.ProjectInspection{Result: removal.Result{
			Status: removal.Refused,
			Reason: "configuration_unavailable",
		}}
	}
	catalog := forgeletProjectCatalog{}
	service := removal.Service{
		StatePath: filepath.Join(cfg.StateDir, "bridge-state.json"),
		Operator:  cfg.Operator,
		Config:    cfg,
		Projects:  catalog,
	}
	loaded, err := service.Load()
	if err != nil {
		return removal.Service{}, removal.ProjectInspection{Result: removal.Result{
			Status: removal.Refused,
			Reason: "incomplete_project_state",
		}}
	}
	return service, removal.InspectProject(cfg, loaded, root, project, catalog)
}

func executeProjectRemoval(service removal.Service, inspection removal.ProjectInspection) removal.Result {
	ctx := context.Background()
	client, err := matrix.Connect(ctx, service.Config, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))
	if err != nil {
		return removal.Result{
			Status: removal.Retryable,
			Reason: "matrix_unavailable",
		}
	}
	defer client.Close()
	service.ProjectCleaner = projectRemovalCleaner(client)
	return service.RemoveProject(ctx, inspection)
}

type forgeletProjectCatalog struct{}

func (forgeletProjectCatalog) ForgeExists(root string) (bool, error) {
	info, err := os.Stat(filepath.Join(root, ".swarmforge"))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

func (forgeletProjectCatalog) ManagedProject(root, name string) (bool, error) {
	info, err := os.Stat(filepath.Join(root, "projects", name, ".swarmforge", "single-agent"))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return info.Mode().IsRegular(), nil
}
