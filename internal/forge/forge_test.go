package forge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenProjectsReadsTheForgeList(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".swarmforge"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".swarmforge", "open-projects"), []byte("forgelet-bridge\n\n  saibill  \n"), 0o644); err != nil {
		t.Fatal(err)
	}

	projects, err := OpenProjects(root)
	if err != nil {
		t.Fatalf("OpenProjects: %v", err)
	}
	if len(projects) != 2 || projects[0] != "forgelet-bridge" || projects[1] != "saibill" {
		t.Errorf("projects = %v, want the trimmed names", projects)
	}
}

func TestOpenProjectsOfAForgeWithoutAListIsEmpty(t *testing.T) {
	projects, err := OpenProjects(t.TempDir())
	if err != nil {
		t.Fatalf("OpenProjects: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("projects = %v, want none", projects)
	}
}

func TestProjectDir(t *testing.T) {
	if got := ProjectDir("/forge-a", "forgelet-bridge"); got != filepath.Join("/forge-a", "projects", "forgelet-bridge") {
		t.Errorf("ProjectDir = %q", got)
	}
}
