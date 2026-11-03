// Package pipeline verifies that the single-agent project is described by its
// real Go toolchain rather than a generated template.
package pipeline

import (
	"fmt"
	"strings"
)

// VerifySingleAgentGoProject checks the project marker, language declaration,
// and module coordinate.
func VerifySingleAgentGoProject(marker, language, module string) error {
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

// VerifyProjectStage checks that a stage exists, runs through the stage gate,
// and names the command and evidence log the feature expects.
func VerifyProjectStage(stages, gate, stage, command, log string) error {
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
	block, _, _ = strings.Cut(block, ";;")
	if !strings.Contains(block, command) {
		return fmt.Errorf("stage %s does not run %s", stage, command)
	}
	return nil
}
