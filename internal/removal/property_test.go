//go:build property

package removal

import (
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/unclebob/forgelet-bridge/internal/config"
	"github.com/unclebob/forgelet-bridge/internal/state"
)

// TestPropertyRemovalPlansEveryDistinctRecordedRoomOnce is the core cleanup
// property: adding projects or duplicate references may not add a second visit
// or omit any room recorded for the forge.
func TestPropertyRemovalPlansEveryDistinctRecordedRoomOnce(t *testing.T) {
	property := func(seed int64) bool {
		rnd := rand.New(rand.NewSource(seed))
		projects := map[string]state.Project{}
		pool := []string{
			"!project-a:example.org",
			"!project-b:example.org",
			"!project-c:example.org",
			"!chat:example.org",
		}
		for index := rnd.Intn(12); index > 0; index-- {
			projects[randomProjectName(rnd)] = state.Project{RoomID: pool[rnd.Intn(len(pool))]}
		}
		forge := state.Forge{
			SpaceID:              "!space:example.org",
			RoomID:               "!chat:example.org",
			ApprovalsRoomID:      "!approvals:example.org",
			ActivityRoomID:       "!activity:example.org",
			ClarificationsRoomID: "!clarifications:example.org",
			Projects:             projects,
		}
		loaded := &state.State{Forges: map[string]state.Forge{"/forges/one": forge}}
		loaded.EnsureMaps()
		cfg := config.Config{Forges: []config.Forge{{Root: "/forges/one"}}}

		first := Inspect(cfg, loaded, "/forges/one")
		second := Inspect(cfg, loaded, "/forges/one")
		if first.Result.Status != Ready || !sameRooms(first.Rooms, second.Rooms) {
			return false
		}

		recorded := []string{
			forge.SpaceID,
			forge.RoomID,
			forge.ApprovalsRoomID,
			forge.ActivityRoomID,
			forge.ClarificationsRoomID,
		}
		for _, project := range projects {
			recorded = append(recorded, project.RoomID)
		}
		want := map[string]bool{}
		for _, roomID := range recorded {
			want[roomID] = true
		}
		if len(first.Rooms) != len(want) {
			return false
		}
		seen := map[string]bool{}
		for _, roomID := range first.Rooms {
			if !want[roomID] || seen[roomID] {
				return false
			}
			seen[roomID] = true
		}
		return len(seen) == len(want)
	}
	if err := quick.Check(property, &quick.Config{MaxCount: 500}); err != nil {
		t.Error(err)
	}
}

// TestPropertyUnsafeRootsNeverPlanMatrixWork checks that validation with no
// configured exact root cannot leak a Matrix target into the removal service.
func TestPropertyUnsafeRootsNeverPlanMatrixWork(t *testing.T) {
	cfg := config.Config{Forges: []config.Forge{{Root: "/forges/one"}}}
	loaded := &state.State{Forges: map[string]state.Forge{
		"/forges/one": {
			SpaceID:              "!space:example.org",
			RoomID:               "!chat:example.org",
			ApprovalsRoomID:      "!approvals:example.org",
			ActivityRoomID:       "!activity:example.org",
			ClarificationsRoomID: "!clarifications:example.org",
		},
	}}
	property := func(root string) bool {
		got := Inspect(cfg, loaded, root)
		return got.Root == "" && got.Forge == nil && len(got.Rooms) == 0
	}
	values := func(values []reflect.Value, rnd *rand.Rand) {
		candidates := []string{"", "one", "../one", "/forges/../one", "/forges/two"}
		values[0] = reflect.ValueOf(candidates[rnd.Intn(len(candidates))])
	}
	if err := quick.Check(property, &quick.Config{MaxCount: 200, Values: values}); err != nil {
		t.Error(err)
	}
}

func randomProjectName(rnd *rand.Rand) string {
	const letters = "abcdefghijklmnop"
	return string(letters[rnd.Intn(len(letters))])
}

func sameRooms(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
