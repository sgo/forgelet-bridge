package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/unclebob/forgelet-bridge/internal/state"
)

func TestRemovalPlanValidatesBeforeTheAdapterStopsTheBridge(t *testing.T) {
	configPath, statePath, forgeRoot := removalFixture(t)
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if code := remove(configPath, forgeRoot, true, nil, &output); code != 0 {
		t.Fatalf("code = %d", code)
	}
	if output.String() != "ready\n" {
		t.Fatalf("output = %q, want ready", output.String())
	}
	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("the plan changed bridge state before the bridge stopped")
	}
}

func TestRemovalCommandRefusesExtraArgumentsAsJSON(t *testing.T) {
	configPath, _, forgeRoot := removalFixture(t)
	var output bytes.Buffer
	if code := remove(configPath, forgeRoot, false, []string{"extra"}, &output); code != 0 {
		t.Fatalf("code = %d", code)
	}
	if output.String() != "{\"status\":\"refused\",\"reason\":\"unsafe_root\"}\n" {
		t.Fatalf("output = %q", output.String())
	}
}

func TestRemovalCommandRefusesMalformedStateWithoutLeakingDetails(t *testing.T) {
	configPath, statePath, forgeRoot := removalFixture(t)
	if err := os.WriteFile(statePath, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if code := remove(configPath, forgeRoot, false, nil, &output); code != 0 {
		t.Fatalf("code = %d", code)
	}
	if output.String() != "{\"status\":\"refused\",\"reason\":\"incomplete_state\"}\n" {
		t.Fatalf("output = %q", output.String())
	}
}

func removalFixture(t *testing.T) (configPath, statePath, forgeRoot string) {
	t.Helper()
	dir := t.TempDir()
	forgeRoot = filepath.Join(dir, "forge")
	configPath = filepath.Join(dir, "matrix-bridge.json")
	stateDir := filepath.Join(dir, "bridge-state")
	configBody := `{
  "homeserver_url": "https://matrix.example.org",
  "user_id": "@bridge:example.org",
  "access_token": "secret",
  "operator": "@operator:example.org",
  "forges": [{"root": "` + forgeRoot + `"}],
  "state_dir": "` + stateDir + `"
}
`
	if err := os.WriteFile(configPath, []byte(configBody), 0o644); err != nil {
		t.Fatal(err)
	}
	statePath = filepath.Join(stateDir, "bridge-state.json")
	loaded := &state.State{Forges: map[string]state.Forge{
		forgeRoot: {
			SpaceID:              "!space:example.org",
			RoomID:               "!chat:example.org",
			ApprovalsRoomID:      "!approvals:example.org",
			ActivityRoomID:       "!activity:example.org",
			ClarificationsRoomID: "!clarifications:example.org",
		},
	}}
	loaded.EnsureMaps()
	if err := loaded.Save(statePath); err != nil {
		t.Fatal(err)
	}
	return configPath, statePath, forgeRoot
}
