package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bridge.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadAcceptsForgeRootList(t *testing.T) {
	path := write(t, `{
		"homeserver_url": "http://127.0.0.1:8008/",
		"user_id": "@bridge:example.org",
		"password": "secret",
		"operator": "@operator:example.org",
		"forges": [
			{"root": "/forges/forge-a", "name": "Forgelet"},
			{"root": "/forges/forge-b"}
		],
		"state_dir": "/state"
	}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HomeserverURL != "http://127.0.0.1:8008" {
		t.Errorf("homeserver URL = %q, want trailing slash trimmed", cfg.HomeserverURL)
	}
	if len(cfg.Forges) != 2 {
		t.Fatalf("forges = %v, want two", cfg.Forges)
	}
	if cfg.Forges[0].DisplayName() != "Forgelet" {
		t.Errorf("first forge name = %q, want the configured name", cfg.Forges[0].DisplayName())
	}
	if cfg.Forges[1].DisplayName() != "forge-b" {
		t.Errorf("second forge name = %q, want the folder name", cfg.Forges[1].DisplayName())
	}
	if cfg.StateDir != "/state" {
		t.Errorf("state dir = %q, want /state", cfg.StateDir)
	}
}

func TestLoadDefaultsStateDir(t *testing.T) {
	path := write(t, `{
		"homeserver_url": "http://127.0.0.1:8008",
		"user_id": "@bridge:example.org",
		"access_token": "token",
		"operator": "@operator:example.org",
		"forges": [{"root": "/forges/forge-a"}]
	}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.StateDir != DefaultStateDir {
		t.Errorf("state dir = %q, want %q", cfg.StateDir, DefaultStateDir)
	}
}

func TestLoadRejectsBadConfigurations(t *testing.T) {
	cases := map[string]struct {
		body string
		want string
	}{
		"no homeserver": {
			body: `{"user_id":"@bridge:example.org","password":"p","operator":"@o:example.org","forge_roots":["/forge-a"]}`,
			want: "homeserver_url",
		},
		"no user": {
			body: `{"homeserver_url":"http://hs","password":"p","operator":"@o:example.org","forge_roots":["/forge-a"]}`,
			want: "user_id",
		},
		"no operator": {
			body: `{"homeserver_url":"http://hs","user_id":"@b:example.org","password":"p","forge_roots":["/forge-a"]}`,
			want: "operator",
		},
		"no credentials": {
			body: `{"homeserver_url":"http://hs","user_id":"@b:example.org","operator":"@o:example.org","forge_roots":["/forge-a"]}`,
			want: "access_token or password",
		},
		"no forge roots": {
			body: `{"homeserver_url":"http://hs","user_id":"@b:example.org","password":"p","operator":"@o:example.org","forges":[]}`,
			want: "at least one forge",
		},
		"blank forge root": {
			body: `{"homeserver_url":"http://hs","user_id":"@b:example.org","password":"p","operator":"@o:example.org","forges":[{"root":"  "}]}`,
			want: "must not be blank",
		},
		"same root twice": {
			body: `{"homeserver_url":"http://hs","user_id":"@b:example.org","password":"p","operator":"@o:example.org","forges":[{"root":"/a"},{"root":"/a"}]}`,
			want: "configured twice",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Load(write(t, tc.body))
			if err == nil {
				t.Fatalf("Load succeeded, want error mentioning %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestForgeDisplayName(t *testing.T) {
	cases := map[string]struct {
		forge Forge
		want  string
	}{
		"configured name wins":        {Forge{Root: "/forges/sgo", Name: "Saibill"}, "Saibill"},
		"padded name is trimmed":      {Forge{Root: "/forges/sgo", Name: "  Saibill  "}, "Saibill"},
		"folder name is the fallback": {Forge{Root: "/forges/forge-a"}, "forge-a"},
		"trailing separator":          {Forge{Root: "/forges/forge-a/"}, "forge-a"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := tc.forge.DisplayName(); got != tc.want {
				t.Errorf("DisplayName() = %q, want %q", got, tc.want)
			}
		})
	}
}
