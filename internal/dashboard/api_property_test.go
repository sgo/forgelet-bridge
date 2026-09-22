//go:build property

package dashboard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/quick"
)

// TestPropertyGateNamesTheHandover is what the operator reads on their phone:
// the gate shows who handed the work to whom whenever the forge reports those
// roles, and the forge's own wording when it does not.
func TestPropertyGateNamesTheHandover(t *testing.T) {
	property := func(from, to, gate string) bool {
		request := ApprovalRequest{From: from, To: to, Gate: gate}

		if strings.TrimSpace(from) != "" && strings.TrimSpace(to) != "" {
			return request.gate() == strings.TrimSpace(from)+" → "+strings.TrimSpace(to)
		}
		return request.gate() == gate
	}
	if err := quick.Check(property, nil); err != nil {
		t.Error(err)
	}
}

// TestPropertyURLGivesAUsableAddress checks the address the bridge builds its
// paths on: whatever the configuration or the announcement holds, the address
// either names something or explains why not, and never ends in a separator.
func TestPropertyURLGivesAUsableAddress(t *testing.T) {
	dir := t.TempDir()
	property := func(configured string) bool {
		address, err := URL(dir, configured)
		if err != nil {
			return dashboardAddress(configured) == ""
		}
		return address != "" && !strings.HasSuffix(address, "/")
	}
	if err := quick.Check(property, nil); err != nil {
		t.Error(err)
	}
}

// TestPropertyURLFallsBackToTheAnnouncement checks the same for a forge that
// announces itself: an address the configuration does not give, or gives as
// nothing usable, reads as the announced one.
func TestPropertyURLFallsBackToTheAnnouncement(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".swarmforge"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".swarmforge", "dashboard-url"), []byte("http://127.0.0.1:1234/"), 0o644); err != nil {
		t.Fatal(err)
	}

	property := func(configured string) bool {
		if dashboardAddress(configured) != "" {
			_, err := URL(root, configured)
			return err == nil
		}
		address, err := URL(root, configured)
		return err == nil && address == "http://127.0.0.1:1234"
	}
	if err := quick.Check(property, nil); err != nil {
		t.Error(err)
	}
}
