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

func TestLoadIgnoresWhatIsNotARuleFile(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "stopping-without-finishing.md"), ruleText)
	// A note beside the rules, and a directory whose name merely ends in .md:
	// neither is a rule, and neither may stop the installer.
	write(t, filepath.Join(dir, "notes.txt"), "not a rule\n")
	if err := os.MkdirAll(filepath.Join(dir, "old.md"), 0o755); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != 1 || loaded[0].Subject != "stopping-without-finishing" {
		t.Errorf("loaded = %+v, want the one rule file", loaded)
	}
}

func TestInstallLeavesAHalfMarkedFileAlone(t *testing.T) {
	root := forge(t)
	half := filepath.Join(root, "swarmforge", "constitution", "articles", "stopping-without-finishing.prompt")
	// The forge started from the block and wrote its own wording into it: the
	// installer owns a block it can recognise end to end, and nothing else.
	write(t, half, "<!-- bridge-rule: stopping-without-finishing -->\nThe forge's own wording from here.\n")

	if _, err := Install(root, load(t)); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if got := read(t, half); got != "<!-- bridge-rule: stopping-without-finishing -->\nThe forge's own wording from here.\n" {
		t.Errorf("%s = %q, want the forge's own file left as it was", half, got)
	}
}

func TestInstallReportsARuleItCannotWrite(t *testing.T) {
	root := forge(t)
	if _, err := Install(root, load(t)); err != nil {
		t.Fatalf("first Install: %v", err)
	}
	drifted := filepath.Join(root, "swarmforge", "constitution", "articles", "stopping-without-finishing.prompt")
	write(t, drifted, strings.Replace(ruleText, "raise a clarification first", "go quiet", 1))
	if err := os.Chmod(drifted, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(drifted, 0o644) })

	if _, err := Install(root, load(t)); err == nil {
		t.Error("Install reported success for a rule it could not write")
	}
}

func TestInstallRefusesAForgeRootThatIsNotThere(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "typo")

	if _, err := Install(missing, load(t)); err == nil {
		t.Error("Install reported success for a forge root that is not there")
	}
	if _, err := os.Stat(filepath.Join(missing, "swarmforge")); !os.IsNotExist(err) {
		t.Errorf("Install left a tree behind at %s: %v", missing, err)
	}
}

func TestInstallRefusesAForgeRootThatIsNotADirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "forge")
	write(t, root, "not a forge\n")

	if _, err := Install(root, load(t)); err == nil {
		t.Error("Install reported success for a forge root that is a file")
	}
}

func TestLoadReadsTheRulesInSubjectOrder(t *testing.T) {
	dir := t.TempDir()
	// The files are named the other way round, so only their subjects can put
	// them in this order.
	write(t, filepath.Join(dir, "aaa.md"), "<!-- bridge-rule: work-holds-you -->\n## Work holds you\n<!-- end: work-holds-you -->\n")
	write(t, filepath.Join(dir, "zzz.md"), ruleText)

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != 2 || loaded[0].Subject != "stopping-without-finishing" || loaded[1].Subject != "work-holds-you" {
		t.Errorf("loaded = %+v, want the rules in subject order", loaded)
	}
}

func TestLoadReportsWhatItCannotRead(t *testing.T) {
	notADirectory := filepath.Join(t.TempDir(), "rules")
	write(t, notADirectory, "not a directory\n")
	if _, err := Load(notADirectory); err == nil {
		t.Error("Load reported success for a rules path that is not a directory")
	}

	dir := t.TempDir()
	unreadable := filepath.Join(dir, "stopping-without-finishing.md")
	write(t, unreadable, ruleText)
	if err := os.Chmod(unreadable, 0o200); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o644) })
	if _, err := Load(dir); err == nil {
		t.Error("Load reported success for a rule file it could not read")
	}
}

func TestParseRefusesABlockItCannotRead(t *testing.T) {
	cases := map[string]string{
		"a block that names no subject":     "<!-- bridge-rule:  -->\n## Nothing named\n<!-- end:  -->\n",
		"an end marker for another subject": "<!-- bridge-rule: a-subject -->\n## A subject\n<!-- end: another-subject -->\n",
	}
	for what, block := range cases {
		if _, err := Parse(block); err == nil {
			t.Errorf("Parse accepted %s", what)
		}
	}
}

func TestInstallReportsAPacksPathItCannotRead(t *testing.T) {
	root := forge(t)
	packs := filepath.Join(root, "packs")
	if err := os.RemoveAll(packs); err != nil {
		t.Fatal(err)
	}
	write(t, packs, "not a directory\n")

	if _, err := Install(root, load(t)); err == nil {
		t.Error("Install reported success for a forge whose packs cannot be read")
	}
}

func TestInstallIgnoresWhatIsNotAPack(t *testing.T) {
	root := forge(t)
	write(t, filepath.Join(root, "packs", "a-note.txt"), "not a pack\n")

	report, err := Install(root, load(t))
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(report.Changed) != 3 {
		t.Errorf("changed = %v, want the constitution and the two packs", report.Changed)
	}
}

func TestInstallReportsAnArticlePathItCannotCreate(t *testing.T) {
	root := forge(t)
	constitution := filepath.Join(root, "swarmforge", "constitution")
	if err := os.RemoveAll(constitution); err != nil {
		t.Fatal(err)
	}
	write(t, constitution, "not a directory\n")

	if _, err := Install(root, load(t)); err == nil {
		t.Error("Install reported success for an articles path it could not create")
	}
}

func TestInstallReportsARulePathItCannotRead(t *testing.T) {
	root := forge(t)
	// A directory where the rule file goes is not a rule the installer can
	// read or overwrite.
	if err := os.MkdirAll(filepath.Join(root, "swarmforge", "constitution", "articles", "stopping-without-finishing.prompt"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := Install(root, load(t)); err == nil {
		t.Error("Install reported success for a rule path it could not read")
	}
}

func TestInstallReportsAnArticleDirectoryItCannotMake(t *testing.T) {
	root := forge(t)
	constitution := filepath.Join(root, "swarmforge", "constitution")
	if err := os.RemoveAll(filepath.Join(constitution, "articles")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(constitution, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(constitution, 0o755) })

	if _, err := Install(root, load(t)); err == nil {
		t.Error("Install reported success for an article directory it could not make")
	}
}
