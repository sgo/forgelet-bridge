package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
