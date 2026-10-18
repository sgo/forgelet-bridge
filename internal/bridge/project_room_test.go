package bridge

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/unclebob/forgelet-bridge/internal/config"
	"github.com/unclebob/forgelet-bridge/internal/relay"
)

type fakeProjectSource struct {
	projects []Project
	err      error
}

func (f fakeProjectSource) Projects(string) ([]Project, error) {
	return f.projects, f.err
}

func projectBridge(t *testing.T, cfg config.Config, rooms *fakeRooms, source ProjectSource) *Bridge {
	t.Helper()
	root := cfg.Forges[0].Root
	built, err := New(cfg, rooms,
		map[string]ForgeStore{root: &fakeStore{}},
		map[string]ApprovalStore{root: &fakeApprovals{}},
		map[string]ClarificationStore{root: &fakeClarifications{}},
		map[string]BoardStore{root: &fakeBoard{}},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	built.SetProjectSource(source)
	return built
}

func TestTickProvisionsAProjectRoomInsideItsForgeSpaceAndReusesIt(t *testing.T) {
	cfg := newTestConfig(t, "/forges/forge-a")
	project := Project{
		Name:           "agent",
		Root:           "/forges/forge-a/projects/agent",
		Store:          &fakeStore{},
		Clarifications: &fakeClarifications{},
	}
	rooms := &fakeRooms{}
	built := projectBridge(t, cfg, rooms, fakeProjectSource{projects: []Project{project}})

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	ensured := rooms.ensuredProjectRooms()
	if len(ensured) != 1 {
		t.Fatalf("ensured project rooms = %+v, want one", ensured)
	}
	if ensured[0].spaceID != "!space-forge-a" ||
		ensured[0].forgeName != "forge-a" ||
		ensured[0].projectName != "agent" ||
		ensured[0].roomID != "!project-agent-forge-a" {
		t.Errorf("project room = %+v, want the forge space and the project's room", ensured[0])
	}
	if _, ok := built.State().ProjectFor("/forges/forge-a", "agent"); !ok {
		t.Error("the project room was not recorded in the bridge state")
	}

	restartedRooms := &fakeRooms{}
	restarted := projectBridge(t, cfg, restartedRooms, fakeProjectSource{projects: []Project{project}})
	if err := restarted.Tick(context.Background()); err != nil {
		t.Fatalf("restarted Tick: %v", err)
	}
	if ensured := restartedRooms.ensuredProjectRooms(); len(ensured) != 0 {
		t.Errorf("restart created another project room: %+v", ensured)
	}
	refreshed := restartedRooms.refreshedProjectRooms()
	if len(refreshed) != 1 || refreshed[0].roomID != "!project-agent-forge-a" {
		t.Errorf("restart = %+v, want the remembered project room refreshed", refreshed)
	}
}

func TestProjectRoomCarriesChatToItsOwnQueue(t *testing.T) {
	cfg := newTestConfig(t, "/forges/forge-a")
	projectStore := &fakeStore{requests: []relay.Request{
		{ID: "req-1", Body: "is the build green?"},
	}}
	project := Project{
		Name:           "agent",
		Root:           "/forges/forge-a/projects/agent",
		Store:          projectStore,
		Clarifications: &fakeClarifications{},
	}
	rooms := &fakeRooms{}
	built := projectBridge(t, cfg, rooms, fakeProjectSource{projects: []Project{project}})

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	sent := rooms.sentMessages()
	if len(sent) != 1 || sent[0].roomID != "!project-agent-forge-a" ||
		sent[0].body != "is the build green?" {
		t.Fatalf("sent = %+v, want the project's request in its own room", sent)
	}

	rooms.push(relay.RoomEvent{
		RoomID:  "!project-agent-forge-a",
		EventID: "$operator",
		Sender:  operator,
		Body:    "yes, ship it",
	})
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick: %v", err)
	}
	if created := projectStore.createdBodies(); len(created) != 1 || created[0] != "yes, ship it" {
		t.Fatalf("created = %v, want the operator's message on the project queue", created)
	}
}

func TestProjectRoomCarriesClarifications(t *testing.T) {
	cfg := newTestConfig(t, "/forges/forge-a")
	clarifications := &fakeClarifications{pending: []relay.Clarification{
		clarificationOf("agent", "clar-1", "agent", "which stage is next?"),
	}}
	project := Project{
		Name:           "agent",
		Root:           "/forges/forge-a/projects/agent",
		Store:          &fakeStore{},
		Clarifications: clarifications,
	}
	rooms := &fakeRooms{}
	built := projectBridge(t, cfg, rooms, fakeProjectSource{projects: []Project{project}})

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	message := lastMessageIn(rooms, "!project-agent-forge-a")
	if message == nil {
		t.Fatalf("sent = %+v, want the project clarification in its room", rooms.sentMessages())
	}

	rooms.push(relay.RoomEvent{
		EventID:    "$reply",
		RoomID:     "!project-agent-forge-a",
		Sender:     operator,
		Body:       "clean is next",
		ThreadRoot: "$event-" + clarificationMessage(clarificationOf("agent", "clar-1", "agent", "which stage is next?")),
	})
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick: %v", err)
	}
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("third Tick: %v", err)
	}
	answers := clarifications.answered()
	if len(answers) != 1 || answers[0].project != "agent" ||
		answers[0].id != "clar-1" || answers[0].text != "clean is next" {
		t.Fatalf("answers = %+v, want the operator's reply on the project queue", answers)
	}
}

func TestProjectSourceErrorNamesTheForgeWithoutStoppingOtherWork(t *testing.T) {
	cfg := newTestConfig(t, "/forges/forge-a")
	rooms := &fakeRooms{}
	built := projectBridge(t, cfg, rooms, fakeProjectSource{err: context.DeadlineExceeded})

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if len(rooms.ensuredProjectRooms()) != 0 {
		t.Error("a failed project discovery still provisioned a room")
	}
	if built.unhappy["/forges/forge-a"] == "" {
		t.Error("a failed project discovery was not reported against its forge")
	}
}
