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
