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

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-22T16:09:06+02:00","module_hash":"8e837b1abc8c511eaac241b97251f299073ea30cd8d6d5e2d8224e913a458e78","functions":[{"id":"func/OpenProjects","name":"OpenProjects","line":19,"end_line":34,"hash":"617a5db3257f35ee1083392743181280f1d8b0ce53d4bda9baac72a9f8e8d7a2"},{"id":"func/ProjectDir","name":"ProjectDir","line":37,"end_line":39,"hash":"834388830e2f721e992c81f696bbcd32baeec7ccc62e659ddf05d34cb7671b18"}]}
// mutate4go-manifest-end
