package board

import (
	"os"
	"path/filepath"
	"testing"
)

func newStore(t *testing.T, projects ...string) *Store {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".swarmforge"), 0o755); err != nil {
		t.Fatal(err)
	}
	list := ""
	for _, project := range projects {
		list += project + "\n"
		if err := os.MkdirAll(filepath.Join(root, "projects", project, ".swarmforge", "board"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".swarmforge", "open-projects"), []byte(list), 0o644); err != nil {
		t.Fatal(err)
	}
	return New(root)
}

func writeBoard(t *testing.T, store *Store, project, body string) {
	t.Helper()
	path := filepath.Join(store.root, "projects", project, ".swarmforge", "board", "tasks.tsv")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeLanes(t *testing.T, store *Store, project, body string) {
	t.Helper()
	path := filepath.Join(store.root, "projects", project, ".swarmforge", "roles.tsv")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBoardsReadsEachOpenProjectsLanesAndCards(t *testing.T) {
	store := newStore(t, "forgelet-bridge", "forgelet-app")
	writeLanes(t, store, "forgelet-bridge", "specifier\tmaster\t/w\tw\tSpecifier\tcodex\ttask\tforward-only\ncoder\tcoder\t/w\tw\tCoder\tcodex\ttask\tforward-only\n")
	writeBoard(t, store, "forgelet-bridge", "card-activity-feed\tcoder\t2026-09-22T13:04:00Z\t2026-09-22T13:04:00Z\tid-1\t0\n")
	writeBoard(t, store, "forgelet-app", "a-card\tmaster\t2026-09-22T13:04:00Z\t2026-09-22T13:04:00Z\tid-2\t0\n")

	boards, err := store.Boards()
	if err != nil {
		t.Fatalf("Boards: %v", err)
	}
	if len(boards) != 2 {
		t.Fatalf("boards = %+v, want every open project", boards)
	}
	if boards[0].Project != "forgelet-app" || boards[1].Project != "forgelet-bridge" {
		t.Errorf("projects = %q, %q, want the open projects in order", boards[0].Project, boards[1].Project)
	}
	bridge := boards[1]
	if len(bridge.Lanes) != 2 || bridge.Lanes[0] != "specifier" || bridge.Lanes[1] != "coder" {
		t.Errorf("lanes = %v, want the project's roles", bridge.Lanes)
	}
	if len(bridge.Cards) != 1 || bridge.Cards[0].Name != "card-activity-feed" || bridge.Cards[0].Lane != "coder" {
		t.Errorf("cards = %+v, want the board's cards", bridge.Cards)
	}
}

func TestLanesForLeavesAProjectWithoutRolesLaneLess(t *testing.T) {
	store := newStore(t, "forgelet-bridge")

	lanes, err := store.LanesFor("forgelet-bridge")
	if err != nil {
		t.Fatalf("LanesFor: %v", err)
	}
	if len(lanes) != 0 {
		t.Errorf("lanes = %v, want none for a project with no roles file", lanes)
	}
}

func TestCardsReadsTheBoardRows(t *testing.T) {
	store := newStore(t, "forgelet-bridge")
	writeBoard(t, store, "forgelet-bridge",
		"card-activity-feed\tspecifier\t2026-09-22T13:04:00Z\t2026-09-22T13:04:00Z\t20260922T130400589904Z-card-activity-feed\t0\n"+
			"another-card\tdone\t2026-09-22T13:00:00Z\t2026-09-22T13:10:00Z\tid-2\t2\n")

	cards, err := store.Cards()
	if err != nil {
		t.Fatalf("Cards: %v", err)
	}
	if len(cards) != 2 {
		t.Fatalf("cards = %+v, want two", cards)
	}
	if cards[0].Name != "another-card" || cards[0].Project != "forgelet-bridge" || cards[0].Lane != "done" {
		t.Errorf("first card = %+v", cards[0])
	}
	if !cards[0].Done() {
		t.Error("a card in the done lane should be finished")
	}
	if cards[1].Name != "card-activity-feed" || cards[1].Lane != "specifier" || cards[1].Done() {
		t.Errorf("second card = %+v", cards[1])
	}
}

func TestCardsIgnoresProjectsThatAreNotOpen(t *testing.T) {
	store := newStore(t, "forgelet-bridge")
	writeBoard(t, store, "another-project", "card\tspecifier\tnow\tnow\tid\t0\n")

	cards, err := store.Cards()
	if err != nil {
		t.Fatalf("Cards: %v", err)
	}
	if len(cards) != 0 {
		t.Errorf("cards = %+v, want none from a closed project", cards)
	}
}

func TestCardsOfAProjectWithoutABoardIsEmpty(t *testing.T) {
	store := newStore(t, "forgelet-bridge")
	cards, err := store.Cards()
	if err != nil {
		t.Fatalf("Cards: %v", err)
	}
	if len(cards) != 0 {
		t.Errorf("cards = %+v, want none", cards)
	}
}

func TestParseRowIgnoresMalformedLines(t *testing.T) {
	for _, line := range []string{"", "only-a-name", "\t lane-only", "name\t"} {
		if _, _, ok := ParseRow(line); ok {
			t.Errorf("ParseRow(%q) accepted a malformed row", line)
		}
	}
}

func TestParseRowReadsARowOfTwoColumns(t *testing.T) {
	name, lane, ok := ParseRow("card-activity-feed\tspecifier")

	if !ok || name != "card-activity-feed" || lane != "specifier" {
		t.Errorf("ParseRow = %q, %q, %v, want the card and its lane", name, lane, ok)
	}
}
