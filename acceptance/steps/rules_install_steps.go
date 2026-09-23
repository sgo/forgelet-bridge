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
	return installerSaid(w.adapterOutput, "changed", captures[1])
}

// installerAlreadyCurrent checks the installer said a rule was already current.
func installerAlreadyCurrent(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	return installerSaid(w.adapterOutput, "already current", captures[1])
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
	return installerSaid(w.adapterOutput, "left alone", projects)
}

// installerSaid checks the installer's output carried both a phrase about a
// result and what that result was about.
func installerSaid(output, phrase, about string) error {
	if !strings.Contains(output, phrase) || !strings.Contains(output, about) {
		return fmt.Errorf("the installer's output does not say %q about %s:\n%s", phrase, about, output)
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
