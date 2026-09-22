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
