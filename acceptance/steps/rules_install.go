package steps

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/unclebob/forgelet-bridge/internal/rules"
)

// subject is the rule the features install.
const installedRule = "stopping-without-finishing"

// forgeWrote is the wording a fixture forge wrote for itself, which an install
// must keep.
const forgeWrote = "This forge switched its language and tooling to Go, and says so here.\n"

func ruleArticle(root, where string) string {
	dir := filepath.Join(root, "swarmforge", "constitution", "articles")
	if where != "" {
		dir = filepath.Join(root, where, "swarmforge", "constitution", "articles")
	}
	return filepath.Join(dir, installedRule+".prompt")
}

// forgeWroteItsOwnConstitution gives a fixture forge a constitution of its own:
// the text it wrote, and a project it is working on.
func forgeWroteItsOwnConstitution(ctx context.Context, world any, captures []string) error {
	w := world.(*World)
	store, err := w.forge(ctx, captures[1])
	if err != nil {
		return err
	}
	root := store.Root()
	for path, content := range map[string]string{
		filepath.Join(root, "swarmforge", "constitution.prompt"):                        forgeWrote,
		filepath.Join(root, "swarmforge", "constitution", "articles", "project.prompt"): forgeWrote,
	} {
		if err := writeFile(path, content); err != nil {
			return err
		}
	}
	return nil
}

// forgeScaffoldsFromAPack gives a fixture forge a pack its projects come from,
// with wording of its own.
func forgeScaffoldsFromAPack(ctx context.Context, world any, captures []string) error {
	w := world.(*World)
	store, err := w.forge(ctx, captures[1])
	if err != nil {
		return err
	}
	root := filepath.Join(store.Root(), "packs", captures[2])
	for path, content := range map[string]string{
		filepath.Join(root, "swarmforge", "constitution.prompt"):                        forgeWrote,
		filepath.Join(root, "swarmforge", "constitution", "articles", "project.prompt"): forgeWrote,
		filepath.Join(root, "swarmforge", "swarmforge.conf"):                            "pack " + captures[2] + "\n",
	} {
		if err := writeFile(path, content); err != nil {
			return err
		}
	}
	return nil
}

// projectIsMidCard gives a fixture forge a project with work in flight and its
// own tracked copy of the forge's rules.
func projectIsMidCard(ctx context.Context, world any, captures []string) error {
	w := world.(*World)
	store, err := w.forge(ctx, captures[2])
	if err != nil {
		return err
	}
	root := filepath.Join(store.Root(), "projects", captures[1])
	for path, content := range map[string]string{
		filepath.Join(root, "swarmforge", "constitution.prompt"): "This project's own tracked copy.\n",
		filepath.Join(root, "tmp", "question.txt"):               "which lane should the refund card start in?\n",
		filepath.Join(root, "features", "work.feature"):          "Feature: work in flight\n",
	} {
		if err := writeFile(path, content); err != nil {
			return err
		}
	}
	return nil
}

// writeFile writes a fixture file, making its directory.
func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// adapterInstallsRules runs the served forge root's adapter, which installs the
// rules the bridge's rooms rely on, and remembers the tree it left behind.
func adapterInstallsRules(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	served, err := w.declaredForge(captures[1])
	if err != nil {
		return err
	}
	w.servedRoot = served.Root()
	if _, err := w.adapterCommand(ctx, "install-rules"); err != nil {
		return fmt.Errorf("the adapter's install-rules failed: %w\n%s", err, w.adapterOutput)
	}
	snapshot, err := snapshotTree(served.Root())
	if err != nil {
		return err
	}
	w.rulesSnapshot = snapshot
	return nil
}

// installerChangedRule checks the installer said it changed a rule.
func installerChangedRule(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	if !strings.Contains(w.adapterOutput, "changed") || !strings.Contains(w.adapterOutput, captures[1]) {
		return fmt.Errorf("the installer's output does not say it changed %s:\n%s", captures[1], w.adapterOutput)
	}
	return nil
}

// installerAlreadyCurrent checks the installer said a rule was already current.
func installerAlreadyCurrent(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	if !strings.Contains(w.adapterOutput, "already current") || !strings.Contains(w.adapterOutput, captures[1]) {
		return fmt.Errorf("the installer's output does not say %s was already current:\n%s", captures[1], w.adapterOutput)
	}
	return nil
}

