//go:build property

package removal

import (
	"math/rand"
	"strings"
	"testing"
	"testing/quick"

	"github.com/unclebob/forgelet-bridge/internal/config"
	"github.com/unclebob/forgelet-bridge/internal/relay"
	"github.com/unclebob/forgelet-bridge/internal/state"
)

func TestPropertyProjectRemovalPlansOnlyTheRecordedTargetRoom(t *testing.T) {
	property := func(seed int64) bool {
		rnd := rand.New(rand.NewSource(seed))
		names := []string{"target", "sibling", "third", "a-project"}
		target := names[rnd.Intn(len(names))]
		sibling := ""
		for _, candidate := range names {
			if candidate != target {
				sibling = candidate
				break
			}
		}
		loaded := &state.State{Forges: map[string]state.Forge{
			"/forges/one": {
				SpaceID:              "!space:example.org",
				RoomID:               "!chat:example.org",
				ApprovalsRoomID:      "!approvals:example.org",
				ActivityRoomID:       "!activity:example.org",
				ClarificationsRoomID: "!clarifications:example.org",
				Projects: map[string]state.Project{
					target:  {RoomID: "!target:example.org"},
					sibling: {RoomID: "!sibling:example.org"},
				},
			},
		}}
		loaded.EnsureMaps()
		cfg := config.Config{Forges: []config.Forge{{Root: "/forges/one"}}}
		got := InspectProject(cfg, loaded, "/forges/one", target,
			fakeProjectCatalog{forgeExists: true, managed: true})
		return got.Result.Status == Ready &&
			got.ProjectName == target &&
			got.RoomID == "!target:example.org" &&
			got.RoomID != loaded.Forges["/forges/one"].Projects[sibling].RoomID
	}
	if err := quick.Check(property, &quick.Config{MaxCount: 300}); err != nil {
		t.Error(err)
	}
}

func TestPropertyProjectPurgePreservesEverySiblingOwner(t *testing.T) {
	property := func(seed int64) bool {
		rnd := rand.New(rand.NewSource(seed))
		var bookkeeping relay.State
		bookkeeping.EnsureMaps()
		for index := 0; index < rnd.Intn(12); index++ {
			key := "target-" + string(rune('a'+index))
			bookkeeping.Waiting["!target:example.org\n"+relay.BoardFactType+"\n"+key] = "{}"
			bookkeeping.Waiting["!sibling:example.org\n"+relay.BoardFactType+"\n"+key] = "{}"
			bookkeeping.Approvals["target/"+key] = relay.ApprovalState{RoomID: "!approvals:example.org"}
			bookkeeping.Approvals["sibling/"+key] = relay.ApprovalState{RoomID: "!approvals:example.org"}
			bookkeeping.Clarifications["target/"+key] = relay.ClarificationState{RoomID: "!target:example.org"}
			bookkeeping.Clarifications["sibling/"+key] = relay.ClarificationState{RoomID: "!sibling:example.org"}
			bookkeeping.Activity["target/"+key] = relay.CardState{ForgeRoot: "/forges/one", Project: "target"}
			bookkeeping.Activity["sibling/"+key] = relay.CardState{ForgeRoot: "/forges/one", Project: "sibling"}
			bookkeeping.Threads["target-"+key] = "$target"
			bookkeeping.Threads["sibling-"+key] = "$sibling"
			bookkeeping.ScopeProject(relay.ScopeThreads, "target-"+key, "/forges/one", "target")
			bookkeeping.ScopeProject(relay.ScopeThreads, "sibling-"+key, "/forges/one", "sibling")
		}
		purgeProjectRelay(&bookkeeping, "/forges/one", "target", "!target:example.org", "!activity:example.org")
		for key := range bookkeeping.Waiting {
			if strings.HasPrefix(key, "!target:example.org\n") {
				return false
			}
		}
		for key := range bookkeeping.Approvals {
			if strings.HasPrefix(key, "target/") {
				return false
			}
		}
		for key := range bookkeeping.Clarifications {
			if strings.HasPrefix(key, "target/") {
				return false
			}
		}
		for key := range bookkeeping.Activity {
			if strings.HasPrefix(key, "target/") {
				return false
			}
		}
		for key, owner := range bookkeeping.ProjectScopes {
			if owner == "target" {
				return false
			}
			_, request, ok := strings.Cut(key, "\n")
			if !ok || bookkeeping.Threads[request] == "" {
				return false
			}
		}
		return true
	}
	if err := quick.Check(property, &quick.Config{MaxCount: 300}); err != nil {
		t.Error(err)
	}
}
