package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/unclebob/forgelet-bridge/internal/approvals"
)

// approvalProject is the project the acceptance approvals belong to, the way
// the forge reports it: the project's room in the forge.
const approvalProject = "forgelet-bridge"

// approvalCardID is the id the fixture's approval handoffs carry.
func approvalCardID(card string) string {
	return "approval-" + card
}

// approvalTaskID is the card's task id in the fixture's handoffs.
func approvalTaskID(card string) string {
	return "20260922T124152671989Z-" + card
}

// projectPaths are the places a forge project keeps its approvals.
type projectPaths struct {
	pending string
	outbox  string
	reviews string
	notify  string
}

func pathsFor(root string) projectPaths {
	return projectPaths{
		pending: filepath.Join(root, "projects", approvalProject, ".swarmforge", "handoffs", "pending_approval"),
		outbox:  filepath.Join(root, "projects", approvalProject, ".swarmforge", "handoffs", "outbox"),
		reviews: filepath.Join(root, "projects", approvalProject, ".swarmforge", "rejected-tasks"),
		notify:  filepath.Join(root, "projects", approvalProject, ".swarmforge", "notify"),
	}
}

// pendingApproval seeds a handoff waiting for the operator's approval, the way
// the forge's dashboard holds it: a pending handoff in the project's forge
// directory, with or without the roles that handed the work over.
func pendingApproval(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	card, roles := captures[1], captures[2]
	root, err := singleForge(w)
	if err != nil {
		return err
	}
	withRoles, err := handoverRoles(roles)
	if err != nil {
		return err
	}

	headers := []string{
		"id: " + approvalCardID(card),
		"from: coder",
		"to: refactorer",
		"recipient: refactorer",
		"priority: 50",
		"type: git_handoff",
		"task_id: " + approvalTaskID(card),
		"task: " + card,
		"commit: 301a407dc0",
		"artifacts: internal/bridge/bridge.go, internal/relay/relay.go",
	}
	if withRoles {
		headers = append(headers, "role: coder")
	}
	content := strings.Join(headers, "\n") + "\n\nRe-read your role and constitution.\n"
	if err := os.MkdirAll(pathsFor(root).pending, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(pathsFor(root).pending, approvalCardID(card)+".handoff"), []byte(content), 0o644); err != nil {
		return err
	}
	return markProjectOpen(root)
}

// handoverRoles reads the outline column that says whether an approval handoff
// carries the roles that handed the work over. The step has to interpret that
// wording, so a wording it does not know is an error rather than one of the two
// cases by accident.
func handoverRoles(wording string) (bool, error) {
	switch strings.TrimSpace(wording) {
	case "with its handover roles":
		return true, nil
	case "without its handover roles":
		return false, nil
	default:
		return false, fmt.Errorf("the approval step does not know the wording %q, which says whether the handoff carries its handover roles", wording)
	}
}

// markProjectOpen tells the forge the project is open, so its approvals are
// ones the operator has to see.
func markProjectOpen(root string) error {
	path := filepath.Join(root, ".swarmforge", "open-projects")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, line := range strings.Split(string(existing), "\n") {
		if strings.TrimSpace(line) == approvalProject {
			return nil
		}
	}
	return os.WriteFile(path, append(existing, []byte(approvalProject+"\n")...), 0o644)
}

// approvalApproved checks the forge recorded the approval as approved.
func approvalApproved(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := singleForge(w)
	if err != nil {
		return err
	}
	ctx, cancel := stepContext()
	defer cancel()
	approved := filepath.Join(pathsFor(root).outbox, approvalCardID(captures[1])+".handoff")
	return waitFor(ctx, fmt.Sprintf("the forge never recorded %s as approved", captures[1]), func() (bool, error) {
		data, err := os.ReadFile(approved)
		if err != nil {
			return false, nil
		}
		return strings.Contains(string(data), "approved: true"), nil
	})
}

// approvalSentBack checks the forge recorded the operator's feedback.
func approvalSentBack(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := singleForge(w)
	if err != nil {
		return err
	}
	ctx, cancel := stepContext()
	defer cancel()
	card, feedback := captures[1], captures[2]
	history := filepath.Join(pathsFor(root).reviews, approvalTaskID(card), "reviews.json")
	return waitFor(ctx, fmt.Sprintf("the forge never recorded %s as sent back", card), func() (bool, error) {
		data, err := os.ReadFile(history)
		if err != nil {
			return false, nil
		}
		return strings.Contains(string(data), feedback), nil
	})
}

// approvalStillPending checks the forge is still waiting for the decision.
func approvalStillPending(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	root, err := singleForge(w)
	if err != nil {
		return err
	}
	// Wait for the bridge to have handled everything before the operator's
	// events, so a decision it should not have made shows up here.
	if err := w.caughtUp(ctx); err != nil {
		return err
	}
	pending := filepath.Join(pathsFor(root).pending, approvalCardID(captures[1])+".handoff")
	if _, err := os.Stat(pending); err != nil {
		return fmt.Errorf("the approval for %s is no longer pending: %v", captures[1], err)
	}
	return nil
}

// resolutionCount checks the forge recorded exactly one resolution: the card is
// either still waiting or resolved, never both.
func resolutionCount(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := singleForge(w)
	if err != nil {
		return err
	}
	paths := pathsFor(root)
	card := approvalCardID(captures[1])
	count := 0
	if _, err := os.Stat(filepath.Join(paths.outbox, card+".handoff")); err == nil {
		count++
	}
	if _, err := os.Stat(filepath.Join(paths.pending, card+".handoff")); err == nil {
		count++
	}
	if count != 1 {
		return fmt.Errorf("the forge recorded %d resolutions for %s, want exactly one", count, captures[1])
	}
	return nil
}

// forgeNeverDestroyed checks nothing asked the forge to delete work or tear a
// project down: that stays on the desktop.
func forgeNeverDestroyed(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	root, err := singleForge(w)
	if err != nil {
		return err
	}
	for _, dir := range []string{pathsFor(root).notify, filepath.Join(root, ".swarmforge", "notify")} {
		found, err := destroyingRequests(dir)
		if err != nil {
			return err
		}
		if found != "" {
			return fmt.Errorf("the forge was asked to destroy work: %s", found)
		}
	}
	return nil
}

// destroyingRequests is the first request in a notify directory that would
// delete work or tear a project down, empty when there is none.
func destroyingRequests(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		name := strings.ToLower(entry.Name())
		for _, word := range []string{"delete", "teardown", "destroy", "archive"} {
			if strings.Contains(name, word) {
				return filepath.Join(dir, entry.Name()), nil
			}
		}
	}
	return "", nil
}

// desktopApproved resolves the approval the way the desktop dashboard does.
func desktopApproved(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := singleForge(w)
	if err != nil {
		return err
	}
	return approvals.New(root).Approve(approvalProject, approvalCardID(captures[1]))
}
