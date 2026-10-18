package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestForgeProjectsFindsOnlyMarkedOpenProjects(t *testing.T) {
	root := t.TempDir()
	write := func(path, text string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, ".swarmforge", "open-projects"), "pack\nagent\n")
	if err := os.MkdirAll(filepath.Join(root, "projects", "pack"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(root, "projects", "agent", ".swarmforge", "single-agent"), "agent\n")

	projects, err := (forgeProjects{}).Projects(root)
	if err != nil {
		t.Fatalf("Projects: %v", err)
	}
	if len(projects) != 1 || projects[0].Name != "agent" {
		t.Fatalf("projects = %+v, want only the marked project", projects)
	}
	if projects[0].Root != filepath.Join(root, "projects", "agent") {
		t.Errorf("root = %q", projects[0].Root)
	}
	if projects[0].Store == nil || projects[0].Clarifications == nil {
		t.Error("the project has no dashboard queue or clarification store")
	}
}

func TestForgeProjectsIsEmptyWithoutAnOpenProjectList(t *testing.T) {
	projects, err := (forgeProjects{}).Projects(t.TempDir())
	if err != nil {
		t.Fatalf("Projects: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("projects = %+v, want none", projects)
	}
}
