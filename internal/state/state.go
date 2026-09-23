// Package state persists the bridge's bookkeeping so that a restart neither
// replays nor duplicates work.
package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

// Forge is the Matrix side the bridge created for one forge root.
type Forge struct {
	SpaceID              string `json:"space_id"`
	RoomID               string `json:"room_id"`
	ApprovalsRoomID      string `json:"approvals_room_id,omitempty"`
	ActivityRoomID       string `json:"activity_room_id,omitempty"`
	ClarificationsRoomID string `json:"clarifications_room_id,omitempty"`
}

// State is everything the bridge remembers between runs.
type State struct {
	Forges map[string]Forge `json:"forges,omitempty"`
	Relay  relay.State      `json:"relay"`
}

// Load reads the state file. A missing file is an empty state, not an error.
func Load(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			loaded := &State{}
			loaded.EnsureMaps()
			return loaded, nil
		}
		return nil, err
	}

	var loaded State
	if err := json.Unmarshal(data, &loaded); err != nil {
		return nil, err
	}
	loaded.EnsureMaps()
	return &loaded, nil
}

// EnsureMaps makes every map writable.
func (s *State) EnsureMaps() {
	if s.Forges == nil {
		s.Forges = map[string]Forge{}
	}
	s.Relay.EnsureMaps()
}

// ForgeFor returns the Matrix side already recorded for a forge root.
func (s *State) ForgeFor(root string) (Forge, bool) {
	forge, ok := s.Forges[root]
	return forge, ok && forge.SpaceID != "" && forge.RoomID != "" &&
		forge.ApprovalsRoomID != "" && forge.ActivityRoomID != "" &&
		forge.ClarificationsRoomID != ""
}

// RecordForge remembers the Matrix side of a forge root.
func (s *State) RecordForge(root string, forge Forge) {
	s.EnsureMaps()
	s.Forges[root] = forge
}

// Save writes the state file through a temporary file so a reader never sees a
// half-written state.
func (s *State) Save(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	temp := path + ".tmp"
	if err := os.WriteFile(temp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-23T13:37:24+02:00","module_hash":"9c9f0aec8e80a455a83df6c4e74b1b4529a8d9ffed7214be692b2e2d93d94426","functions":[{"id":"func/Load","name":"Load","line":30,"end_line":47,"hash":"3a51707dde5154917e8a5a53035d24ce3fcac7ffb26af5a85ffe35c0b9c58f46"},{"id":"func/State.EnsureMaps","name":"State.EnsureMaps","line":50,"end_line":55,"hash":"d9f9c1a00570037d86f9a039b80fa8df6656b5f789e1dad9d3d80e30917321a4"},{"id":"func/State.ForgeFor","name":"State.ForgeFor","line":58,"end_line":63,"hash":"c852f290df6e1655f351b19833365c0e5801f3c2e27a8de78d66a67e7b347f9d"},{"id":"func/State.RecordForge","name":"State.RecordForge","line":66,"end_line":69,"hash":"c9d5772df498ae9d642682e74a6eb07e8831c9fd60eedc9fa115861dfce8c70c"},{"id":"func/State.Save","name":"State.Save","line":73,"end_line":87,"hash":"699bd5003c86aa3da98352d99437c38ed3b5a654bc6aa755d90976d728ac240d"}]}
// mutate4go-manifest-end
