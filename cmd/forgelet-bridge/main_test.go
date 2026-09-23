package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// goodConfig is a configuration the bridge would serve.
const goodConfig = `{
  "homeserver_url": "https://matrix.example.org",
  "user_id": "@bridge:example.org",
  "access_token": "token",
  "operator": "@operator:example.org",
  "forges": [{"root": "/srv/forges/one", "name": "One"}]
}`

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "forgelet-bridge.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCheckAcceptsAConfigurationTheBridgeWouldServe(t *testing.T) {
	if err := check(writeConfig(t, goodConfig)); err != nil {
		t.Errorf("check: %v, want the configuration accepted", err)
	}
}

func TestCheckRefusesAConfiguredTwice(t *testing.T) {
	twice := strings.Replace(goodConfig,
		`{"root": "/srv/forges/one", "name": "One"}`,
		`{"root": "/srv/forges/one", "name": "One"}, {"root": "/srv/forges/one", "name": "Again"}`, 1)

	err := check(writeConfig(t, twice))
	if err == nil {
		t.Fatal("check accepted a forge root configured twice")
	}
	if !strings.Contains(err.Error(), "configured twice") {
		t.Errorf("error = %q, want the reason the configuration cannot be used", err)
	}
}

func TestCheckRefusesAConfigurationThatIsNotJSON(t *testing.T) {
	if err := check(writeConfig(t, "not json")); err == nil {
		t.Fatal("check accepted a configuration that is not JSON")
	}
}
