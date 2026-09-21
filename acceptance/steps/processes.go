package steps

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
)

// stub is a running dashboard stub, the fixture stand-in for a forge's
// dashboard.
type stub struct {
	cmd *exec.Cmd
	log *os.File
}

// bridgeProcess is the running bridge.
type bridgeProcess struct {
	cmd *exec.Cmd
	log *os.File
}

// startDashboard runs the forge's dashboard stub.
func (w *World) startDashboard(name string) error {
	if _, ok := w.stubs[name]; ok {
		return nil
	}
	store, err := w.forge(context.Background(), name)
	if err != nil {
		return err
	}
	binary, err := buildHelper("forge-dashboard-stub", "./cmd/forge-dashboard-stub")
	if err != nil {
		return err
	}
	logFile, err := os.Create(filepath.Join(w.workDir, name+"-dashboard.log"))
	if err != nil {
		return err
	}
	cmd := exec.Command(binary, "--root", store.Root())
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return err
	}
	w.stubs[name] = &stub{cmd: cmd, log: logFile}
	return nil
}

// startBridge runs the bridge process.
func (w *World) startBridge(ctx context.Context) error {
	if w.bridge != nil {
		return nil
	}
	if w.configPath == "" {
		if err := w.configure(ctx); err != nil {
			return err
		}
	}
	binary, err := w.bridgeBinary()
	if err != nil {
		return err
	}
	logFile, err := os.OpenFile(filepath.Join(w.workDir, "bridge.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	cmd := exec.Command(binary, "--config", w.configPath, "--interval", "200ms")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return err
	}
	w.bridge = &bridgeProcess{cmd: cmd, log: logFile}
	return w.waitForOperator(ctx)
}

func (w *World) stopBridge() {
	if w.bridge == nil {
		return
	}
	stopProcess(w.bridge.cmd, w.bridge.log)
	w.bridge = nil
}

func (w *World) bridgeBinary() (string, error) {
	if w.binaryPath != "" {
		return w.binaryPath, nil
	}
	binary, err := buildHelper("forgelet-bridge", "./cmd/forgelet-bridge")
	if err != nil {
		return "", err
	}
	w.binaryPath = binary
	return binary, nil
}

var (
	buildMu     sync.Mutex
	builtBinary = map[string]string{}
)

// buildHelper builds a command once per test process and returns its path.
func buildHelper(name, pkg string) (string, error) {
	buildMu.Lock()
	defer buildMu.Unlock()
	if path, ok := builtBinary[name]; ok {
		return path, nil
	}
	path := filepath.Join(fixtures.ProjectRoot(), "build", "acceptance", "bin", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	cmd := exec.Command("go", "build", "-tags", "goolm", "-o", path, pkg)
	cmd.Dir = fixtures.ProjectRoot()
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("build %s: %w: %s", name, err, out)
	}
	builtBinary[name] = path
	return path, nil
}

// stopProcess asks a child process to stop, and kills it if it does not.
func stopProcess(cmd *exec.Cmd, logFile *os.File) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Signal(os.Interrupt)
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		<-done
	}
	if logFile != nil {
		logFile.Close()
	}
}
