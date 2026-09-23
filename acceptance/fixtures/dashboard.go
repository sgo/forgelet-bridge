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
	cmd.Env = append(os.Environ(),
		"SWARMFORGE_TMUX_STUB="+wakeLog(root),
		// Whatever the dashboard does with git, it does inside the fixture:
		// git must not walk up out of it into the worktree running the suite.
		"GIT_CEILING_DIRECTORIES="+filepath.Dir(root),
	)
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
	if d == nil {
		return
	}
	stopProcess(d.cmd, d.log, 15*time.Second)
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

// prepareForgeRoot gives the forge root what its dashboard needs: its own git
// world, the roles it serves with a worktree each, and the tmux socket it types
// into. The repository is the fixture's own, so the snapshots and resets the
// dashboard performs on a worktree it is handed stay inside the fixture.
func prepareForgeRoot(root string) error {
	if err := PrepareSwarmRoot(root); err != nil {
		return err
	}
	return initFixtureRepo(root)
}

// PrepareSwarmRoot gives a root the layout the forge's dashboard serves it
// with: the roles it serves, each with a worktree and a pane to wake, and the
// tmux socket the dashboard types into. A project is a swarm root of its own,
// so a dashboard serving a project resolves all of this inside the project.
func PrepareSwarmRoot(root string) error {
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
	if err := os.WriteFile(filepath.Join(stateDir, "tmux-socket"), []byte(filepath.Join(stateDir, "tmux.sock")+"\n"), 0o644); err != nil {
		return err
	}
	return nil
}

// initFixtureRepo gives the fixture forge root a repository of its own, with
// one commit, so every git command the dashboard runs on the fixture's behalf
// resolves to it and stops there.
func initFixtureRepo(root string) error {
	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		return nil
	}
	keep := filepath.Join(root, "README.fixture")
	if err := os.WriteFile(keep, []byte("fixture forge root\n"), 0o644); err != nil {
		return err
	}
	for _, args := range [][]string{
		{"init", "--quiet"},
		{"add", "-A"},
		{"-c", "user.email=fixture@example.org", "-c", "user.name=Fixture", "commit", "--quiet", "-m", "fixture forge root"},
	} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("prepare the fixture repository: git %s: %w: %s", strings.Join(args, " "), err, out)
		}
	}
	return nil
}

// FixtureSnapshots lists the snapshot branches the fixture's own repository
// holds for a card, which is where the dashboard's snapshots have to land.
func (d *Dashboard) FixtureSnapshots(card string) ([]string, error) {
	cmd := exec.Command("git", "-C", d.Root, "branch", "--list", "rejected/*"+card+"*", "--format=%(refname:short)")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("list the fixture's snapshots: %w: %s", err, out)
	}
	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			branches = append(branches, trimmed)
		}
	}
	return branches, nil
}

// FixtureHead is the commit the fixture's own repository is on, which is the
// commit a fixture handoff can name: the fixture's git world ends at the
// fixture, so a hash from anywhere else means nothing to it. A root that is
// not a repository has no commit to name.
func FixtureHead(root string) string {
	cmd := exec.Command("git", "-C", root, "rev-parse", "HEAD")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-23T13:46:01+02:00","module_hash":"d4ff84bf529bd1f6330107d2a2dff41e38438103794a4081438538ab9441a8ee","functions":[{"id":"func/StartDashboard","name":"StartDashboard","line":32,"end_line":67,"hash":"e74bba90a1ed7c5ee21f290ac3036c1fbfb4bb4e81a62d6e29ce0fa0f635d292"},{"id":"func/Dashboard.Stop","name":"Dashboard.Stop","line":70,"end_line":75,"hash":"ed128aeacb0ef763b4e0d775f38ccc937f48a6257e8b123fa30a79479da80a05"},{"id":"func/Dashboard.Typed","name":"Dashboard.Typed","line":78,"end_line":93,"hash":"49e3a082d1956dedd3c1a757fe01f94f068f9badadc5514417aca3848a538c2e"},{"id":"func/Dashboard.WokeWith","name":"Dashboard.WokeWith","line":96,"end_line":107,"hash":"a4965612d5f6cc27d347ea342717c06e300fff6cac17f703ffb046fe193a6b68"},{"id":"func/Dashboard.Ask","name":"Dashboard.Ask","line":110,"end_line":130,"hash":"535e219d03091f22bc5503f53681f013c1ee1d53de83a1c39d5674a836b9dcd4"},{"id":"func/Dashboard.waitForURL","name":"Dashboard.waitForURL","line":132,"end_line":149,"hash":"8e68da3f655708e3207a978452aa49e945bdc48cd5e80ab628a98cd1dc1cdc5b"},{"id":"func/wakeLog","name":"wakeLog","line":152,"end_line":154,"hash":"8af29ced2d7b72874cbe4c007a95721799351c8ad5b114586ef8b42cc4fe640b"},{"id":"func/prepareForgeRoot","name":"prepareForgeRoot","line":160,"end_line":165,"hash":"216ade16d35c250b73ee83d71b25af3ed551d27326fa2907cdbca87c8c2c164a"},{"id":"func/PrepareSwarmRoot","name":"PrepareSwarmRoot","line":171,"end_line":195,"hash":"5b48265772e09bd8db401eed77bb979260d4bab9e78cfe5014b73b90048afbcf"},{"id":"func/initFixtureRepo","name":"initFixtureRepo","line":200,"end_line":219,"hash":"df550d3c6859848b1c9e5ed2d46e38d3c756c1b9673bded60a467cdb137151c1"},{"id":"func/Dashboard.FixtureSnapshots","name":"Dashboard.FixtureSnapshots","line":223,"end_line":236,"hash":"f0c4c64523099bcd2c6cb94e4cf2f11a7489e4b63afd995942d1763f207a4c80"},{"id":"func/FixtureHead","name":"FixtureHead","line":242,"end_line":249,"hash":"96aee472035f5167658178286621c52d3509e6542c8ec38366632a081b180135"}]}
// mutate4go-manifest-end
