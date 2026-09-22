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
		return approvalThreadReplies(operator, roomID, messageID, w.bridgeUserID) >= 1, nil
	}); err != nil {
		return err
	}
	if err := fixtures.Sleep(ctx, settle); err != nil {
		return err
	}
	if found := approvalThreadReplies(operator, roomID, messageID, w.bridgeUserID); found != 1 {
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

// approvalThreadReplies counts the bridge's replies in an approval's thread.
func approvalThreadReplies(operator *fixtures.User, roomID, messageID, bridgeUserID string) int {
	found := 0
	for _, message := range operator.Messages(roomID) {
		if message.ThreadRoot == messageID && message.Sender == bridgeUserID {
			found++
		}
	}
	return found
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
	return w.encryptedRoom(ctx, roomID, "the approvals room "+captures[1])
}
