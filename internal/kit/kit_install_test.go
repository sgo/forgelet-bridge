package kit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeScript writes an executable fixture file.
func writeScript(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// fixtureForge is a forge root with one project the tools can read, and the
// lieutenant prompt that holds its gate policy.
func fixtureForge(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	project := filepath.Join(root, "projects", "forgelet-bridge")
	if err := os.MkdirAll(filepath.Join(project, ".swarmforge"), 0o755); err != nil {
		t.Fatal(err)
	}
	roles := "master\tmaster\t" + project + "\tfixture-master\tMaster\tcodex\ttask\tforward-only\n" +
		"coder\tcoder\t" + filepath.Join(project, "worktrees", "coder") + "\tfixture-coder\tCoder\tcodex\ttask\tforward-only\n"
	if err := os.WriteFile(filepath.Join(project, ".swarmforge", "roles.tsv"), []byte(roles), 0o644); err != nil {
		t.Fatal(err)
	}
	prompt := filepath.Join(root, "swarmforge", "roles", "lieutenant.prompt")
	if err := os.MkdirAll(filepath.Dir(prompt), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(prompt, []byte("Ask the operator before a card is created.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// fixtureKit is a kit of small tools that answer the way the self-checks read
// them, each naming the forge it is run against.
func fixtureKit(t *testing.T, forgeRoot string) string {
	t.Helper()
	dir := t.TempDir()
	writeScript(t, filepath.Join(dir, "route_card.sh"), "#!/bin/sh\necho 'No proposal record for install-kit-self-check'\n")
	writeScript(t, filepath.Join(dir, "route_card.bb"), "# the route gate\n")
	writeScript(t, filepath.Join(dir, "role_health.sh"), "#!/bin/sh\necho 'coder idle-holding-card refund-card new=0 in_process=1'\n")
	writeScript(t, filepath.Join(dir, "role_health.bb"), "# the idler check\n")
	writeScript(t, filepath.Join(dir, "stall_watch.sh"), "#!/bin/sh\necho '<string>"+forgeRoot+"</string>'\n")
	return dir
}

func TestInstallCopiesTheKitAndSelfChecksIt(t *testing.T) {
	root := fixtureForge(t)

	report, err := Install(root, fixtureKit(t, root))
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if report.Failed() {
		t.Errorf("a kit whose tools all read the forge was reported as failed:\n%s", report)
	}
	for _, want := range []string{
		"changed route gate",
		"installed the route gate, the idler check and the stall watch",
		"self-check route gate: ran",
		"self-check idler check: ran",
		"self-check stall watch: ran",
		"wrote the stall watch's agent",
		"left alone loading the stall watch's agent",
		"left the gate policy in the lieutenant prompt alone",
	} {
		if !strings.Contains(report.String(), want) {
			t.Errorf("the report does not carry %q:\n%s", want, report)
		}
	}
	scripts := filepath.Join(root, "swarmforge", "scripts")
	for file, mode := range map[string]os.FileMode{
		"route_card.sh":  0o755,
		"route_card.bb":  0o644,
		"stall_watch.sh": 0o755,
	} {
		info, err := os.Stat(filepath.Join(scripts, file))
		if err != nil {
			t.Fatalf("%s was not installed: %v", file, err)
		}
		if info.Mode().Perm() != mode {
			t.Errorf("%s mode = %v, want %v", file, info.Mode().Perm(), mode)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".swarmforge", "stall-watch.plist")); err != nil {
		t.Errorf("the agent was not written: %v", err)
	}
}

func TestInstallLeavesACurrentKitAlone(t *testing.T) {
	root := fixtureForge(t)
	kitDir := fixtureKit(t, root)
	if _, err := Install(root, kitDir); err != nil {
		t.Fatalf("first Install: %v", err)
	}

	report, err := Install(root, kitDir)
	if err != nil {
		t.Fatalf("second Install: %v", err)
	}
	if report.Failed() {
		t.Errorf("a second install failed:\n%s", report)
	}
	for _, want := range []string{
		"already current route gate",
		"the kit was already current in ",
		"already current the stall watch's agent",
	} {
		if !strings.Contains(report.String(), want) {
			t.Errorf("the second report does not carry %q:\n%s", want, report)
		}
	}
}

func TestInstallReportsAToolThatReadsNothing(t *testing.T) {
	root := fixtureForge(t)
	kitDir := fixtureKit(t, root)
	// A gate that reads nothing says nothing at all.
	writeScript(t, filepath.Join(kitDir, "route_card.sh"), "#!/bin/sh\nexit 0\n")

	report, err := Install(root, kitDir)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !report.Failed() {
		t.Errorf("a gate that read nothing was not reported as failed:\n%s", report)
	}
	if !strings.Contains(report.String(), "self-check failed route gate") {
		t.Errorf("the report does not name the gate that read nothing:\n%s", report)
	}
}

func TestInstallReportsAWatchThatNamesNoForge(t *testing.T) {
	root := fixtureForge(t)
	kitDir := fixtureKit(t, root)
	writeScript(t, filepath.Join(kitDir, "stall_watch.sh"), "#!/bin/sh\necho '<string>/somewhere/else</string>'\n")

	report, err := Install(root, kitDir)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !report.Failed() || !strings.Contains(report.String(), "self-check failed stall watch") {
		t.Errorf("a watch that named another forge was not reported as failed:\n%s", report)
	}
}

func TestInstallRefusesAForgeThatIsNotThere(t *testing.T) {
	root := fixtureForge(t)
	missing := filepath.Join(t.TempDir(), "not-a-forge")

	_, err := Install(missing, fixtureKit(t, root))
	if err == nil || !strings.Contains(err.Error(), "is not there") {
		t.Fatalf("Install(missing) = %v, want the forge reported as not there", err)
	}
}

func TestInstallRefusesAForgeThatIsAFile(t *testing.T) {
	root := fixtureForge(t)
	file := filepath.Join(t.TempDir(), "a-file")
	if err := os.WriteFile(file, []byte("not a forge\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Install(file, fixtureKit(t, root))
	if err == nil || !strings.Contains(err.Error(), "is not a directory") {
		t.Fatalf("Install(a file) = %v, want the forge reported as not a directory", err)
	}
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

func TestIdlerSelfCheckFailsWhenTheForgeServesNoProject(t *testing.T) {
	root := t.TempDir()
	scripts := t.TempDir()
	writeScript(t, filepath.Join(scripts, "role_health.sh"), "#!/bin/sh\nexit 0\n")

	line, ok := idlerSelfCheck(scripts, root)
	if ok || !strings.Contains(line, "serves no project to read") {
		t.Errorf("idlerSelfCheck = (%q, %v), want a failure naming the empty forge", line, ok)
	}
}

func TestInstallFileReportsAMissingKitFile(t *testing.T) {
	_, err := installFile("route gate", filepath.Join(t.TempDir(), "absent.sh"), filepath.Join(t.TempDir(), "route_card.sh"))
	if err == nil || !strings.Contains(err.Error(), "the kit is missing") {
		t.Fatalf("installFile(missing source) = %v, want the missing kit file reported", err)
	}
}
