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
	"github.com/unclebob/forgelet-bridge/internal/bridge"
)

// child is a child process the scenario started: a forge's dashboard stub, or
// the bridge. Every child is reaped by its own waiter, so the scenario can tell
// a clean stop from a crash, and one stop routine serves them all.
type child struct {
	cmd     *exec.Cmd
	log     *os.File
	done    chan struct{}
	waitErr error
}

// start launches a child and starts reaping it.
func start(cmd *exec.Cmd, log *os.File) (*child, error) {
	started := &child{cmd: cmd, log: log, done: make(chan struct{})}
	if err := cmd.Start(); err != nil {
		closeLog(log)
		return nil, err
	}
	go func() {
		started.waitErr = cmd.Wait()
		close(started.done)
	}()
	return started, nil
}

// stopped reports whether the child has already ended, and how.
func (c *child) stopped() (bool, error) {
	select {
	case <-c.done:
		return true, c.waitErr
	default:
		return false, nil
	}
}

// stop asks the child to stop, and kills it if it does not.
func (c *child) stop() {
	if c == nil || c.cmd == nil || c.cmd.Process == nil {
		return
	}
	_ = c.cmd.Process.Signal(os.Interrupt)
	select {
	case <-c.done:
	case <-time.After(10 * time.Second):
		_ = c.cmd.Process.Kill()
		<-c.done
	}
	closeLog(c.log)
}

func closeLog(log *os.File) {
	if log != nil {
		log.Close()
	}
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
	running, err := start(cmd, logFile)
	if err != nil {
		return err
	}
	w.stubs[name] = running
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
	started := time.Now()
	running, err := start(cmd, logFile)
	if err != nil {
		return err
	}
	w.bridge = running
	if err := w.waitForBridgeStart(ctx, running, started); err != nil {
		return err
	}
	return w.waitForOperator(ctx)
}

func (w *World) stopBridge() {
	if w.bridge == nil {
		return
	}
	running := w.bridge
	w.bridge = nil
	running.stop()
}

// waitForBridgeStart waits until the bridge process is really running: it has
// written a fresh status after this start. A bridge that dies on startup, or
// that never reaches its first tick, fails here instead of leaving the
// scenario silently asserting stale state.
func (w *World) waitForBridgeStart(ctx context.Context, running *child, started time.Time) error {
	if stopped, err := running.stopped(); stopped {
		return fmt.Errorf("the bridge stopped during startup: %v", err)
	}

	path := filepath.Join(w.stateDir, bridge.StatusName)
	return waitFor(ctx, "the bridge never reported a tick after starting", func() (bool, error) {
		if stopped, err := running.stopped(); stopped {
			return false, fmt.Errorf("the bridge stopped during startup: %v", err)
		}
		info, err := os.Stat(path)
		if err != nil {
			return false, nil
		}
		return info.ModTime().After(started), nil
	})
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
