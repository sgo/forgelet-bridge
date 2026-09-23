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
	saved.RecordForge("/forges/forge-a", Forge{
		SpaceID: "!space", RoomID: "!room",
		ApprovalsRoomID: "!approvals", ActivityRoomID: "!activity", ClarificationsRoomID: "!clarifications",
	})
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
	if !ok || forge != (Forge{
		SpaceID: "!space", RoomID: "!room",
		ApprovalsRoomID: "!approvals", ActivityRoomID: "!activity", ClarificationsRoomID: "!clarifications",
	}) {
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
	// A state file written before the bridge knew about the approvals room: it
	// is a forge the bridge has to provision again, not one it can serve.
	saved.Forges["/forges/forge-b"] = Forge{SpaceID: "!space", RoomID: "!room"}
	// The same for a state file written before the activity room.
	saved.Forges["/forges/forge-c"] = Forge{SpaceID: "!space", RoomID: "!room", ApprovalsRoomID: "!approvals"}
	// A forge missing only the activity room, which the clarifications room
	// being present must not stand in for.
	saved.Forges["/forges/forge-d"] = Forge{
		SpaceID: "!space", RoomID: "!room",
		ApprovalsRoomID: "!approvals", ClarificationsRoomID: "!clarifications",
	}
	// The same for a state file written before the clarifications room.
	saved.Forges["/forges/forge-e"] = Forge{
		SpaceID: "!space", RoomID: "!room",
		ApprovalsRoomID: "!approvals", ActivityRoomID: "!activity",
	}

	if _, ok := saved.ForgeFor("/forges/forge-a"); ok {
		t.Error("a forge without a chat room was reported as known")
	}
	if _, ok := saved.ForgeFor("/forges/forge-b"); ok {
		t.Error("a forge without an approvals room was reported as known")
	}
	if _, ok := saved.ForgeFor("/forges/forge-c"); ok {
		t.Error("a forge without an activity room was reported as known")
	}
	if _, ok := saved.ForgeFor("/forges/forge-d"); ok {
		t.Error("a forge without an activity room was reported as known")
	}
	if _, ok := saved.ForgeFor("/forges/forge-e"); ok {
		t.Error("a forge without a clarifications room was reported as known")
	}
}
