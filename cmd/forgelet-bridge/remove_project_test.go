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

func TestProjectRemovalPlanValidatesBeforeTheAdapterStopsTheBridge(t *testing.T) {
	configPath, statePath, forgeRoot := projectRemovalFixture(t)
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if code := removeProjectCommand(configPath, forgeRoot, "target", true, nil, &output); code != 0 {
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
		t.Fatal("the plan changed bridge state")
	}
}

func TestProjectRemovalPlanRefusesUnsafeAndUnknownInput(t *testing.T) {
	configPath, _, forgeRoot := projectRemovalFixture(t)
	tests := []struct {
		name    string
		root    string
		project string
		extra   []string
		want    string
	}{
		{
			name: "extra root argument", root: forgeRoot, project: "target",
			extra: []string{"extra"}, want: "{\"status\":\"refused\",\"reason\":\"unsafe_root\"}\n",
		},
		{
			name: "unsafe project", root: forgeRoot, project: "../target",
			want: "{\"status\":\"refused\",\"reason\":\"unsafe_project\"}\n",
		},
		{
			name: "unknown project", root: forgeRoot, project: "missing",
			want: "{\"status\":\"refused\",\"reason\":\"unknown_project\"}\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			if code := removeProjectCommand(configPath, test.root, test.project, true, test.extra, &output); code != 0 {
				t.Fatalf("code = %d", code)
			}
			if output.String() != test.want {
				t.Fatalf("output = %q, want %q", output.String(), test.want)
			}
		})
	}
}

func TestProjectRemovalRefusesMalformedStateWithoutLeakingDetails(t *testing.T) {
	configPath, statePath, forgeRoot := projectRemovalFixture(t)
	if err := os.WriteFile(statePath, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if code := removeProjectCommand(configPath, forgeRoot, "target", false, nil, &output); code != 0 {
		t.Fatalf("code = %d", code)
	}
	if output.String() != "{\"status\":\"refused\",\"reason\":\"incomplete_project_state\"}\n" {
		t.Fatalf("output = %q", output.String())
	}
}

func TestProjectRemovalCatalogRequiresAForgeletRootAndManagedMarker(t *testing.T) {
	dir := t.TempDir()
	catalog := forgeletProjectCatalog{}
	if exists, err := catalog.ForgeExists(dir); err != nil || exists {
		t.Fatalf("non-Forgelet root = %t, %v", exists, err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".swarmforge"), 0o755); err != nil {
		t.Fatal(err)
	}
	if exists, err := catalog.ForgeExists(dir); err != nil || !exists {
		t.Fatalf("Forgelet root = %t, %v", exists, err)
	}
	marker := filepath.Join(dir, "projects", "target", ".swarmforge", "single-agent")
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte("target\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if managed, err := catalog.ManagedProject(dir, "target"); err != nil || !managed {
		t.Fatalf("managed target = %t, %v", managed, err)
	}
	if managed, err := catalog.ManagedProject(dir, "pack"); err != nil || managed {
		t.Fatalf("unmanaged pack = %t, %v", managed, err)
	}
}

func TestProjectRemovalCleanerInjectsEachFailureAtTheTargetRoom(t *testing.T) {
	tests := []struct {
		stage string
		call  func(removal.ProjectCleaner) error
	}{
		{
			stage: "relay_cleanup_failed",
			call: func(cleaner removal.ProjectCleaner) error {
				return cleaner.ClearProjectFacts(context.Background(), "!failed:example.org", "!activity:example.org", "!approvals:example.org", "target")
			},
		},
		{
			stage: "operator_removal_failed",
			call: func(cleaner removal.ProjectCleaner) error {
				return cleaner.RemoveOperator(context.Background(), "!failed:example.org", "@operator:example.org")
			},
		},
		{
			stage: "bridge_membership_failed",
			call: func(cleaner removal.ProjectCleaner) error {
				return cleaner.LeaveAndForget(context.Background(), "!failed:example.org")
			},
		},
	}
	for _, test := range tests {
		t.Run(test.stage, func(t *testing.T) {
			base := &recordingProjectCleaner{}
			t.Setenv("FORGELET_BRIDGE_REMOVE_TEST_FAILURE", test.stage+"=!failed:example.org")
			cleaner := projectRemovalCleaner(base)
			if err := test.call(cleaner); err == nil {
				t.Fatal("the target room did not receive the injected failure")
			}
			if cleaner.ClearProjectFacts(context.Background(), "!other:example.org", "!activity:example.org", "!approvals:example.org", "sibling") != nil {
				t.Fatal("another project room failed")
			}
			if len(base.calls) != 1 || base.calls[0] != "facts !other:example.org" {
				t.Fatalf("Matrix calls = %v, want the other room", base.calls)
			}
		})
	}
}

type recordingProjectCleaner struct {
	calls []string
}

func (c *recordingProjectCleaner) ClearProjectFacts(_ context.Context, projectRoomID, _, _, _ string) error {
	c.calls = append(c.calls, "facts "+projectRoomID)
	return nil
}

func (c *recordingProjectCleaner) RemoveOperator(_ context.Context, roomID, _ string) error {
	c.calls = append(c.calls, "operator "+roomID)
	return nil
}

func (c *recordingProjectCleaner) LeaveAndForget(_ context.Context, roomID string) error {
	c.calls = append(c.calls, "leave "+roomID)
	return nil
}

func projectRemovalFixture(t *testing.T) (configPath, statePath, forgeRoot string) {
	t.Helper()
	dir := t.TempDir()
	forgeRoot = filepath.Join(dir, "forge")
	if err := os.MkdirAll(filepath.Join(forgeRoot, ".swarmforge"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, project := range []string{"target", "sibling"} {
		marker := filepath.Join(forgeRoot, "projects", project, ".swarmforge", "single-agent")
		if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(marker, []byte(project+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
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
			Projects: map[string]state.Project{
				"target":  {RoomID: "!target:example.org"},
				"sibling": {RoomID: "!sibling:example.org"},
			},
		},
	}}
	loaded.EnsureMaps()
	if err := loaded.Save(statePath); err != nil {
		t.Fatal(err)
	}
	return configPath, statePath, forgeRoot
}
