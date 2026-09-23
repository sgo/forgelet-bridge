package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/internal/config"
	"github.com/unclebob/forgelet-bridge/internal/dashboard"
)

// clarificationsRoom is the clarifications room of the configured forge.
func (w *World) clarificationsRoom(ctx context.Context) (string, error) {
	space, err := w.forgeSpace()
	if err != nil {
		return "", err
	}
	return w.waitForSpaceChild(ctx, space, config.ClarificationsRoomName)
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

// clarificationProjectPaths are the places a fixture project keeps its
// clarifications, the way the forge's dashboard holds them.
type clarificationProjectPaths struct {
	pending string
	done    string
}

func clarificationPaths(root, project string) clarificationProjectPaths {
	base := filepath.Join(root, "projects", project, ".swarmforge", "dashboard", "clarifications")
	return clarificationProjectPaths{pending: filepath.Join(base, "pending"), done: filepath.Join(base, "done")}
}

// clarificationID is the id the fixture's clarifications carry.
func clarificationID(project string) string {
	return "clarification-" + project
}

// seedPendingClarification seeds a clarification the way the forge's dashboard
// holds it: a pending clarification in the project, which the dashboard lists
// for the bridge to carry into the room.
func seedPendingClarification(w *World, role, project, question string) error {
	root, err := singleForge(w)
	if err != nil {
		return err
	}
	projectRoot := filepath.Join(root, "projects", project)
	// A project is a swarm root of its own: the dashboard wakes the blocked
	// role in the role's own pane, which is inside the project.
	if err := fixtures.PrepareSwarmRoot(projectRoot); err != nil {
		return err
	}
	if err := os.MkdirAll(clarificationPaths(root, project).pending, 0o755); err != nil {
		return err
	}
	headers := []string{
		"id: " + clarificationID(project),
		"status: pending",
		"role: " + role,
		"created_at: 2026-09-23T09:00:00Z",
	}
	content := strings.Join(headers, "\n") + "\n\n" + question + "\n"
	if err := os.WriteFile(filepath.Join(clarificationPaths(root, project).pending, clarificationID(project)+".request"), []byte(content), 0o644); err != nil {
		return err
	}
	return markProjectOpen(root)
}

// pendingClarification seeds a clarification the forge's dashboard is showing.
func pendingClarification(_ context.Context, world any, captures []string) error {
	return seedPendingClarification(world.(*World), captures[1], captures[2], captures[3])
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

// operatorRepliesToClarification sends the operator's answer in the
// clarification's thread.
func operatorRepliesToClarification(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	return oneUserRepliesToClarification(ctx, w, w.operatorID, captures[2], captures[1])
}

// someoneRepliesToClarification sends a reply from any fixture client, so the
// allowlist can be checked.
func someoneRepliesToClarification(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	return oneUserRepliesToClarification(ctx, w, captures[1], captures[2], captures[3])
}

// oneUserRepliesToClarification has one Matrix user reply in the
// clarification's thread.
func oneUserRepliesToClarification(ctx context.Context, w *World, userID, answer, project string) error {
	roomID, messageID, _, err := w.clarificationMessage(ctx, project)
	if err != nil {
		return err
	}
	user, err := w.user(ctx, userID)
	if err != nil {
		return err
	}
	_, err = user.SendInThread(ctx, roomID, answer, messageID)
	return err
}

// operatorSwipesReplyToClarification sends the answer the way a phone does:
// by quoting the clarification message.
func operatorSwipesReplyToClarification(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomID, messageID, _, err := w.clarificationMessage(ctx, captures[2])
	if err != nil {
		return err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	_, err = operator.SwipeReply(ctx, roomID, messageID, captures[1])
	return err
}

// operatorTalksInClarificationsRoom sends a plain message into the
// clarifications room, which must answer nothing.
func operatorTalksInClarificationsRoom(_ context.Context, world any, captures []string) error {
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
	_, err = operator.Send(ctx, roomID, captures[1])
	return err
}

// userJoinedClarificationsRoom brings a fixture client into the clarifications
// room, the way the operator would add someone.
func userJoinedClarificationsRoom(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomID, err := w.clarificationsRoom(ctx)
	if err != nil {
		return err
	}
	return w.addUserToRoom(ctx, captures[1], roomID, "the clarifications room")
}

// clarificationAnswered checks the forge recorded the operator's answer
// exactly, without the question they quoted back.
func clarificationAnswered(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := singleForge(w)
	if err != nil {
		return err
	}
	ctx, cancel := stepContext()
	defer cancel()
	project, answer := captures[1], captures[2]
	done := filepath.Join(clarificationPaths(root, project).done, clarificationID(project)+".request")
	return waitFor(ctx, fmt.Sprintf("the forge never recorded the clarification for %s as answered with %q", project, answer), func() (bool, error) {
		data, err := os.ReadFile(done)
		if err != nil {
			return false, nil
		}
		return recordedAnswer(string(data)) == answer, nil
	})
}

// recordedAnswer is the response a done clarification carries.
func recordedAnswer(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if value, found := strings.CutPrefix(line, "response: "); found {
			return strings.ReplaceAll(value, `\n`, "\n")
		}
	}
	return ""
}

// blockedRoleWoken checks the dashboard woke the blocked role's own pane with
// the answer.
func blockedRoleWoken(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	root, err := singleForge(w)
	if err != nil {
		return err
	}
	running, ok := w.running[filepath.Base(root)]
	if !ok {
		return fmt.Errorf("the fixture forge root %s does not have its dashboard running", root)
	}
	role, answer := captures[1], captures[2]
	// The dashboard writes the wake into the pane through its tmux stub, which
	// records the arguments of the call: the answer is one line of it.
	typed := "Answer:\\n" + answer
	return waitFor(ctx, fmt.Sprintf("the blocked role %s was never woken with the answer %q", role, answer), func() (bool, error) {
		lines, err := running.Typed()
		if err != nil {
			return false, err
		}
		for _, line := range lines {
			if strings.Contains(line, "Clarification requested from: "+role) && strings.Contains(line, typed) {
				return true, nil
			}
		}
		return false, nil
	})
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

// clarificationStillWaiting checks the forge is still waiting for the answer.
func clarificationStillWaiting(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	root, err := singleForge(w)
	if err != nil {
		return err
	}
	// Wait for the bridge to have handled everything before the operator's
	// events, so an answer it should not have carried back shows up here.
	if err := w.caughtUp(ctx); err != nil {
		return err
	}
	project := captures[1]
	pending := filepath.Join(clarificationPaths(root, project).pending, clarificationID(project)+".request")
	if _, err := os.Stat(pending); err != nil {
		return fmt.Errorf("the clarification for %s is no longer pending: %v", project, err)
	}
	return nil
}

// operatorAnsweredFromTheDesktop resolves the clarification the way the
// desktop dashboard does.
func operatorAnsweredFromTheDesktop(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := singleForge(w)
	if err != nil {
		return err
	}
	ctx, cancel := stepContext()
	defer cancel()
	url, err := dashboard.URL(root, "")
	if err != nil {
		return err
	}
	return dashboard.NewAPI(url).AnswerClarification(ctx, captures[1], clarificationID(captures[1]), captures[2])
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
