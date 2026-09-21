package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/internal/dashboard"
)

// settle is how long an "exactly one" step watches for a duplicate after the
// first copy arrives.
const settle = 1500 * time.Millisecond

func dashboardHoldsRequest(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := singleForge(w)
	if err != nil {
		return err
	}
	store := w.dashboards[filepath.Base(root)]
	if store == nil {
		return fmt.Errorf("no fixture dashboard for %s", root)
	}
	_, err = store.CreateRequest(captures[1])
	return err
}

func lieutenantAnswers(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	var request foundRequest
	err := waitFor(ctx, fmt.Sprintf("the forge holds no chat request reading %q", captures[1]), func() (bool, error) {
		found, ok, err := findRequest(w, captures[1])
		if err != nil {
			return false, err
		}
		request = found
		return ok, nil
	})
	if err != nil {
		return err
	}
	return request.store.Answer(request.id, captures[2])
}

type foundRequest struct {
	store *dashboard.Store
	id    string
}

func findRequest(w *World, body string) (foundRequest, bool, error) {
	for _, root := range w.configured {
		store := w.dashboards[filepath.Base(root)]
		if store == nil {
			continue
		}
		requests, err := store.Requests()
		if err != nil {
			return foundRequest{}, false, err
		}
		for _, request := range requests {
			if request.Body == body {
				return foundRequest{store: store, id: request.ID}, true, nil
			}
		}
	}
	return foundRequest{}, false, nil
}

func operatorDecryptsChatMessage(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomID, operator, message, err := w.waitForChatMessage(ctx, captures[1])
	if err != nil {
		return err
	}
	if !message.Encrypted {
		return fmt.Errorf("the chat message %q was not decrypted from an encrypted event", captures[1])
	}
	w.anchors[captures[1]] = message.EventID
	_ = operator
	_ = roomID
	return nil
}

func bridgeSentEncrypted(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomID, operator, message, err := w.waitForChatMessage(ctx, captures[1])
	if err != nil {
		return err
	}
	raw, err := operator.Client().Messages(ctx, id.RoomID(roomID), "", "", mautrix.DirectionBackward, nil, 50)
	if err != nil {
		return err
	}
	for _, evt := range raw.Chunk {
		if evt.ID.String() != message.EventID {
			continue
		}
		if evt.Type != event.EventEncrypted {
			return fmt.Errorf("the bridge sent %q as %s, want %s", captures[1], evt.Type, event.EventEncrypted)
		}
		return nil
	}
	return fmt.Errorf("the room holds no event %s for the chat message %q", message.EventID, captures[1])
}

