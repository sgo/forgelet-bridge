package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"maunium.net/go/mautrix/event"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
)

func adapterPlansProject(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	target, err := w.declaredForge(captures[3])
	if err != nil {
		return err
	}
	return w.invokeProjectPlan(captures[1], target.Root(), captures[2])
}

func adapterAskedToPlanProject(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	if w.projectRemoval == nil {
		return fmt.Errorf("the scenario has not recorded managed projects")
	}
	if captures[2] == "extra-arguments" {
		return w.invokeProjectPlanArgs(captures[1], w.projectRemoval.targetRoot,
			w.projectRemoval.targetName, "extra")
	}
	rootArgs, err := w.projectPlanRootArguments(captures[2])
	if err != nil {
		return err
	}
	project, err := w.projectRemovalProjectArgument(captures[3])
	if err != nil {
		return err
	}
	return w.invokeProjectPlanArgs(captures[1], append(rootArgs, project)...)
}

func (w *World) projectPlanRootArguments(argumentCase string) ([]string, error) {
	if argumentCase == "target" {
		return []string{w.projectRemoval.targetRoot}, nil
	}
	return w.projectRemovalRootArguments(argumentCase)
}

func bridgeAnswersPlan(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	if w.projectRemoval == nil {
		return fmt.Errorf("the scenario has not recorded managed projects")
	}
	return w.watchProjectPlanBinary(captures[1])
}

func watchedProjectPlanMustNotRun(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	if w.projectRemoval == nil {
		return fmt.Errorf("the scenario has not recorded managed projects")
	}
	return w.watchProjectPlanBinary("ready")
}

func (w *World) watchProjectPlanBinary(binaryCase string) error {
	response, exitCode, err := projectPlanResponse(binaryCase)
	if err != nil {
		return err
	}
	planDir := filepath.Join(w.workDir, "project-plan")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		return err
	}
	responsePath := filepath.Join(planDir, "response")
	logPath := filepath.Join(planDir, "bridge.log")
	if err := os.WriteFile(responsePath, []byte(response), 0o644); err != nil {
		return err
	}
	script := fmt.Sprintf(`#!/usr/bin/env zsh
set -euo pipefail
print -r -- "$*" >> %s
cat %s
exit %d
`, shellQuote(logPath), shellQuote(responsePath), exitCode)
	spy := filepath.Join(planDir, "forgelet-bridge")
	if err := os.WriteFile(spy, []byte(script), 0o755); err != nil {
		return err
	}
	w.projectRemoval.planBridgeLog = logPath
	w.adapterEnv = append(w.adapterEnv, adapterBinaryEnv+"="+spy)
	return nil
}

