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

// Config describes one bridge process: the homeserver it talks to, the one
// operator it relays for, and the forge roots it serves.
type Config struct {
	HomeserverURL string   `json:"homeserver_url"`
	UserID        string   `json:"user_id"`
	Password      string   `json:"password,omitempty"`
	AccessToken   string   `json:"access_token,omitempty"`
	DeviceID      string   `json:"device_id,omitempty"`
	Operator      string   `json:"operator"`
	ForgeRoots    []string `json:"forge_roots"`
	StateDir      string   `json:"state_dir,omitempty"`
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
	if len(c.ForgeRoots) == 0 {
		return errors.New("at least one forge root is required")
	}
	for _, root := range c.ForgeRoots {
		if strings.TrimSpace(root) == "" {
			return errors.New("forge roots must not be blank")
		}
	}
	return nil
}

// ForgeName is the Matrix-visible name of a forge: the base name of its root
// directory.
func ForgeName(root string) string {
	return filepath.Base(strings.TrimRight(root, string(filepath.Separator)))
}
