package fixtures

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ServerName is the homeserver name every fixture user id belongs to.
const ServerName = "example.org"

// Synapse is a pinned Synapse homeserver on a throwaway database.
type Synapse struct {
	URL  string
	Dir  string
	port int
	cmd  *exec.Cmd
	log  *os.File
}

// StartSynapse installs the pinned Synapse into the project-local environment
// if needed, then starts a homeserver in dir and waits for it to answer.
func StartSynapse(ctx context.Context, dir string) (*Synapse, error) {
	python, err := synapsePython(ctx)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	port, err := freePort()
	if err != nil {
		return nil, err
	}
	if err := writeSynapseConfig(dir, port); err != nil {
		return nil, err
	}

	logFile, err := os.Create(filepath.Join(dir, "synapse.stdout.log"))
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(python, "-m", "synapse.app.homeserver", "--config-path", filepath.Join(dir, "homeserver.yaml"))
	cmd.Dir = dir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return nil, fmt.Errorf("start synapse: %w", err)
	}

	server := &Synapse{
		URL:  fmt.Sprintf("http://127.0.0.1:%d", port),
		Dir:  dir,
		port: port,
		cmd:  cmd,
		log:  logFile,
	}
	if err := server.waitUntilReady(ctx); err != nil {
		server.Stop()
		return nil, err
	}
	return server, nil
}

// Stop shuts the homeserver down.
func (s *Synapse) Stop() {
	if s == nil || s.cmd == nil || s.cmd.Process == nil {
		return
	}
	_ = s.cmd.Process.Signal(os.Interrupt)
	done := make(chan struct{})
	go func() {
		_ = s.cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		_ = s.cmd.Process.Kill()
		<-done
	}
	if s.log != nil {
		s.log.Close()
	}
}

func (s *Synapse) waitUntilReady(ctx context.Context) error {
	deadline := time.Now().Add(2 * time.Minute)
	url := s.URL + "/_matrix/client/versions"
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err == nil {
			resp, err := http.DefaultClient.Do(req)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return nil
				}
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("synapse did not answer at %s; see %s", url, filepath.Join(s.Dir, "synapse.stdout.log"))
}

// Register creates a fixture user and returns its credentials.
func (s *Synapse) Register(ctx context.Context, localpart, password string) (userID, accessToken, deviceID string, err error) {
	body, err := json.Marshal(map[string]any{
		"username":                    localpart,
		"password":                    password,
		"auth":                        map[string]any{"type": "m.login.dummy"},
		"initial_device_display_name": localpart,
	})
	if err != nil {
		return "", "", "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.URL+"/_matrix/client/v3/register", bytes.NewReader(body))
	if err != nil {
		return "", "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()

	var parsed struct {
		UserID      string `json:"user_id"`
		AccessToken string `json:"access_token"`
		DeviceID    string `json:"device_id"`
		ErrCode     string `json:"errcode"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", "", "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", "", fmt.Errorf("register %s: %s %s", localpart, parsed.ErrCode, parsed.Error)
	}
	return parsed.UserID, parsed.AccessToken, parsed.DeviceID, nil
}

// UserID builds a fixture user id on this homeserver.
func UserID(localpart string) string {
	return "@" + localpart + ":" + ServerName
}

// synapsePython makes sure the pinned Synapse is installed in the
// project-local environment and returns its interpreter.
func synapsePython(ctx context.Context) (string, error) {
	envDir := SynapseEnvDir()
	python := filepath.Join(envDir, "venv", "bin", "python")
	if _, err := os.Stat(python); err == nil {
		return python, nil
	}

	requirements := SynapseRequirements()
	if _, err := os.Stat(requirements); err != nil {
		return "", fmt.Errorf("pinned synapse requirements missing: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(python), 0o755); err != nil {
		return "", err
	}
	if out, err := run(ctx, "", "python3", "-m", "venv", filepath.Dir(filepath.Dir(python))); err != nil {
		return "", fmt.Errorf("create synapse environment: %w: %s", err, out)
	}
	if out, err := run(ctx, "", python, "-m", "pip", "install", "--quiet", "--requirement", requirements); err != nil {
		return "", fmt.Errorf("install pinned synapse: %w: %s", err, out)
	}
	return python, nil
}

func run(ctx context.Context, dir string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func freePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func writeSynapseConfig(dir string, port int) error {
	secret, err := randomHex(32)
	if err != nil {
		return err
	}
	config := fmt.Sprintf(`server_name: "%s"
pid_file: %s
listeners:
  - port: %d
    type: http
    bind_addresses: ["127.0.0.1"]
    resources:
      - names: [client, federation]
        compress: false
database:
  name: sqlite3
  args:
    database: %s
log_config: %s
media_store_path: %s
signing_key_path: %s
registration_shared_secret: "%s"
macaroon_secret_key: "%s"
form_secret: "%s"
enable_registration: true
enable_registration_without_verification: true
report_stats: false
trusted_key_servers: []
suppress_key_server_warning: true
`,
		ServerName,
		filepath.Join(dir, "synapse.pid"),
		port,
		filepath.Join(dir, "homeserver.db"),
		filepath.Join(dir, "log.config"),
		filepath.Join(dir, "media"),
		filepath.Join(dir, "signing.key"),
		secret, secret, secret,
	)
	if err := os.WriteFile(filepath.Join(dir, "homeserver.yaml"), []byte(config), 0o600); err != nil {
		return err
	}
	logConfig := fmt.Sprintf(`version: 1
formatters:
  precise:
    format: '%%(asctime)s - %%(name)s - %%(levelname)s - %%(request)s - %%(message)s'
handlers:
  file:
    class: logging.FileHandler
    formatter: precise
    filename: %s
root:
  level: INFO
  handlers: [file]
disable_existing_loggers: false
`, filepath.Join(dir, "homeserver.log"))
	return os.WriteFile(filepath.Join(dir, "log.config"), []byte(logConfig), 0o600)
}

func randomHex(bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// SynapseURLFromEnv reports the homeserver the runner already started, if any.
func SynapseURLFromEnv() string {
	return strings.TrimSpace(os.Getenv("FORGELET_SYNAPSE_URL"))
}
