package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/unclebob/forgelet-bridge/internal/bridge"
	"github.com/unclebob/forgelet-bridge/internal/dashboard"
)

// forgeProjects discovers the single-agent projects a forge currently has
// open. The forge's own open-project list is the source of truth; the marker
// in the project root is what distinguishes a single-agent project from a
// pack project.
type forgeProjects struct{}

func (forgeProjects) Projects(forgeRoot string) ([]bridge.Project, error) {
	file := filepath.Join(forgeRoot, ".swarmforge", "open-projects")
	data, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var projects []bridge.Project
	for _, line := range strings.Split(string(data), "\n") {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		root := filepath.Join(forgeRoot, "projects", name)
		marker := filepath.Join(root, ".swarmforge", "single-agent")
		if _, err := os.Stat(marker); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		projects = append(projects, bridge.Project{
			Name: name,
			Root: root,
			Store: dashboard.Queue{
				Store: dashboard.New(root),
				Root:  root,
			},
			Clarifications: dashboard.Clarifications{
				Root:    root,
				Project: name,
			},
		})
	}
	return projects, nil
}
