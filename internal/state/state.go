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
	SpaceID         string `json:"space_id"`
	RoomID          string `json:"room_id"`
	ApprovalsRoomID string `json:"approvals_room_id,omitempty"`
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
	return forge, ok && forge.SpaceID != "" && forge.RoomID != "" && forge.ApprovalsRoomID != ""
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
// {"version":1,"tested_at":"2026-09-22T15:50:29+02:00","module_hash":"f4d9908425ddf9364546c8986e0832c66e91b7798ee16a210fb72bf551f77a17","functions":[{"id":"func/Load","name":"Load","line":28,"end_line":45,"hash":"3a51707dde5154917e8a5a53035d24ce3fcac7ffb26af5a85ffe35c0b9c58f46"},{"id":"func/State.EnsureMaps","name":"State.EnsureMaps","line":48,"end_line":53,"hash":"d9f9c1a00570037d86f9a039b80fa8df6656b5f789e1dad9d3d80e30917321a4"},{"id":"func/State.ForgeFor","name":"State.ForgeFor","line":56,"end_line":59,"hash":"92d8df5e5b6162b01c2ec0463cebb8a9bb5ef6ce3da51a1c5959c3296994dcdd"},{"id":"func/State.RecordForge","name":"State.RecordForge","line":62,"end_line":65,"hash":"c9d5772df498ae9d642682e74a6eb07e8831c9fd60eedc9fa115861dfce8c70c"},{"id":"func/State.Save","name":"State.Save","line":69,"end_line":83,"hash":"699bd5003c86aa3da98352d99437c38ed3b5a654bc6aa755d90976d728ac240d"}]}
// mutate4go-manifest-end
