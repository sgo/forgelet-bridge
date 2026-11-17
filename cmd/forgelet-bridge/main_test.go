package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unclebob/forgelet-bridge/internal/dashboard"
)

// goodConfig is a configuration the bridge would serve.
const goodConfig = `{
  "homeserver_url": "https://matrix.example.org",
  "user_id": "@bridge:example.org",
  "access_token": "token",
  "operator": "@operator:example.org",
  "forges": [{"root": "/srv/forges/one", "name": "One"}]
}`

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "forgelet-bridge.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCheckAcceptsAConfigurationTheBridgeWouldServe(t *testing.T) {
	if err := check(writeConfig(t, goodConfig)); err != nil {
		t.Errorf("check: %v, want the configuration accepted", err)
	}
}

func TestCheckRefusesAConfiguredTwice(t *testing.T) {
	twice := strings.Replace(goodConfig,
		`{"root": "/srv/forges/one", "name": "One"}`,
		`{"root": "/srv/forges/one", "name": "One"}, {"root": "/srv/forges/one", "name": "Again"}`, 1)

	err := check(writeConfig(t, twice))
	if err == nil {
		t.Fatal("check accepted a forge root configured twice")
	}
	if !strings.Contains(err.Error(), "configured twice") {
		t.Errorf("error = %q, want the reason the configuration cannot be used", err)
	}
}

func TestCheckRefusesAConfigurationThatIsNotJSON(t *testing.T) {
	if err := check(writeConfig(t, "not json")); err == nil {
		t.Fatal("check accepted a configuration that is not JSON")
	}
}

func TestTypedClarificationPreflightAcceptsEveryFamilyProjection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("X-SwarmForge-Projection", dashboard.AutomationProjection)
		_ = json.NewEncoder(writer).Encode(preflightState("family", "One"))
	}))
	defer server.Close()

	root := t.TempDir()
	config := `{
  "homeserver_url": "https://matrix.example.org",
  "user_id": "@bridge:example.org",
  "access_token": "token",
  "operator": "@operator:example.org",
  "forges": [{"root": "` + root + `", "name": "One", "dashboard_url": "` + server.URL + `"}]
}`
	if err := checkTypedClarifications(writeConfig(t, config)); err != nil {
		t.Fatalf("typed clarification preflight: %v", err)
	}
}

func TestTypedClarificationPreflightRefusesANonFamilyProjection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("X-SwarmForge-Projection", dashboard.AutomationProjection)
		_ = json.NewEncoder(writer).Encode(preflightState("project", "One"))
	}))
	defer server.Close()

	root := t.TempDir()
	config := `{
  "homeserver_url": "https://matrix.example.org",
  "user_id": "@bridge:example.org",
  "access_token": "token",
  "operator": "@operator:example.org",
  "forges": [{"root": "` + root + `", "name": "One", "dashboard_url": "` + server.URL + `"}]
}`
	err := checkTypedClarifications(writeConfig(t, config))
	if err == nil || !strings.Contains(err.Error(), "want family") {
		t.Fatalf("error = %v, want the Family identity refusal", err)
	}
}

func TestTypedClarificationPreflightExitDistinguishesEveryOutcome(t *testing.T) {
	if got := typedPreflightExit("false", "missing.json"); got != 2 {
		t.Fatalf("boolean exit = %d, want 2", got)
	}
	if got := typedPreflightExit("true", filepath.Join(t.TempDir(), "missing.json")); got != 1 {
		t.Fatalf("preflight failure exit = %d, want 1", got)
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("X-SwarmForge-Projection", dashboard.AutomationProjection)
		_ = json.NewEncoder(writer).Encode(preflightState("family", "One"))
	}))
	defer server.Close()
	root := t.TempDir()
	config := `{
  "homeserver_url": "https://matrix.example.org",
  "user_id": "@bridge:example.org",
  "access_token": "token",
  "operator": "@operator:example.org",
  "forges": [{"root": "` + root + `", "name": "One", "dashboard_url": "` + server.URL + `"}]
}`
	if got := typedPreflightExit("true", writeConfig(t, config)); got != 0 {
		t.Fatalf("success exit = %d, want 0", got)
	}
}

func preflightState(kind, name string) dashboard.AutomationState {
	section := func(count int) *dashboard.AutomationSection[string] {
		return &dashboard.AutomationSection[string]{Count: count, Items: make([]string, count)}
	}
	return dashboard.AutomationState{
		Projection: dashboard.AutomationProjection,
		Identity: &dashboard.AutomationIdentity{
			Kind: kind, Name: name, AllProjects: section(0), OpenProjects: section(0),
		},
		Approvals:         &dashboard.AutomationSection[dashboard.AutomationApproval]{},
		ResolvedApprovals: &dashboard.AutomationSection[dashboard.AutomationResolvedApproval]{},
		Clarifications: &dashboard.AutomationClarifications{
			Legacy: &dashboard.AutomationSection[dashboard.AutomationClarification]{},
			Typed:  &dashboard.AutomationSection[dashboard.AutomationClarification]{},
		},
		Projects: &dashboard.AutomationSection[dashboard.AutomationProject]{},
	}
}