func projectPlanResponse(binaryCase string) (string, int, error) {
	switch binaryCase {
	case "ready":
		return "ready\n", 0, nil
	case "removed":
		return `{"status":"removed"}` + "\n", 0, nil
	case "nothing-to-remove":
		return `{"status":"nothing_to_remove"}` + "\n", 0, nil
	case "retryable":
		return `{"status":"retryable","reason":"matrix_unavailable"}` + "\n", 0, nil
	case "refused":
		return `{"status":"refused","reason":"incomplete_project_state"}` + "\n", 0, nil
	case "extra-field":
		return `{"status":"refused","reason":"incomplete_project_state","path":"/private/forge","user":"@operator:example.org"}` + "\n", 0, nil
	case "malformed":
		return "{broken\n", 0, nil
	case "empty":
		return "", 0, nil
	case "unknown-status":
		return `{"status":"mystery","reason":"unknown"}` + "\n", 0, nil
	case "nonzero":
		return `{"status":"removed"}` + "\n", 13, nil
	default:
		return "", 0, fmt.Errorf("unknown project plan binary case %q", binaryCase)
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func (w *World) invokeProjectPlan(servedName, root, project string, extra ...string) error {
	return w.invokeProjectPlanArgs(servedName, append([]string{root, project}, extra...)...)
}

func (w *World) invokeProjectPlanArgs(servedName string, args ...string) error {
	served, err := w.declaredForge(servedName)
	if err != nil {
		return err
	}
	if w.projectRemoval == nil {
		return fmt.Errorf("the scenario has not recorded managed projects")
	}
	w.projectRemoval.stateBefore, err = os.ReadFile(w.removalStatePath())
	if err != nil {
		return err
	}
	w.projectRemoval.configBefore, err = os.ReadFile(w.bridgeConfigOf(served.Root()))
	if err != nil {
		return err
	}
	w.projectRemoval.targetBefore, err = snapshotRemovalTree(w.projectRemoval.targetRoot)
	if err != nil {
		return err
	}
	w.projectRemoval.planDone = false

	ctx, cancel := stepContext()
	defer cancel()
	if _, runErr := w.adapterCommand(ctx, append([]string{"remove-project-plan"}, args...)...); runErr != nil {
		return fmt.Errorf("project removal plan exited with %v: %s", runErr, w.adapterOutput)
	}
	output := strings.TrimSpace(w.adapterStdout)
	if output == "ready" {
		w.projectRemoval.planStatus = "ready"
		w.projectRemoval.planReason = ""
	} else {
		var report removalReport
		if err := json.Unmarshal([]byte(output), &report); err != nil {
			return fmt.Errorf("project removal plan did not return ready or one JSON object: %q: %w", output, err)
		}
		w.projectRemoval.planStatus = report.Status
		w.projectRemoval.planReason = report.Reason
	}
	w.projectRemoval.planRawOutput = w.adapterStdout
	w.projectRemoval.rawOutput = w.adapterStdout
	w.projectRemoval.planDone = true
	return nil
}

func projectPlanResultReports(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	if w.projectRemoval == nil || !w.projectRemoval.planDone {
		return fmt.Errorf("the adapter has not answered a project removal plan request")
	}
	if w.projectRemoval.planStatus != captures[1] {
		return fmt.Errorf("project removal plan status = %q, want %q; output: %s",
			w.projectRemoval.planStatus, captures[1], w.projectRemoval.planRawOutput)
	}
	wantReason := ""
	if len(captures) > 2 {
		wantReason = captures[2]
	}
	if w.projectRemoval.planReason != wantReason {
		return fmt.Errorf("project removal plan reason = %q, want %q; output: %s",
			w.projectRemoval.planReason, wantReason, w.projectRemoval.planRawOutput)
	}
	return nil
}

func projectPlanLeftBridgeRunning(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	output, err := w.adapterCommand(ctx, "status")
	if err != nil {
		return err
	}
	if !strings.HasPrefix(strings.TrimSpace(output), "running ") {
		return fmt.Errorf("the project removal plan left the bridge stopped:\n%s", output)
	}
	return nil
}

func projectPlanChangedNothing(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	stateAfter, err := os.ReadFile(w.removalStatePath())
	if err != nil {
		return err
	}
	if string(stateAfter) != string(w.projectRemoval.stateBefore) {
		return fmt.Errorf("the project removal plan changed bridge state")
	}
	configAfter, err := os.ReadFile(w.configPath)
	if err != nil {
		return err
	}
	if string(configAfter) != string(w.projectRemoval.configBefore) {
		return fmt.Errorf("the project removal plan changed bridge configuration")
	}
	treeAfter, err := snapshotRemovalTree(w.projectRemoval.targetRoot)
	if err != nil {
		return err
	}
	if !sameSnapshot(w.projectRemoval.targetBefore, treeAfter) {
		return fmt.Errorf("the project removal plan changed forge or project files")
	}

	ctx, cancel := stepContext()
	defer cancel()
	observer, err := w.user(ctx, removalObserver)
	if err != nil {
		return err
	}
	for _, roomID := range []string{
		w.projectRemoval.targetRoom,
		w.projectRemoval.siblingRoom,
	} {
		for _, userID := range []string{w.operatorID, w.bridgeUserID} {
			membership, err := observer.Membership(ctx, roomID, userID)
			if err != nil {
				return err
			}
			if membership != string(event.MembershipJoin) &&
				membership != string(event.MembershipInvite) {
				return fmt.Errorf("the project removal plan changed %s membership in %s to %s",
					userID, roomID, membership)
			}
		}
	}
	return nil
}

func projectPlanInvokedBridge(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	body, err := os.ReadFile(w.projectRemoval.planBridgeLog)
	if err != nil {
		return fmt.Errorf("the planning adapter did not invoke the bridge: %w", err)
	}
	line := string(body)
	for _, expected := range []string{
		"--remove-project-plan", w.projectRemoval.targetRoot,
		w.projectRemoval.targetName,
	} {
		if !strings.Contains(line, expected) {
			return fmt.Errorf("the planning adapter invocation does not name %q: %s", expected, line)
		}
	}
	return nil
}

func projectPlanDidNotInvokeBridge(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	if _, err := os.Stat(w.projectRemoval.planBridgeLog); err == nil {
		return fmt.Errorf("a refused project removal plan invoked the bridge")
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func projectDeploymentConformanceCheck(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	installed, err := w.installAdapter()
	if err != nil {
		return err
	}
	ctx, cancel := stepContext()
	defer cancel()
	command := exec.CommandContext(ctx,
		filepath.Join(fixtures.ProjectRoot(), "scripts", "check-adapter-conformance.sh"),
		installed)
	command.Dir = fixtures.ProjectRoot()
	output, err := command.CombinedOutput()
	w.adapterOutput = string(output)
	if err != nil {
		return fmt.Errorf("adapter conformance failed: %w: %s", err, output)
	}
	return nil
}

func installedBridgeAdapterExposesHostCommands(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	for _, command := range []string{"remove-project-plan", "remove-project"} {
		if !strings.Contains(w.adapterOutput, command) {
			return fmt.Errorf("adapter conformance did not name %s: %s", command, w.adapterOutput)
		}
	}
	return nil
}
