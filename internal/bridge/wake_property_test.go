//go:build property

package bridge

import (
	"context"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

// TestPropertyAPhoneMessageIsHandedOverOnce checks what the operator's phone
// depends on: a message the forge has taken is handed over exactly once,
// however many ticks run while the queue has not shown the request it became
// yet, so a tick that arrives early cannot double it.
func TestPropertyAPhoneMessageIsHandedOverOnce(t *testing.T) {
	property := func(ticks uint8) bool {
		store := &fakeStore{}
		rooms := &fakeRooms{}
		built, _ := newTestBridge(t, rooms, map[string]ForgeStore{"/forges/forge-a": store}, "/forges/forge-a")
		rooms.push(relay.RoomEvent{
			RoomID:  "!room-forge-a",
			EventID: "$operator-message",
			Sender:  operator,
			Body:    "is the build green?",
		})

		for range ticks {
			if err := built.Tick(context.Background()); err != nil {
				return false
			}
			// The forge has taken the message, but its queue does not show the
			// request the message became.
			store.mu.Lock()
			store.requests = nil
			store.mu.Unlock()
		}

		handedOver := len(store.createdBodies())
		if ticks == 0 {
			return handedOver == 0
		}
		return handedOver == 1
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 60,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(uint8(rnd.Intn(8)))
		},
	}); err != nil {
		t.Error(err)
	}
}
