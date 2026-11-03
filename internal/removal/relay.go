package removal

import (
	"strings"

	"github.com/unclebob/forgelet-bridge/internal/relay"
	"github.com/unclebob/forgelet-bridge/internal/state"
)

func purgeRelay(bookkeeping *relay.State, root string, forge state.Forge) {
	bookkeeping.EnsureMaps()
	bookkeeping.ForgetScoped(root)
	rooms := removalRooms(forge)
	purgeWaiting(bookkeeping, rooms)
	purgeRoomStates(bookkeeping.Approvals, root, rooms,
		func(approval relay.ApprovalState) string { return approval.ForgeRoot },
		func(approval relay.ApprovalState) string { return approval.RoomID })
	purgeRoomStates(bookkeeping.Clarifications, root, rooms,
		func(clarification relay.ClarificationState) string { return clarification.ForgeRoot },
		func(clarification relay.ClarificationState) string { return clarification.RoomID })
	purgeActivity(bookkeeping, root)
}

func removalRooms(forge state.Forge) map[string]bool {
	rooms := make(map[string]bool)
	for _, roomID := range orderedRooms(forge) {
		rooms[roomID] = true
	}
	return rooms
}

func purgeWaiting(bookkeeping *relay.State, rooms map[string]bool) {
	for key := range bookkeeping.Waiting {
		roomID, _, _, ok := splitWaiting(key)
		if ok && rooms[roomID] {
			delete(bookkeeping.Waiting, key)
		}
	}
}

type roomState interface {
	relay.ApprovalState | relay.ClarificationState
}

func purgeRoomStates[T roomState](
	entries map[string]T,
	root string,
	rooms map[string]bool,
	owner, roomID func(T) string,
) {
	for key, entry := range entries {
		if owner(entry) == root || rooms[roomID(entry)] {
			delete(entries, key)
		}
	}
}

func purgeActivity(bookkeeping *relay.State, root string) {
	for key, activity := range bookkeeping.Activity {
		if activity.ForgeRoot == root {
			delete(bookkeeping.Activity, key)
		}
	}
}

func splitWaiting(key string) (string, string, string, bool) {
	parts := strings.SplitN(key, "\n", 3)
	if len(parts) != 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-11-03T00:54:46+01:00","module_hash":"0eeb155c59c0c10635718c37c99f5eaef3986bc7d0f1dd7af94fbc05bcec1ead","functions":[{"id":"func/purgeRelay","name":"purgeRelay","line":10,"end_line":22,"hash":"4b90223ed35af6aca61a3d9e7213fbb0b504cc9edc0a20c30589e284b40dc4d7"},{"id":"func/removalRooms","name":"removalRooms","line":24,"end_line":30,"hash":"76c377c727d9e79fbee783e421b844052f5f8a02a6b4ae835e6af0304572667d"},{"id":"func/purgeWaiting","name":"purgeWaiting","line":32,"end_line":39,"hash":"77b4449bec5ba22c45c43d30b32669234f53e507e46aeaca3b7fe7723d4c584f"},{"id":"func/purgeRoomStates","name":"purgeRoomStates","line":45,"end_line":56,"hash":"e4961743f601281a11e5438df8e6db49f6443b57016801c8ccb6aad4cbd3f779"},{"id":"func/purgeActivity","name":"purgeActivity","line":58,"end_line":64,"hash":"415aa022553e033a25a1849b2a3759f001dfd54db5c2835c94a31e3e3a01205a"},{"id":"func/splitWaiting","name":"splitWaiting","line":66,"end_line":72,"hash":"1a50ca1b5c56d7502b24b853e012e26d80ba2426de664932732cda2550f14fea"}]}
// mutate4go-manifest-end