// installerLeftProjectsAlone checks the installer said it left the forge's
// projects alone.
func installerLeftProjectsAlone(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	served, err := w.declaredForge(captures[1])
	if err != nil {
		return err
	}
	projects := filepath.Join(served.Root(), "projects")
	output := w.adapterOutput
	if !strings.Contains(output, "left alone") || !strings.Contains(output, projects) {
		return fmt.Errorf("the installer's output does not say it left %s alone:\n%s", projects, output)
	}
	return nil
}

// installerChangedNothing checks the forge root is exactly the tree the last
// install left behind.
func installerChangedNothing(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	served, err := w.declaredForge(captures[1])
	if err != nil {
		return err
	}
	now, err := snapshotTree(served.Root())
	if err != nil {
		return err
	}
	if now != w.rulesSnapshot {
		return fmt.Errorf("the forge root %s is not the tree the install left:\n%s", served.Root(), diffTrees(w.rulesSnapshot, now))
	}
	return nil
}

// constitutionCarriesRule checks the forge's own constitution carries the rule.
func constitutionCarriesRule(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	served, err := w.declaredForge(captures[1])
	if err != nil {
		return err
	}
	return carriesRule(ruleArticle(served.Root(), ""), captures[2])
}

// packsCarryRule checks every pack the forge scaffolds from carries the rule.
func packsCarryRule(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	served, err := w.declaredForge(captures[1])
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(filepath.Join(served.Root(), "packs"))
	if err != nil {
		return err
	}
	carried := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if err := carriesRule(ruleArticle(served.Root(), filepath.Join("packs", entry.Name())), captures[2]); err != nil {
			return err
		}
		carried++
	}
	if carried == 0 {
		return fmt.Errorf("the forge root %s scaffolds from no pack", served.Root())
	}
	return nil
}

// carriesRule checks one file holds the rule's marked block.
func carriesRule(path, subject string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	rule, err := rules.Parse(string(data))
	if err != nil {
		return fmt.Errorf("%s does not carry the rule: %w", path, err)
	}
	if rule.Subject != subject {
		return fmt.Errorf("%s carries %s, want %s", path, rule.Subject, subject)
	}
	return nil
}

// forgeKeepsItsWording checks the forge still says what it wrote itself.
func forgeKeepsItsWording(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	served, err := w.declaredForge(captures[1])
	if err != nil {
		return err
	}
	for _, path := range []string{
		filepath.Join(served.Root(), "swarmforge", "constitution.prompt"),
		filepath.Join(served.Root(), "swarmforge", "constitution", "articles", "project.prompt"),
		filepath.Join(served.Root(), "packs", "four-pack", "swarmforge", "constitution.prompt"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if string(data) != forgeWrote {
			return fmt.Errorf("%s = %q, want the forge's own wording", path, data)
		}
	}
	return nil
}

// projectIsTheTreeItWas checks one project's own tree is untouched.
func projectIsTheTreeItWas(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	served, err := w.declaredForge(captures[2])
	if err != nil {
		return err
	}
	root := filepath.Join(served.Root(), "projects", captures[1])
	now, err := snapshotTree(root)
	if err != nil {
		return err
	}
	for _, path := range []string{
		filepath.Join(root, "swarmforge", "constitution.prompt"),
		filepath.Join(root, "tmp", "question.txt"),
	} {
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("the project's own file %s is gone: %v", path, err)
		}
	}
	if strings.Contains(now, installedRule+".prompt") {
		return fmt.Errorf("the install reached into the project's own tree:\n%s", now)
	}
	return nil
}

// ruleHasGoneStale makes the installed rule drift, the way a hand-edited copy
// would.
func ruleHasGoneStale(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	served, err := w.declaredForge(captures[2])
	if err != nil {
		return err
	}
	path := ruleArticle(served.Root(), "")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	stale := strings.Replace(string(data), "raise a clarification", "go quiet and hope", 1)
	if stale == string(data) {
		return fmt.Errorf("%s does not carry the rule's wording", path)
	}
	return os.WriteFile(path, []byte(stale), 0o644)
}

// snapshotTree is a forge root's tree as one string: every file's path and
// contents, so a change anywhere shows up.
func snapshotTree(root string) (string, error) {
	var lines []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		lines = append(lines, fmt.Sprintf("%s %x", relative, sha256.Sum256(data)))
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n"), nil
}

// diffTrees is the lines one snapshot has that the other does not.
func diffTrees(before, after string) string {
	seen := map[string]bool{}
	for _, line := range strings.Split(before, "\n") {
		seen[line] = true
	}
	var difference []string
	for _, line := range strings.Split(after, "\n") {
		if !seen[line] {
			difference = append(difference, "+ "+line)
		}
	}
	return strings.Join(difference, "\n")
}
