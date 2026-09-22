package fixtures

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Dashboard is a forge's own dashboard, running against a fixture forge root.
// The suite uses it rather than a stand-in, so what the dashboard does with a
// chat request — the wake included — is the real thing.
type Dashboard struct {
	Root string
	URL  string

	cmd *exec.Cmd
	log *os.File
}

// StartDashboard prepares the forge root a dashboard needs, starts the forge's
// own dashboard on a free port, and waits until it announces its address. The
// dashboard types into the lieutenant's pane through its own tmux stub, which
// records what it typed instead of talking to a real terminal.
func StartDashboard(ctx context.Context, root string) (*Dashboard, error) {
	if err := prepareForgeRoot(root); err != nil {
		return nil, err
	}
	script := filepath.Join(ProjectRoot(), "swarmforge", "scripts", "pack_web.sh")
	if _, err := os.Stat(script); err != nil {
		return nil, fmt.Errorf("the forge's dashboard script is missing: %w", err)
	}
	logFile, err := os.Create(filepath.Join(root, "dashboard.log"))
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, script, "--serve", root)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "SWARMFORGE_TMUX_STUB="+wakeLog(root))
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return nil, err
	}

	dashboard := &Dashboard{Root: root, cmd: cmd, log: logFile}
	url, err := dashboard.waitForURL(ctx)
	if err != nil {
		dashboard.Stop()
		return nil, err
	}
	dashboard.URL = url
	return dashboard, nil
}

// Stop shuts the dashboard down.
func (d *Dashboard) Stop() {
	if d == nil || d.cmd == nil || d.cmd.Process == nil {
		return
	}
	_ = d.cmd.Process.Signal(os.Interrupt)
	done := make(chan struct{})
	go func() {
		_ = d.cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		_ = d.cmd.Process.Kill()
		<-done
	}
	if d.log != nil {
		d.log.Close()
	}
}

// Typed is what the dashboard typed into the lieutenant's pane.
func (d *Dashboard) Typed() ([]string, error) {
	data, err := os.ReadFile(wakeLog(d.Root))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var typed []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.TrimSpace(line) != "" {
			typed = append(typed, line)
		}
	}
	return typed, nil
}

// WokeWith reports whether the dashboard typed the given text into the pane.
func (d *Dashboard) WokeWith(text string) (bool, error) {
	typed, err := d.Typed()
	if err != nil {
		return false, err
	}
	for _, line := range typed {
		if strings.Contains(line, text) {
			return true, nil
		}
	}
	return false, nil
}

// Ask gives the dashboard a chat request the way its clients do.
func (d *Dashboard) Ask(ctx context.Context, text string) error {
	body, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, d.URL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(response.Body)
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("the dashboard refused the chat request: %s: %s", response.Status, strings.TrimSpace(string(data)))
	}
	return nil
}

func (d *Dashboard) waitForURL(ctx context.Context) (string, error) {
	path := filepath.Join(d.Root, filepath.FromSlash(".swarmforge/dashboard-url"))
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if data, err := os.ReadFile(path); err == nil {
			if url := strings.TrimSpace(string(data)); url != "" {
				return strings.TrimRight(url, "/"), nil
			}
		}
		if err := Sleep(ctx, 200*time.Millisecond); err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("the forge's dashboard did not announce itself; see %s", filepath.Join(d.Root, "dashboard.log"))
}

// wakeLog is where the dashboard's tmux stub records what it typed.
func wakeLog(root string) string {
	return filepath.Join(root, filepath.FromSlash(".swarmforge/tmux-stub.log"))
}

// prepareForgeRoot gives the forge root what its dashboard needs: the roles it
// serves, with a worktree each, and the tmux socket it types into.
func prepareForgeRoot(root string) error {
	stateDir := filepath.Join(root, ".swarmforge")
	if err := os.MkdirAll(filepath.Join(stateDir, "board"), 0o755); err != nil {
		return err
	}
	roles := [][]string{
		{"master", "master", root, "fixture-lieutenant", "Lieutenant", "codex", "task", "forward-only"},
		{"coder", "coder", filepath.Join(root, "worktrees", "coder"), "fixture-coder", "Coder", "codex", "task", "forward-only"},
		{"refactorer", "refactorer", filepath.Join(root, "worktrees", "refactorer"), "fixture-refactorer", "Refactorer", "codex", "task", "back-one"},
	}
	var lines []string
	for _, role := range roles {
		if err := os.MkdirAll(role[2], 0o755); err != nil {
			return err
		}
		lines = append(lines, strings.Join(role, "\t"))
	}
	if err := os.WriteFile(filepath.Join(stateDir, "roles.tsv"), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(stateDir, "tmux-socket"), []byte(filepath.Join(stateDir, "tmux.sock")+"\n"), 0o644)
}
