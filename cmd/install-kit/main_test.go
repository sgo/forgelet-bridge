package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureForge is a forge root the installer can read: one project with a roles
// file, and the lieutenant prompt that holds the forge's gate policy.
func fixtureForge(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	project := filepath.Join(root, "projects", "forgelet-bridge")
	if err := os.MkdirAll(filepath.Join(project, ".swarmforge"), 0o755); err != nil {
		t.Fatal(err)
	}
	roles := "coder\tcoder\t" + project + "\tfixture-coder\tCoder\tcodex\ttask\tforward-only\n"
	if err := os.WriteFile(filepath.Join(project, ".swarmforge", "roles.tsv"), []byte(roles), 0o644); err != nil {
		t.Fatal(err)
	}
	prompt := filepath.Join(root, "swarmforge", "roles", "lieutenant.prompt")
	if err := os.MkdirAll(filepath.Dir(prompt), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(prompt, []byte("Ask the operator before a card is created.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// fixtureKit is a kit of small tools that answer the way the self-checks read
// them, each naming the forge it is run against.
func fixtureKit(t *testing.T, forgeRoot string) string {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, "route_card.sh"), "#!/bin/sh\necho 'No proposal record for install-kit-self-check'\n")
	write(t, filepath.Join(dir, "route_card.bb"), "# the route gate\n")
	write(t, filepath.Join(dir, "role_health.sh"), "#!/bin/sh\necho 'coder idle-holding-card refund-card new=0 in_process=1'\n")
	write(t, filepath.Join(dir, "role_health.bb"), "# the idler check\n")
	write(t, filepath.Join(dir, "stall_watch.sh"), "#!/bin/sh\necho '<string>"+forgeRoot+"</string>'\n")
	write(t, filepath.Join(dir, "doorbell.sh"), "#!/bin/sh\necho 'doorbell: read the pane fixture-master of the role master'\n")
	write(t, filepath.Join(dir, "doorbell.bb"), "# the doorbell\n")
	return dir
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestRunRefusesWhenNoForgeRootIsNamed(t *testing.T) {
	err := run("", t.TempDir(), &strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), "--forge-root") {
		t.Fatalf("run = %v, want it to name the flag", err)
	}
}

func TestRunInstallsTheKitAndPrintsTheReport(t *testing.T) {
	forgeRoot := fixtureForge(t)
	var reported strings.Builder

	if err := run(forgeRoot, fixtureKit(t, forgeRoot), &reported); err != nil {
		t.Fatalf("run: %v", err)
	}

	if !strings.Contains(reported.String(), "installed the route gate, the idler check, the stall watch and the doorbell") {
		t.Errorf("the report does not say what it installed:\n%s", reported.String())
	}
	if !strings.Contains(reported.String(), "left the gate policy in the lieutenant prompt alone") {
		t.Errorf("the report does not say it left the policy alone:\n%s", reported.String())
	}
}

func TestRunFailsWhenASelfCheckReadsNothing(t *testing.T) {
	forgeRoot := fixtureForge(t)
	kitDir := fixtureKit(t, forgeRoot)
	// A gate that reads nothing says nothing at all: the report says so, and
	// the command's own status says so, which is what whoever ran it acts on.
	write(t, filepath.Join(kitDir, "route_card.sh"), "#!/bin/sh\nexit 0\n")
	var reported strings.Builder

	err := run(forgeRoot, kitDir, &reported)
	if err == nil || !strings.Contains(err.Error(), "a self-check failed") {
		t.Fatalf("run = %v, want a failed self-check reported", err)
	}
	if !strings.Contains(reported.String(), "self-check failed route gate") {
		t.Errorf("the report does not carry the failure:\n%s", reported.String())
	}
}
