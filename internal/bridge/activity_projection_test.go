package bridge

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/unclebob/forgelet-bridge/internal/relay"
	"github.com/unclebob/forgelet-bridge/internal/state"
)

type countingBoard struct {
	board *fakeBoard
	calls int
	err   error
}

func (b *countingBoard) Boards() ([]relay.Board, error) {
	b.calls++
	if b.err != nil {
		return nil, b.err
	}
	return b.board.Boards()
}

func TestActivityFromChoosesProjectionBoardsOrTheCompatibleStore(t *testing.T) {
	tests := []struct {
		name          string
		fromSnapshot  bool
		boards        []relay.Board
		wantStoreRead int
	}{
		{name: "no snapshot source", boards: []relay.Board{{Project: "forgelet-bridge"}}, wantStoreRead: 1},
		{name: "snapshot without boards", fromSnapshot: true, wantStoreRead: 1},
		{name: "snapshot boards", fromSnapshot: true, boards: []relay.Board{}, wantStoreRead: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			board := &countingBoard{board: &fakeBoard{}}
			built, _ := newTestBridgeWithStores(t, &fakeRooms{},
				map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
				map[string]ApprovalStore{"/forges/forge-a": &fakeApprovals{}},
				map[string]BoardStore{"/forges/forge-a": board}, "/forges/forge-a")
			_, err := built.activityFrom(context.Background(), "/forges/forge-a",
				Room{ActivityRoomID: "!activity-forge-a"},
				relay.DashboardSnapshot{Boards: test.boards}, test.fromSnapshot)
			if err != nil {
				t.Fatalf("activityFrom: %v", err)
			}
			if board.calls != test.wantStoreRead {
				t.Errorf("board store reads = %d, want %d", board.calls, test.wantStoreRead)
			}
		})
	}
}

func TestActivityFromFallsBackWhenTheProjectionHasNoLanes(t *testing.T) {
	board := &countingBoard{board: &fakeBoard{}}
	built, _ := newTestBridgeWithStores(t, &fakeRooms{},
		map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": &fakeApprovals{}},
		map[string]BoardStore{"/forges/forge-a": board}, "/forges/forge-a")
	if _, err := built.activityFrom(context.Background(), "/forges/forge-a",
		Room{ActivityRoomID: "!activity-forge-a"},
		relay.DashboardSnapshot{BoardFallback: true}, true); err != nil {
		t.Fatalf("activityFrom fallback: %v", err)
	}
	if board.calls != 1 {
		t.Errorf("board store reads = %d, want one fallback read", board.calls)
	}
}

func TestActivityFromReportsAFallbackBoardFailure(t *testing.T) {
	board := &countingBoard{board: &fakeBoard{}, err: os.ErrPermission}
	built, _ := newTestBridgeWithStores(t, &fakeRooms{},
		map[string]ForgeStore{"/forges/forge-a": &fakeStore{}},
		map[string]ApprovalStore{"/forges/forge-a": &fakeApprovals{}},
		map[string]BoardStore{"/forges/forge-a": board}, "/forges/forge-a")
	written, err := built.activityFrom(context.Background(), "/forges/forge-a",
		Room{ActivityRoomID: "!activity-forge-a"},
		relay.DashboardSnapshot{BoardFallback: true}, true)
	if err == nil {
		t.Fatal("activityFrom accepted a fallback board failure")
	}
	if written != 0 {
		t.Errorf("activity work = %d, want none after a board failure", written)
	}
}

func TestSaveQuietCardsWritesOnlyWhenItSeedsHistory(t *testing.T) {
	built, _ := newTestBridge(t, &fakeRooms{}, map[string]ForgeStore{
		"/forges/forge-a": &fakeStore{},
	}, "/forges/forge-a")
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	built.statePath = filepath.Join(blocked, "state.json")
	if err := built.saveQuietCards("/forges/forge-a", nil); err != nil {
		t.Fatalf("nothing to seed tried to save: %v", err)
	}

	built.statePath = filepath.Join(t.TempDir(), "state.json")
	if err := built.saveQuietCards("/forges/forge-a", finishedCards(1)); err != nil {
		t.Fatalf("saveQuietCards: %v", err)
	}
	reloaded, err := state.Load(built.statePath)
	if err != nil {
		t.Fatalf("Load state: %v", err)
	}
	if _, known := reloaded.Relay.Activity["forgelet-bridge/finished-card-0"]; !known {
		t.Fatal("the quiet card was not saved")
	}
}
