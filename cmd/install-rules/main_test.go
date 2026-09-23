package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// markedRule is one rule the installer can install.
const markedRule = `<!-- bridge-rule: stopping-without-finishing -->
## Stopping Without Finishing

- If you stop while a card is assigned to you, raise a clarification first.
<!-- end: stopping-without-finishing -->
`

// rulesDir is a directory holding the one marked rule.
func rulesDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, "stopping-without-finishing.md"), markedRule)
	return dir
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunRefusesWhenNoForgeRootIsNamed(t *testing.T) {
	err := run("", rulesDir(t), &strings.Builder{})
	if err == nil {
		t.Fatal("run installed rules with no forge named")
	}
	if !strings.Contains(err.Error(), "--forge-root") {
		t.Errorf("error = %q, want it to name the flag", err)
	}
}

func TestRunInstallsTheRulesAndSaysWhatItDid(t *testing.T) {
	forgeRoot := t.TempDir()
	var reported strings.Builder

	if err := run(forgeRoot, rulesDir(t), &reported); err != nil {
		t.Fatalf("run: %v", err)
	}

	installed := filepath.Join(forgeRoot, "swarmforge", "constitution", "articles", "stopping-without-finishing.prompt")
	got, err := os.ReadFile(installed)
	if err != nil || strings.TrimSpace(string(got)) != strings.TrimSpace(markedRule) {
		t.Errorf("the installed rule = %q, %v; want the block", got, err)
	}
	if !strings.Contains(reported.String(), "changed stopping-without-finishing") {
		t.Errorf("reported = %q, want it to say what it changed", reported.String())
	}
}

func TestRunReportsAForgeRootThatIsNotThere(t *testing.T) {
	err := run(filepath.Join(t.TempDir(), "typo"), rulesDir(t), &strings.Builder{})
	if err == nil {
		t.Fatal("run reported success for a forge root that is not there")
	}
}

func TestRunReportsWhenTheRulesCannotBeRead(t *testing.T) {
	err := run(t.TempDir(), filepath.Join(t.TempDir(), "missing"), &strings.Builder{})
	if err == nil {
		t.Fatal("run reported success for a rules directory it could not read")
	}
}
