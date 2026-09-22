package state

import (
	"os"
	"path/filepath"
	"testing"
)

func statePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "state", "bridge-state.json")
}

func TestLoadMissingFileIsEmptyState(t *testing.T) {
	loaded, err := Load(statePath(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Forges) != 0 {
		t.Errorf("forges = %+v, want none", loaded.Forges)
	}
	if _, ok := loaded.ForgeFor("/forges/forge-a"); ok {
		t.Error("empty state reported a known forge")
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	path := statePath(t)
	saved := &State{}
	saved.EnsureMaps()
	saved.RecordForge("/forges/forge-a", Forge{SpaceID: "!space", RoomID: "!room", ApprovalsRoomID: "!approvals", ActivityRoomID: "!activity"})
	saved.Relay.Threads["req-1"] = "$message"
	saved.Relay.Replied["req-1"] = "$reply"
	saved.Relay.Relayed["$operator-message"] = "req-1"

	if err := saved.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	forge, ok := loaded.ForgeFor("/forges/forge-a")
	if !ok || forge != (Forge{SpaceID: "!space", RoomID: "!room", ApprovalsRoomID: "!approvals", ActivityRoomID: "!activity"}) {
		t.Errorf("forge = %+v, %v", forge, ok)
	}
	if loaded.Relay.Threads["req-1"] != "$message" ||
		loaded.Relay.Replied["req-1"] != "$reply" ||
		loaded.Relay.Relayed["$operator-message"] != "req-1" {
		t.Errorf("relay state = %+v", loaded.Relay)
	}
}

func TestSaveLeavesNoTemporaryFile(t *testing.T) {
	path := statePath(t)
	saved := &State{}
	saved.EnsureMaps()
	if err := saved.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("temporary state file still present: %v", err)
	}
}

func TestForgeForRejectsHalfRecordedForge(t *testing.T) {
	saved := &State{}
	saved.EnsureMaps()
	saved.Forges["/forges/forge-a"] = Forge{SpaceID: "!space"}

	if _, ok := saved.ForgeFor("/forges/forge-a"); ok {
		t.Error("a forge without a chat room was reported as known")
	}
}
