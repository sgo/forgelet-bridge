package approvals

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const pending = `id: 20260922T124152671989Z-phone-approvals
from: coder
to: refactorer
recipient: refactorer
priority: 50
type: git_handoff
role: coder
task_id: 20260922T124152671989Z-phone-approvals
task: phone-approvals
commit: abc123
artifacts: internal/bridge/bridge.go, internal/relay/relay.go

Re-read your role and constitution.
`

func newStore(t *testing.T, projects ...string) *Store {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".swarmforge"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := ""
	for _, project := range projects {
		body += project + "\n"
		if err := os.MkdirAll(filepath.Join(root, "projects", project, ".swarmforge", "handoffs", "pending_approval"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".swarmforge", "open-projects"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return New(root)
}

func seed(t *testing.T, store *Store, project, id, content string) string {
	t.Helper()
	path := filepath.Join(store.root, "projects", project, ".swarmforge", "handoffs", "pending_approval", id+".handoff")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPendingReadsTheProjectsApprovals(t *testing.T) {
	store := newStore(t, "forgelet-bridge")
	seed(t, store, "forgelet-bridge", "20260922T124152671989Z-phone-approvals", pending)

	pending, err := store.Pending()
	if err != nil {
		t.Fatalf("Pending: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("pending = %d, want one", len(pending))
	}
	approval := pending[0]
	if approval.Project != "forgelet-bridge" {
		t.Errorf("project = %q", approval.Project)
	}
	if approval.ID != "20260922T124152671989Z-phone-approvals" {
		t.Errorf("id = %q", approval.ID)
	}
	if approval.Card != "phone-approvals" {
		t.Errorf("card = %q", approval.Card)
	}
	if approval.Gate != "coder → refactorer" {
		t.Errorf("gate = %q, want the handover roles", approval.Gate)
	}
	if len(approval.Artifacts) != 2 || approval.Artifacts[0] != "internal/bridge/bridge.go" {
		t.Errorf("artifacts = %v", approval.Artifacts)
	}
}

func TestPendingFallsBackToTheSpecRoleWhenNoRolesAreReported(t *testing.T) {
	store := newStore(t, "forgelet-bridge")
	seed(t, store, "forgelet-bridge", "approval-1", `to: refactorer
task: phone-approvals
artifacts: internal/bridge/bridge.go

body
`)

	pending, err := store.Pending()
	if err != nil {
		t.Fatalf("Pending: %v", err)
	}
	if pending[0].Gate != "spec → refactorer" {
		t.Errorf("gate = %q, want the reported gate", pending[0].Gate)
	}
}

func TestPendingIgnoresProjectsThatAreNotOpen(t *testing.T) {
	store := newStore(t, "forgelet-bridge")
	seed(t, store, "closed-project", "approval-1", pending)

	pending, err := store.Pending()
	if err != nil {
		t.Fatalf("Pending: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("pending = %+v, want nothing from a closed project", pending)
	}
}

func TestPendingForIgnoresWhatIsNotAHandoff(t *testing.T) {
	store := newStore(t, "forgelet-bridge")
	seed(t, store, "forgelet-bridge", "approval-1", pending)
	dir := filepath.Join(store.root, "projects", "forgelet-bridge", ".swarmforge", "handoffs", "pending_approval")
	if err := os.Mkdir(filepath.Join(dir, "a-directory.handoff"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not a handoff"), 0o644); err != nil {
		t.Fatal(err)
	}

	pendingApprovals, err := store.PendingFor("forgelet-bridge")
	if err != nil {
		t.Fatalf("PendingFor: %v", err)
	}
	if len(pendingApprovals) != 1 || pendingApprovals[0].ID != "approval-1" {
		t.Errorf("pending = %+v, want only the handoff", pendingApprovals)
	}
}

func TestPendingForAProjectWithoutApprovals(t *testing.T) {
	store := newStore(t)

	pendingApprovals, err := store.PendingFor("forgelet-bridge")
	if err != nil {
		t.Fatalf("PendingFor: %v", err)
	}
	if len(pendingApprovals) != 0 {
		t.Errorf("pending = %+v, want none", pendingApprovals)
	}
}

func TestApproveMovesTheHandoffToTheOutboxApproved(t *testing.T) {
	store := newStore(t, "forgelet-bridge")
	source := seed(t, store, "forgelet-bridge", "approval-1", pending)
	reviews := filepath.Join(filepath.Dir(source), "approval-1.reviews.json")
	if err := os.WriteFile(reviews, []byte(`{"/x":"note"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := store.Approve("forgelet-bridge", "approval-1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Errorf("the pending handoff is still there: %v", err)
	}
	if _, err := os.Stat(reviews); !os.IsNotExist(err) {
		t.Errorf("the review notes are still there: %v", err)
	}

	approved := filepath.Join(store.root, "projects", "forgelet-bridge", ".swarmforge", "handoffs", "outbox", "approval-1.handoff")
	data, err := os.ReadFile(approved)
	if err != nil {
		t.Fatalf("read approved handoff: %v", err)
	}
	if !strings.Contains(string(data), "approved: true") {
		t.Errorf("approved handoff =\n%s\nwant an approved header", data)
	}
	if !strings.Contains(string(data), "task: phone-approvals") {
		t.Errorf("approved handoff lost its headers:\n%s", data)
	}

	remaining, err := store.Pending()
	if err != nil {
		t.Fatalf("Pending: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("pending = %+v, want none after approving", remaining)
	}
}

func TestApproveRejectsAnUnknownApproval(t *testing.T) {
	store := newStore(t, "forgelet-bridge")
	if err := store.Approve("forgelet-bridge", "approval-1"); err == nil {
		t.Fatal("Approve accepted an unknown approval")
	}
}

func TestSendBackRecordsTheFeedbackAndStopsTheApprovalBeingPending(t *testing.T) {
	store := newStore(t, "forgelet-bridge")
	seed(t, store, "forgelet-bridge", "approval-1", pending)
	at := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return at }

	if err := store.SendBack("forgelet-bridge", "approval-1", "the timesheet total is still wrong"); err != nil {
		t.Fatalf("SendBack: %v", err)
	}

	historyPath := filepath.Join(store.root, "projects", "forgelet-bridge", ".swarmforge", "rejected-tasks",
		"20260922T124152671989Z-phone-approvals", "reviews.json")
	data, err := os.ReadFile(historyPath)
	if err != nil {
		t.Fatalf("read the card's review history: %v", err)
	}
	var history map[string][]review
	if err := json.Unmarshal(data, &history); err != nil {
		t.Fatalf("parse review history: %v", err)
	}
	for _, artifact := range []string{"internal/bridge/bridge.go", "internal/relay/relay.go"} {
		entries := history[artifact]
		if len(entries) != 1 || entries[0].Text != "the timesheet total is still wrong" || entries[0].At != "2026-09-22T12:00:00Z" {
			t.Errorf("review history for %s = %+v", artifact, entries)
		}
	}

	remaining, err := store.Pending()
	if err != nil {
		t.Fatalf("Pending: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("pending = %+v, want none after sending the approval back", remaining)
	}
}

func TestSendBackAppendsToExistingHistory(t *testing.T) {
	store := newStore(t, "forgelet-bridge")
	seed(t, store, "forgelet-bridge", "approval-1", pending)
	historyPath := filepath.Join(store.root, "projects", "forgelet-bridge", ".swarmforge", "rejected-tasks",
		"20260922T124152671989Z-phone-approvals", "reviews.json")
	if err := os.MkdirAll(filepath.Dir(historyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(historyPath, []byte(`{"internal/bridge/bridge.go":[{"at":"2026-01-01T00:00:00Z","text":"older note"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := store.SendBack("forgelet-bridge", "approval-1", "new note"); err != nil {
		t.Fatalf("SendBack: %v", err)
	}

	data, err := os.ReadFile(historyPath)
	if err != nil {
		t.Fatal(err)
	}
	var history map[string][]review
	if err := json.Unmarshal(data, &history); err != nil {
		t.Fatal(err)
	}
	entries := history["internal/bridge/bridge.go"]
	if len(entries) != 2 || entries[0].Text != "older note" || entries[1].Text != "new note" {
		t.Errorf("history = %+v, want the new note appended", entries)
	}
}

func TestApprovedHeaderIsNotAddedTwice(t *testing.T) {
	content := "id: x\napproved: true\n\nbody\n"
	if got := approved(content); got != content {
		t.Errorf("approved() = %q, want it left alone", got)
	}
}
