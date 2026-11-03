package steps

import (
	"context"
	"os"
	"path/filepath"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/internal/pipeline"
)

func singleAgentGoProject(_ context.Context, _ any, _ []string) error {
	root := fixtures.ProjectRoot()
	singleAgent, err := os.ReadFile(filepath.Join(root, ".swarmforge", "single-agent"))
	if err != nil {
		return err
	}
	language, err := os.ReadFile(filepath.Join(root, "swarmforge", "language.conf"))
	if err != nil {
		return err
	}
	module, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return err
	}
	return pipeline.VerifySingleAgentGoProject(string(singleAgent), string(language), string(module))
}

func projectStageRunsToolchain(_ context.Context, _ any, captures []string) error {
	root := fixtures.ProjectRoot()
	stages, err := os.ReadFile(filepath.Join(root, "stages.edn"))
	if err != nil {
		return err
	}
	gate, err := os.ReadFile(filepath.Join(root, "scripts", "stage-gate.sh"))
	if err != nil {
		return err
	}
	return pipeline.VerifyProjectStage(string(stages), string(gate),
		captures[1], captures[2], captures[3])
}
