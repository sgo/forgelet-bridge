package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/internal/relay"
)

// dashboardHoldsAppReport seeds the request file the Forgelet app's completed
// card report creates, including the safe source header the dashboard exposes.
func dashboardHoldsAppReport(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := singleForge(w)
	if err != nil {
		return err
	}
	store := w.dashboards[filepath.Base(root)]
	if store == nil {
		return fmt.Errorf("the fixture forge root %s has no dashboard queue", root)
	}
	id, err := store.CreateRequest(captures[1])
	if err != nil {
		return err
	}
	path := filepath.Join(root, ".swarmforge", "dashboard", "requests", "pending", id+".request")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	status := "status: pending\n"
	source := "source: " + relay.AppCompletedCardReportSource + "\n"
	updated := strings.Replace(string(data), status, status+source, 1)
	if updated == string(data) {
		return fmt.Errorf("the dashboard request has no status header to carry its source")
	}
	return os.WriteFile(path, []byte(updated), 0o644)
}

// operatorDecryptsAppRelayedReport waits for the exact presentation and keeps
// its event as the thread root the Lieutenant's answer must use.
func operatorDecryptsAppRelayedReport(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	body := appRelayedBody(captures[1])
	roomID, _, message, err := w.waitForChatMessage(ctx, body)
	if err != nil {
		return err
	}
	if !message.Encrypted {
		return fmt.Errorf("the app-relayed report %q was not decrypted from an encrypted event", captures[1])
	}
	if message.Sender != w.bridgeUserID {
		return fmt.Errorf("the app-relayed report %q was sent by %s, want the bridge account", captures[1], message.Sender)
	}
	w.anchors[appReportAnchorKey(captures[1])] = message.EventID
	_ = roomID
	return nil
}

// bridgeSentAppRelayedReportEncrypted checks the Matrix event itself is
// encrypted, rather than relying only on the operator's decrypted view.
func bridgeSentAppRelayedReportEncrypted(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomID, operator, message, err := w.waitForChatMessage(ctx, appRelayedBody(captures[1]))
	if err != nil {
		return err
	}
	raw, err := operator.Client().Messages(ctx, id.RoomID(roomID), "", "", mautrix.DirectionBackward, nil, 50)
	if err != nil {
		return err
	}
	for _, evt := range raw.Chunk {
		if evt.ID.String() == message.EventID {
			if evt.Type != event.EventEncrypted {
				return fmt.Errorf("the bridge sent the app report as %s, want %s", evt.Type, event.EventEncrypted)
			}
			return nil
		}
	}
	return fmt.Errorf("the room holds no event %s for the app report %q", message.EventID, captures[1])
}

// operatorDecryptsAppReportReply checks the answer landed in the thread rooted
// at the app-relayed report rather than under a later or ordinary request.
func operatorDecryptsAppReportReply(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	anchor := w.anchors[appReportAnchorKey(captures[2])]
	if anchor == "" {
		return fmt.Errorf("the scenario never read the app-relayed report %q", captures[2])
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
		return fmt.Errorf("the app report's thread reply %q was not decrypted from an encrypted event", captures[1])
	}
	return nil
}

// roomHoldsOneAppRelayedReport checks a restart left one explicit app report,
// not a second copy with the same dashboard request id.
func roomHoldsOneAppRelayedReport(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	roomID, err := w.chatRoom(ctx, "Chat")
	if err != nil {
		return err
	}
	operator, err := w.operator(ctx)
	if err != nil {
		return err
	}
	report := captures[2]
	body := appRelayedBody(report)
	if err := waitFor(ctx, fmt.Sprintf("chat room Chat never held the app-relayed report %q", report), func() (bool, error) {
		return countMessages(operator, roomID, body, false) >= 1, nil
	}); err != nil {
		return err
	}
	if err := fixtures.Sleep(ctx, settle); err != nil {
		return err
	}
	if copies := countMessages(operator, roomID, body, false); copies != 1 {
		return fmt.Errorf("chat room Chat holds %d app-relayed copies of %q, want exactly one", copies, report)
	}
	return nil
}

func appRelayedBody(report string) string {
	return relay.AppReportPrefix + report
}

func appReportAnchorKey(report string) string {
	return "app-report:" + report
}
