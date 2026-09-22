// Package forge reads what a forge says about its projects: which ones are
// open, and where each project keeps its own state.
package forge

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	// ProjectsDir is where a forge keeps its projects.
	ProjectsDir = "projects"
	// OpenProjectsFile lists the projects whose work the operator is following.
	OpenProjectsFile = ".swarmforge/open-projects"
)

// OpenProjects lists the forge's open projects.
func OpenProjects(root string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(OpenProjectsFile)))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var projects []string
	for _, line := range strings.Split(string(data), "\n") {
		if name := strings.TrimSpace(line); name != "" {
			projects = append(projects, name)
		}
	}
	return projects, nil
}

// ProjectDir is where one project lives inside a forge.
func ProjectDir(root, project string) string {
	return filepath.Join(root, ProjectsDir, project)
}
