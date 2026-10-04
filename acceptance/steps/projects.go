package steps

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// forgeClosesProject takes a project out of the forge's open list, the way the
// dashboard's Close does: the bridge then has no board of its own to carry for
// it, and the board's state is cleared.
func forgeClosesProject(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := singleForge(w)
	if err != nil {
		return err
	}
	return closeProject(root, captures[1])
}

// closeProject removes a project from the forge's open list.
func closeProject(root, project string) error {
	path := filepath.Join(root, ".swarmforge", "open-projects")
	existing, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var kept []string
	for _, line := range strings.Split(string(existing), "\n") {
		name := strings.TrimSpace(line)
		if name == "" || name == project {
			continue
		}
		kept = append(kept, name)
	}
	content := ""
	if len(kept) > 0 {
		content = strings.Join(kept, "\n") + "\n"
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
