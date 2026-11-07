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
	SpaceID              string             `json:"space_id"`
	RoomID               string             `json:"room_id"`
	ApprovalsRoomID      string             `json:"approvals_room_id,omitempty"`
	ActivityRoomID       string             `json:"activity_room_id,omitempty"`
	ClarificationsRoomID string             `json:"clarifications_room_id,omitempty"`
	Projects             map[string]Project `json:"projects,omitempty"`
	Cleanup              *Cleanup           `json:"cleanup,omitempty"`
}

// Project is the Matrix room the bridge created for one single-agent project
// inside its forge's space.
type Project struct {
	RoomID  string       `json:"room_id,omitempty"`
	Cleanup *RoomCleanup `json:"cleanup,omitempty"`
}

// Cleanup is the progress of removing one forge's Matrix rooms. It stays with
// the forge record until every room has been cleaned, so a retry knows both
// which room ids to use and which operations already succeeded.
type Cleanup struct {
	Rooms map[string]RoomCleanup `json:"rooms,omitempty"`
}

// RoomCleanup is one room's completed cleanup stages. A stage is recorded only
// after Matrix accepted it, so retrying resumes at the first unfinished stage.
type RoomCleanup struct {
	FactsCleared    bool `json:"facts_cleared,omitempty"`
	OperatorRemoved bool `json:"operator_removed,omitempty"`
	BridgeLeft      bool `json:"bridge_left,omitempty"`
}

// Done reports whether every cleanup stage succeeded for the room.
func (r RoomCleanup) Done() bool {
	return r.FactsCleared && r.OperatorRemoved && r.BridgeLeft
}

// MarkFactsCleared records the first completed cleanup stage.
func (r RoomCleanup) MarkFactsCleared() RoomCleanup {
	r.FactsCleared = true
	return r
}

// MarkOperatorRemoved records the second completed cleanup stage.
func (r RoomCleanup) MarkOperatorRemoved() RoomCleanup {
	r.OperatorRemoved = true
	return r
}

// MarkBridgeLeft records the final completed cleanup stage.
func (r RoomCleanup) MarkBridgeLeft() RoomCleanup {
	r.BridgeLeft = true
	return r
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

// ProjectFor returns the project room already recorded for a forge root.
func (s *State) ProjectFor(root, name string) (Project, bool) {
	forge, ok := s.Forges[root]
	if !ok {
		return Project{}, false
	}
	project, ok := forge.Projects[name]
	return project, ok && project.RoomID != ""
}

// RecordProject remembers the room of one single-agent project.
func (s *State) RecordProject(root, name string, project Project) {
	s.EnsureMaps()
	forge := s.Forges[root]
	if forge.Projects == nil {
		forge.Projects = map[string]Project{}
	}
	forge.Projects[name] = project
	s.Forges[root] = forge
}

// ProjectRemovalPending reports whether the project has recorded removal
// progress. The bridge leaves such a project alone until the removal either
// finishes or is retried to completion.
func (s State) ProjectRemovalPending(root, name string) bool {
	forge, ok := s.Forges[root]
	if !ok {
		return false
	}
	project, ok := forge.Projects[name]
	return ok && project.Cleanup != nil
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
// {"version":1,"tested_at":"2026-11-07T23:17:17+01:00","module_hash":"6f45fbbf0f99e97cada05fadb6c7c55c67b45adaec7bc65275f4b157437f94ea","functions":[{"id":"func/RoomCleanup.Done","name":"RoomCleanup.Done","line":48,"end_line":50,"hash":"c3b779fd0a4360a5317a35ea05a0a7e0a97fe8293684710da051fa3dcda0df29"},{"id":"func/RoomCleanup.MarkFactsCleared","name":"RoomCleanup.MarkFactsCleared","line":53,"end_line":56,"hash":"18d1de6cc9a4b5708f44fa341ffd6a2c0f1a6093c70d800d67db46738cd55b34"},{"id":"func/RoomCleanup.MarkOperatorRemoved","name":"RoomCleanup.MarkOperatorRemoved","line":59,"end_line":62,"hash":"76fd5e4308fc5998d00a7b58c29230add5c68c4be75f9675545b67b1b47cfa41"},{"id":"func/RoomCleanup.MarkBridgeLeft","name":"RoomCleanup.MarkBridgeLeft","line":65,"end_line":68,"hash":"fb6880668471738ea8d06f8a9bdb0cdb69f68cd8a4f1bf745c97faeefd66f986"},{"id":"func/Load","name":"Load","line":77,"end_line":94,"hash":"3a51707dde5154917e8a5a53035d24ce3fcac7ffb26af5a85ffe35c0b9c58f46"},{"id":"func/State.EnsureMaps","name":"State.EnsureMaps","line":97,"end_line":102,"hash":"d9f9c1a00570037d86f9a039b80fa8df6656b5f789e1dad9d3d80e30917321a4"},{"id":"func/State.ForgeFor","name":"State.ForgeFor","line":105,"end_line":110,"hash":"c852f290df6e1655f351b19833365c0e5801f3c2e27a8de78d66a67e7b347f9d"},{"id":"func/State.RecordForge","name":"State.RecordForge","line":113,"end_line":116,"hash":"c9d5772df498ae9d642682e74a6eb07e8831c9fd60eedc9fa115861dfce8c70c"},{"id":"func/State.ProjectFor","name":"State.ProjectFor","line":119,"end_line":126,"hash":"d9873bdb7b0f80346f24befdf2c45eb0c72436f95717c20bfd4505532020a3fc"},{"id":"func/State.RecordProject","name":"State.RecordProject","line":129,"end_line":137,"hash":"440dd2356bd3fc7b2ac47460e4b87b139c025b555d99f9faf9b8810c9c5855d5"},{"id":"func/State.ProjectRemovalPending","name":"State.ProjectRemovalPending","line":142,"end_line":149,"hash":"22b82e80b9a2b49656ba3c24499bb911432bcbdea9a538f6fc18d15459d43204"},{"id":"func/State.Save","name":"State.Save","line":153,"end_line":167,"hash":"699bd5003c86aa3da98352d99437c38ed3b5a654bc6aa755d90976d728ac240d"}]}
// mutate4go-manifest-end
