package steps

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
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

// approvalCarriesBridgeMark waits for the bridge's own reaction on the
// approval's message: the mark that says the forge confirmed it, sitting on
// the line the operator marked rather than under it.
func approvalCarriesBridgeMark(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	card, mark := captures[1], captures[2]

	roomID, messageID, _, err := w.approvalMessage(ctx, card)
	if err != nil {
		return err
	}
	return waitForBridgeMark(ctx, w, roomID, messageID, mark, "the approval for "+card)
}

// forgeApprovalCarriesBridgeMark is the same for an approval in one named
// forge, which is how a many-forge scenario tells the two apart.
func forgeApprovalCarriesBridgeMark(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	card, forgeName, mark := captures[1], captures[2], captures[3]

	roomID, messageID, _, err := w.forgeApprovalMessage(ctx, forgeName, card)
	if err != nil {
		return err
	}
	return waitForBridgeMark(ctx, w, roomID, messageID, mark, fmt.Sprintf("the approval for %s in %s", card, forgeName))
}

// waitForBridgeMark waits until the operator sees the bridge's own reaction
// with the given key on a message.
func waitForBridgeMark(ctx context.Context, w *World, roomID, messageID, mark, subject string) error {
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	return waitFor(ctx, fmt.Sprintf("the bridge never marked %s with %s", subject, mark), func() (bool, error) {
		return slices.Contains(bridgeReactions(operator, roomID, messageID, w.bridgeUserID), mark), nil
	})
}

// approvalCarriesNoBridgeReaction checks the bridge left the approval's own
// message unmarked: a desktop resolution carries its threaded reply instead.
func approvalCarriesNoBridgeReaction(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()

	roomID, messageID, _, err := w.approvalMessage(ctx, captures[1])
	if err != nil {
		return err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	if err := fixtures.Sleep(ctx, settle); err != nil {
		return err
	}
	if marks := bridgeReactions(operator, roomID, messageID, w.bridgeUserID); len(marks) != 0 {
		return fmt.Errorf("the approval carries the bridge's reactions %v, want none", marks)
	}
	return nil
}

// approvalThreadHoldsNoBridgeReply checks the approval's thread stays quiet
// when the bridge marks the message instead of answering in the thread.
func approvalThreadHoldsNoBridgeReply(_ context.Context, world any, _ []string) error {
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
	if err := fixtures.Sleep(ctx, settle); err != nil {
		return err
	}
	if found := threadRepliesBy(operator, roomID, messageID, w.bridgeUserID); found != 0 {
		return fmt.Errorf("the approval's thread holds %d replies from the bridge, want none", found)
	}
	return nil
}

// bridgeReactions are the reaction keys the bridge put on a message.
func bridgeReactions(operator *fixtures.User, roomID, messageID, sender string) []string {
	var keys []string
	for _, reaction := range operator.Reactions(roomID, messageID) {
		if reaction.Sender == sender {
			keys = append(keys, reaction.Key)
		}
	}
	return keys
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