func operatorDecryptsThreadReply(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	anchor, err := w.chatMessageAnchor(ctx, captures[2])
	if err != nil {
		return err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	roomID, err := w.chatRoom(ctx, "Chat")
	if err != nil {
		return err
	}
	reply, err := operator.WaitForThreadReply(ctx, roomID, anchor, captures[1], stepTimeout)
	if err != nil {
		return err
	}
	if !reply.Encrypted {
		return fmt.Errorf("the thread reply %q was not decrypted from an encrypted event", captures[1])
	}
	return nil
}

// chatMessageAnchor is the chat message a thread reply belongs under. The
// operator has to have read that message, so this waits for it.
func (w *World) chatMessageAnchor(ctx context.Context, body string) (string, error) {
	if anchor, ok := w.anchors[body]; ok {
		return anchor, nil
	}
	_, _, message, err := w.waitForChatMessage(ctx, body)
	if err != nil {
		return "", err
	}
	w.anchors[body] = message.EventID
	return message.EventID, nil
}

func operatorSendsMessage(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	roomID, err := w.chatRoom(ctx, captures[2])
	if err != nil {
		return err
	}
	_, err = operator.Send(ctx, roomID, captures[1])
	return err
}

func userSendsMessage(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	user, err := w.user(ctx, captures[1])
	if err != nil {
		return err
	}
	roomID, err := w.chatRoom(ctx, captures[3])
	if err != nil {
		return err
	}
	_, err = user.Send(ctx, roomID, captures[2])
	return err
}

func userJoinedChatRoom(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	userID := captures[1]
	roomID, err := w.chatRoom(ctx, captures[2])
	if err != nil {
		return err
	}
	if user, err := w.user(ctx, userID); err == nil {
		for _, room := range mustRooms(ctx, user) {
			if room == roomID {
				return nil
			}
		}
	} else {
		return err
	}

	// The bridge only invites the operator, so the operator has to add anyone
	// else, the way they would add a friend in Element.
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	if err := operator.Invite(ctx, roomID, userID); err != nil {
		return err
	}
	user, err := w.user(ctx, userID)
	if err != nil {
		return err
	}
	return waitFor(ctx, fmt.Sprintf("%s never joined %s", userID, captures[2]), func() (bool, error) {
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

func forgeHoldsRequest(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	return waitFor(ctx, fmt.Sprintf("the forge never held a chat request reading %q", captures[1]), func() (bool, error) {
		count, err := countRequests(w, captures[1])
		return count >= 1, err
	})
}

func forgeHoldsOneRequest(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	if err := waitFor(ctx, fmt.Sprintf("the forge never held a chat request reading %q", captures[1]), func() (bool, error) {
		count, err := countRequests(w, captures[1])
		return count >= 1, err
	}); err != nil {
		return err
	}
	if err := sleep(ctx, settle); err != nil {
		return err
	}
	count, err := countRequests(w, captures[1])
	if err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("the forge holds %d chat requests reading %q, want exactly one", count, captures[1])
	}
	return nil
}

func forgeHoldsRequests(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	want := captures[1]
	return waitFor(ctx, fmt.Sprintf("the forge never held %s chat requests", want), func() (bool, error) {
		count, err := countRequests(w, "")
		if err != nil {
			return false, err
		}
		return fmt.Sprintf("%d", count) == want, nil
	})
}

func lieutenantWoken(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := singleForge(w)
	if err != nil {
		return err
	}
	ctx, cancel := stepContext()
	defer cancel()
	wakeLog := filepath.Join(root, ".swarmforge", "dashboard", "wake.log")
	body := captures[1]
	return waitFor(ctx, fmt.Sprintf("the lieutenant was never woken with %q", body), func() (bool, error) {
		data, err := os.ReadFile(wakeLog)
		if err != nil {
			if os.IsNotExist(err) {
				return false, nil
			}
			return false, err
		}
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			if _, recorded, found := strings.Cut(line, "\t"); found && recorded == body {
				return true, nil
			}
		}
		return false, nil
	})
}

func roomHoldsOneChatMessage(_ context.Context, world any, captures []string) error {
	return expectMessages(world.(*World), captures[1], captures[2], false)
}

func roomHoldsOneThreadReply(_ context.Context, world any, captures []string) error {
	return expectMessages(world.(*World), captures[1], captures[2], true)
}

func expectMessages(w *World, roomName, body string, threadOnly bool) error {
	ctx, cancel := stepContext()
	defer cancel()
	roomID, err := w.chatRoom(ctx, roomName)
	if err != nil {
		return err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	count := func() int {
		found := 0
		for _, message := range operator.Messages(roomID) {
			if message.Body != body {
				continue
			}
			if threadOnly && message.ThreadRoot == "" {
				continue
			}
			found++
		}
		return found
	}

	if err := waitFor(ctx, fmt.Sprintf("chat room %s never held %q", roomName, body), func() (bool, error) {
		return count() >= 1, nil
	}); err != nil {
		return err
	}
	if err := sleep(ctx, settle); err != nil {
		return err
	}
	if found := count(); found != 1 {
		kind := "chat messages"
		if threadOnly {
			kind = "thread replies"
		}
		return fmt.Errorf("chat room %s holds %d %s reading %q, want exactly one", roomName, found, kind, body)
	}
	return nil
}

// waitForChatMessage waits until the operator has decrypted a message.
func (w *World) waitForChatMessage(ctx context.Context, body string) (string, *fixtures.User, fixtures.Message, error) {
	operator, err := w.operator(ctx)
	if err != nil {
		return "", nil, fixtures.Message{}, err
	}
	roomID, err := w.chatRoom(ctx, "Chat")
	if err != nil {
		return "", nil, fixtures.Message{}, err
	}
	message, err := operator.WaitForMessage(ctx, roomID, body, stepTimeout)
	if err != nil {
		return "", nil, fixtures.Message{}, err
	}
	return roomID, operator, message, nil
}

func countRequests(w *World, body string) (int, error) {
	count := 0
	for _, root := range w.configured {
		store := w.dashboards[filepath.Base(root)]
		if store == nil {
			continue
		}
		requests, err := store.Requests()
		if err != nil {
			return 0, err
		}
		for _, request := range requests {
			if body == "" || request.Body == body {
				count++
			}
		}
	}
	return count, nil
}

func mustRooms(ctx context.Context, user *fixtures.User) []string {
	rooms, err := user.Rooms(ctx)
	if err != nil {
		return nil
	}
	return rooms
}
