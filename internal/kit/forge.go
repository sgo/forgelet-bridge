package kit

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// forgeDir is the forge root as a full path that is a directory. Everything the
// report names, and every tool the installer runs, reads the forge by its full
// path: a tool that resolves the root itself would otherwise name a different
// place in the report than the one the installer worked on.
func forgeDir(forgeRoot string) (string, error) {
	absolute, err := filepath.Abs(forgeRoot)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", fmt.Errorf("the forge %s is not there: %w", absolute, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("the forge %s is not a directory", absolute)
	}
	return absolute, nil
}

// projects are the projects a forge root serves, in name order.
func projects(forgeRoot string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(forgeRoot, "projects"))
	if err != nil {
		return nil, err
	}
	var found []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		project := filepath.Join(forgeRoot, "projects", entry.Name())
		if _, err := os.Stat(filepath.Join(project, ".swarmforge", "roles.tsv")); err != nil {
			continue
		}
		found = append(found, project)
	}
	sort.Strings(found)
	return found, nil
}

// started says whether the forge has been started at all: its own roles file is
// what the first start writes down, and what the tools the kit ships read a
// forge root by. A forge that has been composed but never started holds none,
// so there a tool that can prove nothing has proved all there is to prove.
func started(forgeRoot string) bool {
	_, err := os.Stat(filepath.Join(forgeRoot, ".swarmforge", "roles.tsv"))
	return err == nil
}

// unstartedProjects are the projects a forge holds and has not started: the
// project's own composition is there and the roles file a start writes is not.
// They are what the check names when it reads a forge whose projects have never
// run, in name order.
func unstartedProjects(forgeRoot string) []string {
	entries, err := os.ReadDir(filepath.Join(forgeRoot, "projects"))
	if err != nil {
		return nil
	}
	var found []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		project := filepath.Join(forgeRoot, "projects", entry.Name())
		if !composed(project) {
			continue
		}
		if _, err := os.Stat(filepath.Join(project, ".swarmforge", "roles.tsv")); err == nil {
			continue
		}
		found = append(found, project)
	}
	sort.Strings(found)
	return found
}

// composed says whether a directory under projects/ holds a project: composing
// one leaves its own swarmforge/ beside it, so a directory without one is not a
// project the forge serves and does not excuse an install that read nothing.
func composed(project string) bool {
	info, err := os.Stat(filepath.Join(project, "swarmforge"))
	return err == nil && info.IsDir()
}

// paneOf is the pane one role of a project is served in, as the project's own
// roles file records it.
func paneOf(project, role string) string {
	data, err := os.ReadFile(filepath.Join(project, ".swarmforge", "roles.tsv"))
	if err != nil {
		return "-"
	}
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		columns := strings.Split(line, "\t")
		if len(columns) >= 4 && columns[0] == role {
			return columns[3]
		}
	}
	return "-"
}

// run runs one installed tool and keeps everything it said. A tool that refuses
// is often saying exactly what the self-check wants to read, so the exit status
// is the caller's to judge.
func run(command string, args ...string) (string, error) {
	out, err := exec.Command(command, args...).CombinedOutput()
	return string(out), err
}
