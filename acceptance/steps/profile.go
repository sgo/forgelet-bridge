package steps

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type worldProfile struct {
	Backend       string `json:"backend"`
	WorkDir       string `json:"work_dir"`
	TotalMS       int64  `json:"total_ms"`
	SetupMS       int64  `json:"setup_ms"`
	BridgeStartMS int64  `json:"bridge_start_ms"`
	ScenarioMS    int64  `json:"scenario_ms"`
	TeardownMS    int64  `json:"teardown_ms"`
}

func (w *World) writeProfile(teardown time.Duration) {
	if w.profileDir == "" {
		return
	}
	w.teardownTime = teardown
	total := time.Since(w.started)
	scenario := total - w.setupTime - w.bridgeTime - w.teardownTime
	if scenario < 0 {
		scenario = 0
	}
	profile := worldProfile{
		Backend:       string(w.matrixBackend),
		WorkDir:       w.workDir,
		TotalMS:       total.Milliseconds(),
		SetupMS:       w.setupTime.Milliseconds(),
		BridgeStartMS: w.bridgeTime.Milliseconds(),
		ScenarioMS:    scenario.Milliseconds(),
		TeardownMS:    w.teardownTime.Milliseconds(),
	}
	data, err := json.Marshal(profile)
	if err != nil {
		w.log.Error("marshal acceptance profile", "error", err)
		return
	}
	if err := os.MkdirAll(w.profileDir, 0o755); err != nil {
		w.log.Error("create acceptance profile directory", "error", err)
		return
	}
	path := filepath.Join(w.profileDir, fmt.Sprintf("%s-%s.json", w.matrixBackend, filepath.Base(w.workDir)))
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		w.log.Error("write acceptance profile", "error", err)
	}
}
