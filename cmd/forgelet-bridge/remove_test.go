package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/unclebob/forgelet-bridge/internal/removal"
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

func TestRemovalCleanerInjectsEachFailureAtTheNamedRoom(t *testing.T) {
	tests := []struct {
		stage string
		call  func(removal.Cleaner, string) error
	}{
		{"relay_cleanup_failed", func(cleaner removal.Cleaner, roomID string) error {
			return cleaner.ClearFacts(context.Background(), roomID)
		}},
		{"operator_removal_failed", func(cleaner removal.Cleaner, roomID string) error {
			return cleaner.RemoveOperator(context.Background(), roomID, "@operator:example.org")
		}},
		{"bridge_membership_failed", func(cleaner removal.Cleaner, roomID string) error {
			return cleaner.LeaveAndForget(context.Background(), roomID)
		}},
	}
	for _, test := range tests {
		t.Run(test.stage, func(t *testing.T) {
			base := &recordingCleaner{}
			t.Setenv("FORGELET_BRIDGE_REMOVE_TEST_FAILURE", test.stage+"=!failed:example.org")
			cleaner := removalCleaner(base)
			if err := test.call(cleaner, "!failed:example.org"); err == nil {
				t.Fatal("the named room did not receive the injected failure")
			}
			if err := test.call(cleaner, "!other:example.org"); err != nil {
				t.Fatalf("another room failed: %v", err)
			}
			if len(base.calls) != 1 || base.calls[0] != "!other:example.org" {
				t.Fatalf("Matrix calls = %v, want only the other room", base.calls)
			}
		})
	}
}

func TestRemovalCleanerPassesThroughWhenNoFailureIsRequested(t *testing.T) {
	base := &recordingCleaner{}
	t.Setenv("FORGELET_BRIDGE_REMOVE_TEST_FAILURE", "")
	cleaner := removalCleaner(base)
	if err := cleaner.ClearFacts(context.Background(), "!room:example.org"); err != nil {
		t.Fatal(err)
	}
	if len(base.calls) != 1 || base.calls[0] != "!room:example.org" {
		t.Fatalf("Matrix calls = %v, want the real cleaner", base.calls)
	}
}

type recordingCleaner struct {
	calls []string
}

func (c *recordingCleaner) ClearFacts(_ context.Context, roomID string) error {
	c.calls = append(c.calls, roomID)
	return nil
}

func (c *recordingCleaner) RemoveOperator(_ context.Context, roomID, _ string) error {
	c.calls = append(c.calls, roomID)
	return nil
}

func (c *recordingCleaner) LeaveAndForget(_ context.Context, roomID string) error {
	c.calls = append(c.calls, roomID)
	return nil
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
