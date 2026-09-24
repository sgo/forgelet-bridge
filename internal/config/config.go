// Package config loads and validates the forgelet-bridge configuration.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultStateDir is used when the configuration does not name a state directory.
const DefaultStateDir = "forgelet-bridge-state"

// RoomName is the Matrix room that carries a forge's chat channel.
const RoomName = "Chat"

// ApprovalsRoomName is the Matrix room that carries a forge's approvals.
const ApprovalsRoomName = "Approvals"

// ActivityRoomName is the Matrix room that carries a forge's card updates.
const ActivityRoomName = "Activity"

// ClarificationsRoomName is the Matrix room that carries the questions a
// forge's agents are blocked on.
const ClarificationsRoomName = "Clarifications"

// RoomNameIn is the name one of a forge's rooms carries: the channel first,
// then the forge's own name in brackets, so a room list that shows them outside
// their space still says which forge a Chat belongs to. The channel comes first
// because that is the order the operator reads a list in, and the space's own
// name is the forge's, so it is not repeated there.
func RoomNameIn(forgeName, channel string) string {
	return channel + " (" + forgeName + ")"
}

// Forge is one forge the bridge serves: where it lives, and the name the
// operator knows it by.
type Forge struct {
	Root string `json:"root"`
	Name string `json:"name,omitempty"`
	// DashboardURL is the forge's dashboard, when the configuration gives it;
	// otherwise the dashboard announces itself in the forge's state directory.
	DashboardURL string `json:"dashboard_url,omitempty"`
}

// DisplayName is the name the forge shows up under: the configured name, or
// the name of its folder when the configuration does not name it.
func (f Forge) DisplayName() string {
	if name := strings.TrimSpace(f.Name); name != "" {
		return name
	}
	return folderName(f.Root)
}

// folderName is the name of the folder a forge root lives in.
func folderName(root string) string {
	return filepath.Base(strings.TrimRight(root, string(filepath.Separator)))
}

// Config describes one bridge process: the homeserver it talks to, the one
// operator it relays for, and the forges it serves.
type Config struct {
	HomeserverURL string  `json:"homeserver_url"`
	UserID        string  `json:"user_id"`
	Password      string  `json:"password,omitempty"`
	AccessToken   string  `json:"access_token,omitempty"`
	DeviceID      string  `json:"device_id,omitempty"`
	Operator      string  `json:"operator"`
	Forges        []Forge `json:"forges"`
	StateDir      string  `json:"state_dir,omitempty"`
}

// ForgeName is the name the operator knows a forge root by: the name this
// configuration gives that root, or the name of its folder when the
// configuration does not name it.
func (c Config) ForgeName(root string) string {
	for _, forge := range c.Forges {
		if forge.Root == root {
			return forge.DisplayName()
		}
	}
	return folderName(root)
}

// Load reads, defaults, and validates a configuration file.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

func (c *Config) applyDefaults() {
	if c.StateDir == "" {
		c.StateDir = DefaultStateDir
	}
	c.HomeserverURL = strings.TrimRight(c.HomeserverURL, "/")
}

// Validate reports the first reason the configuration cannot be used.
func (c Config) Validate() error {
	if c.HomeserverURL == "" {
		return errors.New("homeserver_url is required")
	}
	if c.UserID == "" {
		return errors.New("user_id is required")
	}
	if c.Operator == "" {
		return errors.New("operator is required")
	}
	if c.AccessToken == "" && c.Password == "" {
		return errors.New("either access_token or password is required")
	}
	return c.validateForges()
}

// validateForges reports the first reason the configured forges cannot be
// served: every forge needs its own root, and no root may be blank.
func (c Config) validateForges() error {
	if len(c.Forges) == 0 {
		return errors.New("at least one forge is required")
	}
	seen := map[string]bool{}
	for _, forge := range c.Forges {
		if strings.TrimSpace(forge.Root) == "" {
			return errors.New("forge roots must not be blank")
		}
		if seen[forge.Root] {
			return fmt.Errorf("forge root %s is configured twice", forge.Root)
		}
		seen[forge.Root] = true
	}
	return nil
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-24T17:01:06+02:00","module_hash":"1fc3eb06196ced81c2554ca25927ad7f8993a4975a02c0ee0588ff3793a6f3e7","functions":[{"id":"func/RoomNameIn","name":"RoomNameIn","line":34,"end_line":36,"hash":"c93a54c3de25f4b1052ae175ad0eefd41c82ab8b7d70751024cd9de0b2a52b4b"},{"id":"func/Forge.DisplayName","name":"Forge.DisplayName","line":50,"end_line":55,"hash":"abd74e0db2d2bc813e069ee4d6a8e8b31c5f4729e9d2b97d2faa571c5a4919f2"},{"id":"func/folderName","name":"folderName","line":58,"end_line":60,"hash":"7d39f1a1e7a0dd237a5bce7b32e68fdbb8158d8973ade5c0543d4ef85825951b"},{"id":"func/Config.ForgeName","name":"Config.ForgeName","line":78,"end_line":85,"hash":"3bb7b6216b66705c85071c001a8310bfbdc6a9c6f7ebe6ad4fb5739f1316aa7a"},{"id":"func/Load","name":"Load","line":88,"end_line":103,"hash":"73c9c9ba411af51e69731b5d63bbac3936492121b264cc966bf69abf9dc94d30"},{"id":"func/Config.applyDefaults","name":"Config.applyDefaults","line":105,"end_line":110,"hash":"a4b127976fa75aa7b446477d71d9914e63c45d5e0f507753b561c8699ba06242"},{"id":"func/Config.Validate","name":"Config.Validate","line":113,"end_line":127,"hash":"b1e189ca09210e62e796ad50d07a1e73be0f4ab8355f9abf8763a7aad86e3d7b"},{"id":"func/Config.validateForges","name":"Config.validateForges","line":131,"end_line":146,"hash":"aacba25ade49903c209423ffd58552034aea05c13b1e0731f9d0cafc32b8f989"}]}
// mutate4go-manifest-end
