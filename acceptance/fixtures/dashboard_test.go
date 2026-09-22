package fixtures

import (
	"os/exec"
	"strings"
	"testing"
)

func TestFixtureRootGetsItsOwnRepository(t *testing.T) {
	root := t.TempDir()

	if err := initFixtureRepo(root); err != nil {
		t.Fatalf("initFixtureRepo: %v", err)
	}

	if head := FixtureHead(root); head == "" {
		t.Error("the fixture has no commit, so a fixture handoff could name none")
	}
	if status := gitOut(t, root, "status", "--porcelain"); status != "" {
		t.Errorf("the fixture repository is dirty:\n%s", status)
	}
}

func TestFixtureRepositoryIsOnlyPreparedOnce(t *testing.T) {
	root := t.TempDir()
	if err := initFixtureRepo(root); err != nil {
		t.Fatalf("initFixtureRepo: %v", err)
	}
	first := FixtureHead(root)

	if err := initFixtureRepo(root); err != nil {
		t.Fatalf("second initFixtureRepo: %v", err)
	}
	if second := FixtureHead(root); second != first {
		t.Errorf("head = %q after a second prepare, want %q", second, first)
	}
}

func TestFixtureHeadNamesNoCommitWithoutARepository(t *testing.T) {
	if head := FixtureHead(t.TempDir()); head != "" {
		t.Errorf("head = %q for a directory that is not a repository, want none", head)
	}
}

func TestFixtureSnapshotsListsTheCardsSnapshots(t *testing.T) {
	root := t.TempDir()
	if err := initFixtureRepo(root); err != nil {
		t.Fatalf("initFixtureRepo: %v", err)
	}
	for _, branch := range []string{
		"rejected/20260922T124152671989Z-phone-approvals/1",
		"rejected/20260922T130400589904Z-card-activity-feed/1",
	} {
		gitOut(t, root, "branch", branch)
	}
	dashboard := &Dashboard{Root: root}

	snapshots, err := dashboard.FixtureSnapshots("phone-approvals")
	if err != nil {
		t.Fatalf("FixtureSnapshots: %v", err)
	}
	if len(snapshots) != 1 || !strings.HasSuffix(snapshots[0], "phone-approvals/1") {
		t.Errorf("snapshots = %v, want the card's snapshot alone", snapshots)
	}
}

// gitOut runs git in a fixture and returns what it printed.
func gitOut(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}
