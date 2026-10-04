package board

import (
	"reflect"
	"testing"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

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

func TestQueuePresentsEachBoardInRelayForm(t *testing.T) {
	store := newStore(t, "forgelet-bridge")
	writeLanes(t, store, "forgelet-bridge", "specifier\tmaster\t/w\tw\tSpecifier\tcodex\ttask\tforward-only\ncoder\tcoder\t/w\tw\tCoder\tcodex\ttask\tforward-only\n")
	writeBoard(t, store, "forgelet-bridge",
		"card-activity-feed\tspecifier\t2026-09-22T15:00:00Z\t2026-09-22T15:00:00Z\ttask-1\t0\n"+
			"phone-approvals\tdone\t2026-09-22T15:01:00Z\t2026-09-22T15:02:00Z\ttask-2\t0\n")

	boards, err := Queue{Store: store, Forge: "/forges/forge-a"}.Boards()
	if err != nil {
		t.Fatalf("Boards: %v", err)
	}
	if len(boards) != 1 {
		t.Fatalf("boards = %+v, want the one open project", boards)
	}
	board := boards[0]
	if board.Project != "forgelet-bridge" {
		t.Errorf("project = %q, want the open project", board.Project)
	}
	if !reflect.DeepEqual(board.Lanes, []string{"specifier", "coder"}) {
		t.Errorf("lanes = %v, want the project's roles", board.Lanes)
	}
	want := []relay.Card{
		{Key: Key("/forges/forge-a", "forgelet-bridge", "card-activity-feed"), Project: "forgelet-bridge", Name: "card-activity-feed", Lane: "specifier"},
		{Key: Key("/forges/forge-a", "forgelet-bridge", "phone-approvals"), Project: "forgelet-bridge", Name: "phone-approvals", Lane: DoneLane, Done: true},
	}
	if !reflect.DeepEqual(board.Cards, want) {
		t.Errorf("cards = %+v, want %+v", board.Cards, want)
	}
}
