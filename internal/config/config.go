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
// {"version":1,"tested_at":"2026-09-22T14:59:15+02:00","module_hash":"35397fa89a11bcb0f35af2fcd7b0b7b06f380b5302836bda6745e75aa1e1c460","functions":[{"id":"func/Forge.DisplayName","name":"Forge.DisplayName","line":28,"end_line":33,"hash":"abd74e0db2d2bc813e069ee4d6a8e8b31c5f4729e9d2b97d2faa571c5a4919f2"},{"id":"func/folderName","name":"folderName","line":36,"end_line":38,"hash":"7d39f1a1e7a0dd237a5bce7b32e68fdbb8158d8973ade5c0543d4ef85825951b"},{"id":"func/Config.ForgeName","name":"Config.ForgeName","line":56,"end_line":63,"hash":"3bb7b6216b66705c85071c001a8310bfbdc6a9c6f7ebe6ad4fb5739f1316aa7a"},{"id":"func/Load","name":"Load","line":66,"end_line":81,"hash":"73c9c9ba411af51e69731b5d63bbac3936492121b264cc966bf69abf9dc94d30"},{"id":"func/Config.applyDefaults","name":"Config.applyDefaults","line":83,"end_line":88,"hash":"a4b127976fa75aa7b446477d71d9914e63c45d5e0f507753b561c8699ba06242"},{"id":"func/Config.Validate","name":"Config.Validate","line":91,"end_line":105,"hash":"b1e189ca09210e62e796ad50d07a1e73be0f4ab8355f9abf8763a7aad86e3d7b"},{"id":"func/Config.validateForges","name":"Config.validateForges","line":109,"end_line":124,"hash":"aacba25ade49903c209423ffd58552034aea05c13b1e0731f9d0cafc32b8f989"}]}
// mutate4go-manifest-end
