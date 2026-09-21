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
		"forge_roots": ["/forges/forge-a", "/forges/forge-b"],
		"state_dir": "/state"
	}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HomeserverURL != "http://127.0.0.1:8008" {
		t.Errorf("homeserver URL = %q, want trailing slash trimmed", cfg.HomeserverURL)
	}
	if len(cfg.ForgeRoots) != 2 {
		t.Errorf("forge roots = %v, want two", cfg.ForgeRoots)
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
		"forge_roots": ["/forges/forge-a"]
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
			body: `{"homeserver_url":"http://hs","user_id":"@b:example.org","password":"p","operator":"@o:example.org","forge_roots":[]}`,
			want: "at least one forge root",
		},
		"blank forge root": {
			body: `{"homeserver_url":"http://hs","user_id":"@b:example.org","password":"p","operator":"@o:example.org","forge_roots":["  "]}`,
			want: "must not be blank",
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

func TestForgeNameUsesDirectoryBaseName(t *testing.T) {
	cases := map[string]string{
		"/forges/forge-a":  "forge-a",
		"/forges/forge-a/": "forge-a",
		"forge-b":          "forge-b",
	}
	for root, want := range cases {
		if got := ForgeName(root); got != want {
			t.Errorf("ForgeName(%q) = %q, want %q", root, got, want)
		}
	}
}
