package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ruleText is one marked block, the way the repository ships it.
const ruleText = `<!-- bridge-rule: stopping-without-finishing -->
## Stopping Without Finishing

- If you stop while a card is assigned to you, raise a clarification first.
<!-- end: stopping-without-finishing -->
`

// forge is a fixture forge with its own constitution, two packs, and a project
// mid-card.
func forge(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, filepath.Join(root, "swarmforge", "constitution.prompt"), "This forge says what it likes.\n")
	write(t, filepath.Join(root, "swarmforge", "constitution", "articles", "project.prompt"), "Go, and its own tooling.\n")
	for _, pack := range []string{"four-pack", "two-pack"} {
		write(t, filepath.Join(root, "packs", pack, "swarmforge", "constitution.prompt"), pack+" says its own thing.\n")
	}
	write(t, filepath.Join(root, "projects", "forgelet-bridge", "swarmforge", "constitution.prompt"), "tracked copy\n")
	write(t, filepath.Join(root, "projects", "forgelet-bridge", "tmp", "question.txt"), "mid-card\n")
	return root
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

func load(t *testing.T) []Rule {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, "stopping-without-finishing.md"), ruleText)
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return loaded
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestInstallWritesTheRuleIntoTheConstitutionAndThePacks(t *testing.T) {
	root := forge(t)

	report, err := Install(root, load(t))
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	for _, where := range []string{"", "packs/four-pack", "packs/two-pack"} {
		path := filepath.Join(root, where, "swarmforge", "constitution", "articles", "stopping-without-finishing.prompt")
		if got := read(t, path); strings.TrimSpace(got) != strings.TrimSpace(ruleText) {
			t.Errorf("the rule in %s = %q, want the block", path, got)
		}
	}
	if len(report.Changed) != 3 {
		t.Errorf("changed = %v, want the constitution and both packs", report.Changed)
	}
	if !strings.Contains(report.String(), "changed stopping-without-finishing") {
		t.Errorf("report = %q, want it to say the rule changed", report.String())
	}
	if !strings.Contains(report.String(), "left alone") || !strings.Contains(report.String(), "projects") {
		t.Errorf("report = %q, want it to say the projects were left alone", report.String())
	}
}

func TestInstallKeepsTheWordingTheForgeWroteItself(t *testing.T) {
	root := forge(t)
	before := map[string]string{}
	for _, path := range []string{
		filepath.Join(root, "swarmforge", "constitution.prompt"),
		filepath.Join(root, "swarmforge", "constitution", "articles", "project.prompt"),
		filepath.Join(root, "packs", "four-pack", "swarmforge", "constitution.prompt"),
		filepath.Join(root, "projects", "forgelet-bridge", "swarmforge", "constitution.prompt"),
	} {
		before[path] = read(t, path)
	}

	if _, err := Install(root, load(t)); err != nil {
		t.Fatalf("Install: %v", err)
	}

	for path, want := range before {
		if got := read(t, path); got != want {
			t.Errorf("%s = %q, want the forge's own wording %q", path, got, want)
		}
	}
}

func TestInstallAgainChangesNothing(t *testing.T) {
	root := forge(t)
	if _, err := Install(root, load(t)); err != nil {
		t.Fatalf("first Install: %v", err)
	}

	report, err := Install(root, load(t))
	if err != nil {
		t.Fatalf("second Install: %v", err)
	}

	if len(report.Changed) != 0 {
		t.Errorf("changed = %v, want nothing changed the second time", report.Changed)
	}
	if len(report.Current) != 3 {
		t.Errorf("current = %v, want the rule current in all three places", report.Current)
	}
	if !strings.Contains(report.String(), "already current stopping-without-finishing") {
		t.Errorf("report = %q, want it to say the rule was already current", report.String())
	}
}

func TestInstallRefreshesABlockThatDrifted(t *testing.T) {
	root := forge(t)
	if _, err := Install(root, load(t)); err != nil {
		t.Fatalf("Install: %v", err)
	}
	stale := strings.Replace(ruleText, "raise a clarification first", "go quiet", 1)
	write(t, filepath.Join(root, "swarmforge", "constitution", "articles", "stopping-without-finishing.prompt"), stale)

	report, err := Install(root, load(t))
	if err != nil {
		t.Fatalf("second Install: %v", err)
	}

	path := filepath.Join(root, "swarmforge", "constitution", "articles", "stopping-without-finishing.prompt")
	if got := read(t, path); got != ruleText {
		t.Errorf("the rule = %q, want the block refreshed", got)
	}
	if len(report.Changed) != 1 || !strings.Contains(report.Changed[0], "forge's own constitution") {
		t.Errorf("changed = %v, want only the drifted copy refreshed", report.Changed)
	}
	if len(report.Current) != 2 {
		t.Errorf("current = %v, want the two packs already current", report.Current)
	}
}

func TestInstallLeavesAFileItDidNotWriteAlone(t *testing.T) {
	root := forge(t)
	foreign := filepath.Join(root, "swarmforge", "constitution", "articles", "stopping-without-finishing.prompt")
	write(t, foreign, "The forge's own wording about stopping.\n")

	report, err := Install(root, load(t))
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	if got := read(t, foreign); got != "The forge's own wording about stopping.\n" {
		t.Errorf("%s = %q, want the forge's own file left as it was", foreign, got)
	}
	if len(report.LeftAlone) != 2 { // the foreign file, and the projects
		t.Errorf("left alone = %v, want the foreign file and the projects", report.LeftAlone)
	}
}

func TestLoadRefusesABlockThatIsNotMarked(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "loose.md"), "# A rule with no markers\n")

	if _, err := Load(dir); err == nil {
		t.Fatal("Load accepted a rule with no markers")
	}
}
