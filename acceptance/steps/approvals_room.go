package steps

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/internal/config"
)

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

// roomNamed is the named room inside the configured forge's space.
func (w *World) roomNamed(ctx context.Context, name string) (string, error) {
	space, err := w.forgeSpace()
	if err != nil {
		return "", err
	}
	return w.waitForSpaceChild(ctx, space, name)
}

// approvalsRoom is the approvals room of the configured forge.
func (w *World) approvalsRoom(ctx context.Context) (string, error) {
	return w.roomNamed(ctx, config.ApprovalsRoomName)
}

// approvalMessageStart is the opening line of the bridge's approval message.
func approvalMessageStart(card, project string) string {
	return fmt.Sprintf("Approval for %s in %s", card, project)
}

// approvalMessage finds the approval message the operator has for a card, in
// the configured forge. The suite's single-forge scenarios name the forge
// through which the bridge serves it, and the many-forge ones name it outright.
func (w *World) approvalMessage(ctx context.Context, card string) (roomID, messageID, body string, err error) {
	forgeName, err := w.forgeSpace()
	if err != nil {
		return "", "", "", err
	}
	return w.forgeApprovalMessage(ctx, forgeName, card)
}

// forgeApprovalMessage finds the approval message one named forge's room holds
// for a card.
func (w *World) forgeApprovalMessage(ctx context.Context, forgeName, card string) (roomID, messageID, body string, err error) {
	roomID, err = w.waitForSpaceChild(ctx, forgeName, config.ApprovalsRoomName)
	if err != nil {
		return "", "", "", err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return "", "", "", err
	}
	start := approvalMessageStart(card, approvalProject)
	err = waitFor(ctx, fmt.Sprintf("the operator never saw the approval message for %s in %s", card, forgeName), func() (bool, error) {
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
	messageID, err := w.oneApprovalMessage(ctx, operator, roomID)
	if err != nil {
		return err
	}
	if err := waitFor(ctx, "the approval's thread never got the bridge's reply", func() (bool, error) {
		return threadRepliesBy(operator, roomID, messageID, w.bridgeUserID) >= 1, nil
	}); err != nil {
		return err
	}
	if err := fixtures.Sleep(ctx, settle); err != nil {
		return err
	}
	if found := threadRepliesBy(operator, roomID, messageID, w.bridgeUserID); found != 1 {
		return fmt.Errorf("the approval's thread holds %d replies, want exactly one", found)
	}
	return nil
}

// oneApprovalMessage waits for any approval message the operator has seen.
func (w *World) oneApprovalMessage(ctx context.Context, operator *fixtures.User, roomID string) (string, error) {
	var messageID string
	err := waitFor(ctx, "the operator never saw an approval message", func() (bool, error) {
		for _, message := range operator.Messages(roomID) {
			if strings.HasPrefix(message.Body, "Approval for ") {
				messageID = message.EventID
				return true, nil
			}
		}
		return false, nil
	})
	return messageID, err
}

// threadRepliesBy counts the messages one sender has in a thread.
func threadRepliesBy(operator *fixtures.User, roomID, messageID, sender string) int {
	found := 0
	for _, message := range operator.Messages(roomID) {
		if message.ThreadRoot == messageID && message.Sender == sender {
			found++
		}
	}
	return found
}

// spaceHoldsNamedRoom checks the forge space holds the named room.
func spaceHoldsNamedRoom(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	_, err := w.waitForSpaceChild(ctx, captures[1], captures[2])
	return err
}

// invitedToNamedRoom checks the bridge invited the operator to the named room.
func invitedToNamedRoom(_ context.Context, world any, captures []string) error {
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
	return w.encryptedRoom(ctx, roomID, "the approvals room "+captures[1])
}

// approvalSaysWhatAReplyMeans checks the approval message says a reply sends
// the approval back, so the two rooms cannot be told apart only in the code.
func approvalSaysWhatAReplyMeans(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	_, _, body, err := w.approvalMessage(ctx, captures[1])
	if err != nil {
		return err
	}
	text := strings.ToLower(body)
	if !strings.Contains(text, "reply") || !strings.Contains(text, "send it back") {
		return fmt.Errorf("the approval message does not tell the operator a reply sends it back:\n%s", body)
	}
	return nil
}
