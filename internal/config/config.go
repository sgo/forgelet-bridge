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

// Forge is one forge the bridge serves: where it lives, and the name the
// operator knows it by.
type Forge struct {
	Root string `json:"root"`
	Name string `json:"name,omitempty"`
}

// DisplayName is the name the forge shows up under: the configured name, or
// the name of its folder when the configuration does not name it.
func (f Forge) DisplayName() string {
	if strings.TrimSpace(f.Name) != "" {
		return strings.TrimSpace(f.Name)
	}
	return filepath.Base(strings.TrimRight(f.Root, string(filepath.Separator)))
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
// {"version":1,"tested_at":"2026-09-21T23:18:45+02:00","module_hash":"4a2f9010c1fcd4e816c21f6bea11172a9b36eb0e51694ce2ceb72af545ba0a2d","functions":[{"id":"func/Load","name":"Load","line":33,"end_line":48,"hash":"73c9c9ba411af51e69731b5d63bbac3936492121b264cc966bf69abf9dc94d30"},{"id":"func/Config.applyDefaults","name":"Config.applyDefaults","line":50,"end_line":55,"hash":"a4b127976fa75aa7b446477d71d9914e63c45d5e0f507753b561c8699ba06242"},{"id":"func/Config.Validate","name":"Config.Validate","line":58,"end_line":80,"hash":"ea426906b592ae4e753437040e3dfdcb37c216a8bb8f5d80bef8aae91466cd2d"},{"id":"func/ForgeName","name":"ForgeName","line":84,"end_line":86,"hash":"14d0ebe0183b97504e95ad0cbf9789c520db6c25982a1e7812746b7fbad670e9"}]}
// mutate4go-manifest-end
