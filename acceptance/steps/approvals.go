package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/internal/approvals"
	"github.com/unclebob/forgelet-bridge/internal/config"
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
	if !strings.Contains(roles, "without") {
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

// projectPaths are the places a forge project keeps its approvals.
type projectPaths struct {
	projectRoot string
	pending     string
	outbox      string
	reviews     string
	notify      string
}

func pathsFor(root string) projectPaths {
	return projectPaths{
		projectRoot: filepath.Join(root, "projects", approvalProject),
		pending:     filepath.Join(root, "projects", approvalProject, ".swarmforge", "handoffs", "pending_approval"),
		outbox:      filepath.Join(root, "projects", approvalProject, ".swarmforge", "handoffs", "outbox"),
		reviews:     filepath.Join(root, "projects", approvalProject, ".swarmforge", "rejected-tasks"),
		notify:      filepath.Join(root, "projects", approvalProject, ".swarmforge", "notify"),
	}
}

// forgeSpace is the name of the configured forge's space.
func (w *World) forgeSpace() (string, error) {
	root, err := singleForge(w)
	if err != nil {
		return "", err
	}
	if name := w.forgeNames[root]; name != "" {
		return name, nil
	}
	return filepath.Base(root), nil
}

// approvalsRoom is the approvals room of the configured forge.
func (w *World) approvalsRoom(ctx context.Context) (string, error) {
	space, err := w.forgeSpace()
	if err != nil {
		return "", err
	}
	return w.waitForSpaceChild(ctx, space, config.ApprovalsRoomName)
}

// approvalMessageStart is the opening line of the bridge's approval message.
func approvalMessageStart(card string) string {
	return fmt.Sprintf("Approval for %s in %s", card, approvalProject)
}

// approvalMessage finds the approval message the operator has for a card.
func (w *World) approvalMessage(ctx context.Context, card string) (roomID, messageID, body string, err error) {
	roomID, err = w.approvalsRoom(ctx)
	if err != nil {
		return "", "", "", err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return "", "", "", err
	}
	start := approvalMessageStart(card)
	err = waitFor(ctx, fmt.Sprintf("the operator never saw the approval message for %s", card), func() (bool, error) {
		for _, message := range operator.Messages(roomID) {
			if strings.HasPrefix(message.Body, start) {
				messageID, body = message.EventID, message.Body
				return true, nil
			}
		}
		return false, nil
	})
	return roomID, messageID, body, err
}

// approvalMessageNames checks what the message tells the operator about the
// approval: the project, the gate, and what changed.
func approvalMessageNames(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	card, project, gate := captures[1], captures[2], captures[3]

	_, _, body, err := w.approvalMessage(ctx, card)
	if err != nil {
		return err
	}
	for _, want := range []string{project, gate, captures[4], captures[5]} {
		if !strings.Contains(body, want) {
			return fmt.Errorf("the approval message for %s does not name %q:\n%s", card, want, body)
		}
	}
	return nil
}

// approvalTapped sends the operator's approval reaction.
func approvalTapped(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	return reactToApproval(ctx, w, w.operatorID, "✅", captures[1])
}

// someoneReacts sends a reaction from any fixture client, so the allowlist can
// be checked.
func someoneReacts(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	reactor, reaction, card := captures[1], captures[2], captures[3]
	userID := w.operatorID
	if reactor != "the operator" {
		userID = reactor
	}
	return reactToApproval(ctx, w, userID, reaction, card)
}

func reactToApproval(ctx context.Context, w *World, userID, reaction, card string) error {
	roomID, messageID, _, err := w.approvalMessage(ctx, card)
	if err != nil {
		return err
	}
	user, err := w.user(ctx, userID)
	if err != nil {
		return err
	}
	return user.React(ctx, roomID, messageID, reaction)
}

// operatorRepliesToApproval sends the operator's feedback in the approval's
// thread.
func operatorRepliesToApproval(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	feedback, card := captures[1], captures[2]

	roomID, messageID, _, err := w.approvalMessage(ctx, card)
	if err != nil {
		return err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	_, err = operator.SendInThread(ctx, roomID, feedback, messageID)
	return err
}

// operatorTalksInApprovalsRoom sends a plain message into the approvals room,
// which must decide nothing.
func operatorTalksInApprovalsRoom(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomID, err := w.approvalsRoom(ctx)
	if err != nil {
		return err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	_, err = operator.Send(ctx, roomID, captures[1])
	return err
}

// spaceHoldsApprovalsRoom checks the forge space holds the approvals room.
func spaceHoldsApprovalsRoom(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	_, err := w.waitForSpaceChild(ctx, captures[1], captures[2])
	return err
}

// invitedToApprovalsRoom checks the bridge invited the operator to the
// approvals room.
func invitedToApprovalsRoom(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	return invitedTo(ctx, w, captures[1], false)
}

// approvalsRoomEncrypted checks the approvals room is encrypted.
func approvalsRoomEncrypted(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomID, err := w.approvalsRoom(ctx)
	if err != nil {
		return err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	algorithm, err := operator.EncryptionAlgorithm(ctx, roomID)
	if err != nil {
		return err
	}
	if algorithm != "m.megolm.v1.aes-sha2" {
		return fmt.Errorf("the approvals room %s is not encrypted: %q", captures[1], algorithm)
	}
	return nil
}

// userJoinedApprovalsRoom brings a fixture client into the approvals room, the
// way the operator would add someone.
func userJoinedApprovalsRoom(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	userID := captures[1]

	roomID, err := w.approvalsRoom(ctx)
	if err != nil {
		return err
	}
	user, err := w.user(ctx, userID)
	if err != nil {
		return err
	}
	if rooms, err := user.Rooms(ctx); err == nil && fixtures.Contains(rooms, roomID) {
		return nil
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	if err := operator.Invite(ctx, roomID, userID); err != nil {
		return err
	}
	return waitFor(ctx, fmt.Sprintf("%s never joined the approvals room", userID), func() (bool, error) {
		rooms, err := user.JoinedRoomIDs(ctx)
		if err != nil {
			return false, err
		}
		for _, room := range rooms {
			if room == roomID {
				return true, nil
			}
		}
		return false, nil
	})
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

// forgeNeverDestroyed checks nothing asked the forge to delete work or tear a
// project down: that stays on the desktop.
func forgeNeverDestroyed(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	root, err := singleForge(w)
	if err != nil {
		return err
	}
	for _, dir := range []string{pathsFor(root).notify, filepath.Join(root, ".swarmforge", "notify")} {
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		for _, entry := range entries {
			name := strings.ToLower(entry.Name())
			for _, word := range []string{"delete", "teardown", "destroy", "archive"} {
				if strings.Contains(name, word) {
					return fmt.Errorf("the forge was asked to destroy work: %s", filepath.Join(dir, entry.Name()))
				}
			}
		}
	}
	return nil
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

// approvalReplyDecrypted waits for the bridge's reply in the approval's thread.
func approvalReplyDecrypted(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	body, card := captures[1], captures[2]

	roomID, messageID, _, err := w.approvalMessage(ctx, card)
	if err != nil {
		return err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	reply, err := operator.WaitForThreadReply(ctx, roomID, messageID, body, stepTimeout)
	if err != nil {
		return err
	}
	if !reply.Encrypted {
		return fmt.Errorf("the approval reply %q was not decrypted from an encrypted event", body)
	}
	return nil
}

// approvalThreadOneReply checks the approval's thread only carries the bridge's
// one answer.
func approvalThreadOneReply(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()

	roomID, err := w.approvalsRoom(ctx)
	if err != nil {
		return err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	var messageID string
	if err := waitFor(ctx, "the operator never saw an approval message", func() (bool, error) {
		for _, message := range operator.Messages(roomID) {
			if strings.HasPrefix(message.Body, "Approval for ") {
				messageID = message.EventID
				return true, nil
			}
		}
		return false, nil
	}); err != nil {
		return err
	}
	count := func() int {
		found := 0
		for _, message := range operator.Messages(roomID) {
			if message.ThreadRoot == messageID && message.Sender == w.bridgeUserID {
				found++
			}
		}
		return found
	}
	if err := waitFor(ctx, "the approval's thread never got the bridge's reply", func() (bool, error) { return count() >= 1, nil }); err != nil {
		return err
	}
	if err := fixtures.Sleep(ctx, settle); err != nil {
		return err
	}
	if found := count(); found != 1 {
		return fmt.Errorf("the approval's thread holds %d replies, want exactly one", found)
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
