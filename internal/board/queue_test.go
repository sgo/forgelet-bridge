package board

import (
	"reflect"
	"testing"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

func TestQueuePresentsTheBoardInRelayForm(t *testing.T) {
	store := newStore(t, "forgelet-bridge")
	writeBoard(t, store, "forgelet-bridge",
		"card-activity-feed\tspecifier\t2026-09-22T15:00:00Z\t2026-09-22T15:00:00Z\ttask-1\t0\n"+
			"phone-approvals\tdone\t2026-09-22T15:01:00Z\t2026-09-22T15:02:00Z\ttask-2\t0\n")

	cards, err := Queue{Store: store}.Cards()
	if err != nil {
		t.Fatalf("Cards: %v", err)
	}

	want := []relay.Card{
		{Key: Key("forgelet-bridge", "card-activity-feed"), Project: "forgelet-bridge", Name: "card-activity-feed", Lane: "specifier"},
		{Key: Key("forgelet-bridge", "phone-approvals"), Project: "forgelet-bridge", Name: "phone-approvals", Lane: DoneLane, Done: true},
	}
	if !reflect.DeepEqual(cards, want) {
		t.Errorf("cards = %+v, want %+v", cards, want)
	}
}

func TestKeyKeepsProjectsApart(t *testing.T) {
	first := Key("forgelet-bridge", "card-activity-feed")
	second := Key("saibill", "card-activity-feed")

	if first == second {
		t.Errorf("key = %q, the same for two projects", first)
	}
	if first != "forgelet-bridge/card-activity-feed" {
		t.Errorf("key = %q, want the project and the card", first)
	}
}
