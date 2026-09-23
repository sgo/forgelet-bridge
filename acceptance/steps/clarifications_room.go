package steps

import (
	"context"
	"fmt"
	"strings"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/internal/config"
)

// clarificationsRoom is the clarifications room of the configured forge.
func (w *World) clarificationsRoom(ctx context.Context) (string, error) {
	return w.roomNamed(ctx, config.ClarificationsRoomName)
}

// clarificationMessageStart is the opening line of the bridge's clarification
// message.
func clarificationMessageStart(project string) string {
	return fmt.Sprintf("Clarification for %s from ", project)
}

// clarificationMessage finds the clarification message the operator has for a
// project.
func (w *World) clarificationMessage(ctx context.Context, project string) (roomID, messageID, body string, err error) {
	roomID, err = w.clarificationsRoom(ctx)
	if err != nil {
		return "", "", "", err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return "", "", "", err
	}
	start := clarificationMessageStart(project)
	err = waitFor(ctx, fmt.Sprintf("the operator never saw the clarification message for %s", project), func() (bool, error) {
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

// clarificationMessageNames checks what the message tells the operator about
// the clarification: the project, the blocked role and the question.
func clarificationMessageNames(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	_, _, body, err := w.clarificationMessage(ctx, captures[1])
	if err != nil {
		return err
	}
	for _, want := range []string{captures[1], captures[2], captures[3]} {
		if !strings.Contains(body, want) {
			return fmt.Errorf("the clarification message for %s does not name %q:\n%s", captures[1], want, body)
		}
	}
	return nil
}

// clarificationsRoomCount checks how many clarification messages the room
// holds.
func clarificationsRoomCount(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	want := captures[1]
	return waitFor(ctx, fmt.Sprintf("the clarifications room never held %s clarification messages", want), func() (bool, error) {
		return fmt.Sprintf("%d", w.clarificationMessages(ctx)) == want, nil
	})
}

// clarificationsRoomEncrypted checks the clarifications room is encrypted.
func clarificationsRoomEncrypted(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomID, err := w.clarificationsRoom(ctx)
	if err != nil {
		return err
	}
	return w.encryptedRoom(ctx, roomID, "the clarifications room "+captures[1])
}

// clarificationMessages counts the clarification messages the operator has in
// the clarifications room.
func (w *World) clarificationMessages(ctx context.Context) int {
	roomID, err := w.clarificationsRoom(ctx)
	if err != nil {
		return 0
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return 0
	}
	found := 0
	for _, message := range operator.Messages(roomID) {
		if message.Sender == w.bridgeUserID && strings.HasPrefix(message.Body, "Clarification for ") {
			found++
		}
	}
	return found
}

// clarificationReplyDecrypted waits for the bridge's reply in the
// clarification's thread.
func clarificationReplyDecrypted(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	body, project := captures[1], captures[2]
	roomID, messageID, _, err := w.clarificationMessage(ctx, project)
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
		return fmt.Errorf("the clarification reply %q was not decrypted from an encrypted event", body)
	}
	return nil
}

// clarificationThreadOneReply checks the clarification's thread only carries
// the bridge's one answer.
func clarificationThreadOneReply(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomID, err := w.clarificationsRoom(ctx)
	if err != nil {
		return err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	messageID, err := oneClarificationMessage(ctx, operator, roomID)
	if err != nil {
		return err
	}
	if err := waitFor(ctx, "the clarification's thread never got the bridge's reply", func() (bool, error) {
		return threadRepliesBy(operator, roomID, messageID, w.bridgeUserID) >= 1, nil
	}); err != nil {
		return err
	}
	if err := fixtures.Sleep(ctx, settle); err != nil {
		return err
	}
	if found := threadRepliesBy(operator, roomID, messageID, w.bridgeUserID); found != 1 {
		return fmt.Errorf("the clarification's thread holds %d replies, want exactly one", found)
	}
	return nil
}

// oneClarificationMessage waits for any clarification message the operator has
// seen.
func oneClarificationMessage(ctx context.Context, operator *fixtures.User, roomID string) (string, error) {
	var messageID string
	err := waitFor(ctx, "the operator never saw a clarification message", func() (bool, error) {
		for _, message := range operator.Messages(roomID) {
			if strings.HasPrefix(message.Body, "Clarification for ") {
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

// clarificationSaysWhatAReplyMeans checks the clarification message says a
// reply is the answer.
func clarificationSaysWhatAReplyMeans(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	_, _, body, err := w.clarificationMessage(ctx, captures[1])
	if err != nil {
		return err
	}
	return saysReplyIsTheAnswer(body)
}

// saysReplyIsTheAnswer checks a message tells the operator that a reply is the
// answer.
func saysReplyIsTheAnswer(body string) error {
	text := strings.ToLower(body)
	if !strings.Contains(text, "reply") || !strings.Contains(text, "answer") {
		return fmt.Errorf("the clarification message does not tell the operator a reply is the answer:\n%s", body)
	}
	return nil
}
