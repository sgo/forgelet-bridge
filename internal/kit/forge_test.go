package kit

import (
	"os"
	"path/filepath"
	"testing"
)

// composeProject lays out a project the forge composed: its own swarmforge/
// beside it, which is what composing one leaves and what the forge reads a
// project by.
func composeProject(t *testing.T, forgeRoot, project string) string {
	t.Helper()
	dir := filepath.Join(forgeRoot, "projects", project)
	if err := os.MkdirAll(filepath.Join(dir, "swarmforge"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// startProject writes what starting one project writes down: the roles file the
// forge and the tools the kit ships read a started project by.
func startProject(t *testing.T, project string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(project, ".swarmforge"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".swarmforge", "roles.tsv"),
		[]byte("coder\tcoder\tx\tpane\tC\ttask\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// servedProject lays out one more project beside the ones a fixture forge
// already serves, named by the test: the roles file a start writes is what makes
// a directory under projects/ a project the check runs over, in name order.
func servedProject(t *testing.T, forgeRoot, project string) string {
	t.Helper()
	dir := filepath.Join(forgeRoot, "projects", project)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	startProject(t, dir)
	return dir
}

func TestProjectsNamesTheProjectsThatHaveRolesFiles(t *testing.T) {
	root := t.TempDir()
	withRoles := filepath.Join(root, "projects", "forgelet-bridge", ".swarmforge")
	withoutRoles := filepath.Join(root, "projects", "empty", ".swarmforge")
	for _, dir := range []string{withRoles, withoutRoles} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(withRoles, "roles.tsv"), []byte("coder\tcoder\tx\tpane\tC\ttask\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	found, err := projects(root)
	if err != nil {
		t.Fatalf("projects: %v", err)
	}
	if len(found) != 1 || filepath.Base(found[0]) != "forgelet-bridge" {
		t.Errorf("projects = %v, want only the project with a roles file", found)
	}
}

func TestProjectsSkipsWhatIsNotAProject(t *testing.T) {
	root := fixtureForge(t)
	if err := os.MkdirAll(filepath.Join(root, "projects", "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "projects", "loose.txt"), []byte("not a project\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	found, err := projects(root)
	if err != nil {
		t.Fatalf("projects: %v", err)
	}
	if len(found) != 1 || filepath.Base(found[0]) != "forgelet-bridge" {
		t.Errorf("projects = %v, want only the project with a roles file", found)
	}
}

// TestUnstartedProjectsNamesTheProjectsAStartHasNotWritten is the other reading
// of the same forge: a project the forge composed and never started, and only
// that. A project that has been started is served, a directory the forge never
// composed is not a project, and a loose file beside them is not one either.
func TestUnstartedProjectsNamesTheProjectsAStartHasNotWritten(t *testing.T) {
	root := t.TempDir()
	served := composeProject(t, root, "forgelet-bridge")
	startProject(t, served)
	held := composeProject(t, root, "forgelet-app")
	if err := os.MkdirAll(filepath.Join(root, "projects", "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "projects", "loose.txt"), []byte("not a project\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	found := unstartedProjects(root)

	if len(found) != 1 || found[0] != held {
		t.Errorf("unstartedProjects = %v, want only %s", found, held)
	}
}
