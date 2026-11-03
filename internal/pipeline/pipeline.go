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

	_, block, found := strings.Cut(gate, "\n  "+stage+")\n")
	if !found {
		return fmt.Errorf("stage-gate.sh has no %s case", stage)
	}
	block, _, _ = strings.Cut(block, ";;")
	if !strings.Contains(block, command) {
		return fmt.Errorf("stage %s does not run %s", stage, command)
	}
	return nil
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-11-03T12:50:55+01:00","module_hash":"669a451343ff65deaf219573306d59a4807dc82b5d073008b96589965473bb34","functions":[{"id":"func/VerifySingleAgentGoProject","name":"VerifySingleAgentGoProject","line":12,"end_line":23,"hash":"9bb8ec35f6e06487da9a221455d67c0792627baf12ea6c8d596055ac8eddc750"},{"id":"func/VerifyProjectStage","name":"VerifyProjectStage","line":27,"end_line":48,"hash":"3fb8e2a1f839a3939fe0799e6e49d4494ace1e6a1f304ab1383e1939fcd7ba8d"}]}
// mutate4go-manifest-end
