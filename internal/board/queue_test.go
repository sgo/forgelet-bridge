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

	cards, err := Queue{Store: store, Forge: "/forges/forge-a"}.Cards()
	if err != nil {
		t.Fatalf("Cards: %v", err)
	}

	want := []relay.Card{
		{Key: Key("/forges/forge-a", "forgelet-bridge", "card-activity-feed"), Project: "forgelet-bridge", Name: "card-activity-feed", Lane: "specifier"},
		{Key: Key("/forges/forge-a", "forgelet-bridge", "phone-approvals"), Project: "forgelet-bridge", Name: "phone-approvals", Lane: DoneLane, Done: true},
	}
	if !reflect.DeepEqual(cards, want) {
		t.Errorf("cards = %+v, want %+v", cards, want)
	}
}

func TestKeyKeepsProjectsAndForgesApart(t *testing.T) {
	first := Key("/forges/forge-a", "forgelet-bridge", "card-activity-feed")
	second := Key("/forges/forge-a", "saibill", "card-activity-feed")
	third := Key("/forges/forge-b", "forgelet-bridge", "card-activity-feed")

	if first == second || first == third {
		t.Errorf("key = %q, the same for two projects or two forges", first)
	}
	if first != "/forges/forge-a/forgelet-bridge/card-activity-feed" {
		t.Errorf("key = %q, want the forge, the project and the card", first)
	}
}
