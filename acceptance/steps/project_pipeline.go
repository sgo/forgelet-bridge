package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
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
	return verifySingleAgentGoProject(string(singleAgent), string(language), string(module))
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
	return verifyProjectStage(string(stages), string(gate),
		captures[1], captures[2], captures[3])
}

func verifySingleAgentGoProject(marker, language, module string) error {
	if strings.TrimSpace(marker) != "forgelet-bridge" {
		return fmt.Errorf("the project is not marked single-agent for forgelet-bridge")
	}
	if strings.TrimSpace(language) != "Go" {
		return fmt.Errorf("project language = %q, want Go", strings.TrimSpace(language))
	}
	if !strings.Contains(module, "module github.com/unclebob/forgelet-bridge") {
		return fmt.Errorf("go.mod does not declare the bridge module")
	}
	return nil
}

func verifyProjectStage(stages, gate, stage, command, log string) error {
	stageName := fmt.Sprintf(`:name %q`, stage)
	if !strings.Contains(stages, stageName) {
		return fmt.Errorf("stages.edn has no %s stage", stage)
	}
	if !strings.Contains(stages, `"scripts/stage-gate.sh"`) {
		return fmt.Errorf("stages.edn does not run through stage-gate.sh")
	}
	if log != "build/stages/"+stage+".log" {
		return fmt.Errorf("stage %s log = %q, want build/stages/%s.log", stage, log, stage)
	}

	start := strings.Index(gate, "\n  "+stage+")\n")
	if start < 0 {
		return fmt.Errorf("stage-gate.sh has no %s case", stage)
	}
	block := gate[start+1:]
	if end := strings.Index(block, ";;"); end >= 0 {
		block = block[:end]
	}
	if !strings.Contains(block, command) {
		return fmt.Errorf("stage %s does not run %s", stage, command)
	}
	return nil
}
