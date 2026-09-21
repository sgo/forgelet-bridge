package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/unclebob/forgelet-bridge/internal/config"
	"github.com/unclebob/forgelet-bridge/internal/relay"
)

type fakeStore struct {
	mu       sync.Mutex
	requests []relay.Request
	created  []string
}

func (s *fakeStore) Requests() ([]relay.Request, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]relay.Request(nil), s.requests...), nil
}

func (s *fakeStore) CreateRequest(body string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.created = append(s.created, body)
	id := fmt.Sprintf("req-%d", len(s.created))
	s.requests = append(s.requests, relay.Request{ID: id, Body: body})
	return id, nil
}

func (s *fakeStore) createdBodies() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.created...)
}

type sentMessage struct {
	roomID string
	body   string
	anchor string
}

type fakeRooms struct {
	mu       sync.Mutex
	ensured  []string
	sent     []sentMessage
	events   []relay.RoomEvent
	drainErr error
}

func (r *fakeRooms) EnsureForge(_ context.Context, forgeName, _ string) (Room, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensured = append(r.ensured, forgeName)
	return Room{SpaceID: "!space-" + forgeName, RoomID: "!room-" + forgeName}, nil
}

func (r *fakeRooms) SendText(_ context.Context, roomID, body, threadAnchor string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, sentMessage{roomID: roomID, body: body, anchor: threadAnchor})
	return "$event-" + body, nil
}

func (r *fakeRooms) DrainEvents(_ context.Context) ([]relay.RoomEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.drainErr != nil {
		return nil, r.drainErr
	}
	events := r.events
	r.events = nil
	return events, nil
}

func (r *fakeRooms) sentMessages() []sentMessage {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]sentMessage(nil), r.sent...)
}

func (r *fakeRooms) push(event relay.RoomEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

const operator = "@operator:example.org"

func newTestBridge(t *testing.T, rooms *fakeRooms, stores map[string]ForgeStore, roots ...string) (*Bridge, config.Config) {
	t.Helper()
	cfg := config.Config{
		HomeserverURL: "http://127.0.0.1:8008",
		UserID:        "@bridge:example.org",
		AccessToken:   "token",
		Operator:      operator,
		ForgeRoots:    roots,
		StateDir:      t.TempDir(),
	}
	built, err := New(cfg, rooms, stores, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return built, cfg
}

func TestTickPostsForgeRequest(t *testing.T) {
	store := &fakeStore{requests: []relay.Request{{ID: "req-1", Body: "is the build green?"}}}
	rooms := &fakeRooms{}
	built, _ := newTestBridge(t, rooms, map[string]ForgeStore{"/forges/forge-a": store}, "/forges/forge-a")

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	sent := rooms.sentMessages()
	if len(sent) != 1 || sent[0].body != "is the build green?" || sent[0].anchor != "" {
		t.Fatalf("sent = %+v, want one unthreaded chat message", sent)
	}
	if sent[0].roomID != "!room-forge-a" {
		t.Errorf("room = %q, want the forge's chat room", sent[0].roomID)
	}
	if anchor, ok := built.State().Relay.Anchor("req-1"); !ok || anchor != "$event-is the build green?" {
		t.Errorf("anchor = %q, %v", anchor, ok)
	}
}

func TestTickThreadsTheLieutenantsAnswer(t *testing.T) {
	store := &fakeStore{requests: []relay.Request{
		{ID: "req-1", Body: "is the build green?", Response: "yes, the build is green"},
	}}
	rooms := &fakeRooms{}
	built, _ := newTestBridge(t, rooms, map[string]ForgeStore{"/forges/forge-a": store}, "/forges/forge-a")

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	sent := rooms.sentMessages()
	if len(sent) != 2 {
		t.Fatalf("sent = %+v, want a chat message and its thread reply", sent)
	}
	if sent[1].body != "yes, the build is green" {
		t.Errorf("reply = %q", sent[1].body)
	}
	if sent[1].anchor != "$event-is the build green?" {
		t.Errorf("reply anchor = %q, want the posted request message", sent[1].anchor)
	}
}

func TestTickQueuesOperatorMessageAsChatRequest(t *testing.T) {
	store := &fakeStore{}
	rooms := &fakeRooms{}
	built, _ := newTestBridge(t, rooms, map[string]ForgeStore{"/forges/forge-a": store}, "/forges/forge-a")
	rooms.push(relay.RoomEvent{RoomID: "!room-forge-a", EventID: "$operator-message", Sender: operator, Body: "is the build green?"})

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	if got := store.createdBodies(); len(got) != 1 || got[0] != "is the build green?" {
		t.Fatalf("created = %v, want one chat request", got)
	}
	if requestID := built.State().Relay.Relayed["$operator-message"]; requestID == "" {
		t.Error("operator message was not recorded as relayed")
	}
	if anchor, ok := built.State().Relay.Anchor("req-1"); !ok || anchor != "$operator-message" {
		t.Errorf("anchor = %q, %v, want the operator's own message", anchor, ok)
	}
	if sent := rooms.sentMessages(); len(sent) != 0 {
		t.Errorf("sent = %+v, want no echo of the operator's message", sent)
	}
}

func TestTickIgnoresMessagesFromAnyoneElse(t *testing.T) {
	store := &fakeStore{}
	rooms := &fakeRooms{}
	built, _ := newTestBridge(t, rooms, map[string]ForgeStore{"/forges/forge-a": store}, "/forges/forge-a")
	rooms.push(relay.RoomEvent{RoomID: "!room-forge-a", EventID: "$stranger", Sender: "@stranger:example.org", Body: "let me in"})

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	if got := store.createdBodies(); len(got) != 0 {
		t.Errorf("created = %v, want none for a stranger", got)
	}
}

func TestTickRepeatsNothingWhenRestarted(t *testing.T) {
	store := &fakeStore{requests: []relay.Request{
		{ID: "req-1", Body: "is the build green?", Response: "yes, the build is green"},
	}}
	rooms := &fakeRooms{}
	built, cfg := newTestBridge(t, rooms, map[string]ForgeStore{"/forges/forge-a": store}, "/forges/forge-a")
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("first Tick: %v", err)
	}

	restartedRooms := &fakeRooms{}
	restarted, err := New(cfg, restartedRooms, map[string]ForgeStore{"/forges/forge-a": store}, nil)
	if err != nil {
		t.Fatalf("New after restart: %v", err)
	}
	if err := restarted.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick: %v", err)
	}

	if sent := restartedRooms.sentMessages(); len(sent) != 0 {
		t.Errorf("sent after restart = %+v, want nothing repeated", sent)
	}
	if ensured := restartedRooms.ensured; len(ensured) != 0 {
		t.Errorf("ensured = %v, want the recorded forge reused", ensured)
	}
}

func TestTickProvisionsEveryForgeRootSeparately(t *testing.T) {
	stores := map[string]ForgeStore{
		"/forges/forge-a": &fakeStore{requests: []relay.Request{{ID: "req-1", Body: "one"}}},
		"/forges/forge-b": &fakeStore{requests: []relay.Request{{ID: "req-2", Body: "two"}}},
	}
	rooms := &fakeRooms{}
	built, _ := newTestBridge(t, rooms, stores, "/forges/forge-a", "/forges/forge-b")

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	if len(rooms.ensured) != 2 {
		t.Fatalf("ensured = %v, want one forge per root", rooms.ensured)
	}
	sent := rooms.sentMessages()
	if len(sent) != 2 || sent[0].roomID == sent[1].roomID {
		t.Errorf("sent = %+v, want one message per forge room", sent)
	}
}

func TestTickRecordsIdleStatus(t *testing.T) {
	rooms := &fakeRooms{}
	built, cfg := newTestBridge(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}}, "/forges/forge-a")

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	status := readStatus(t, filepath.Join(cfg.StateDir, StatusName))
	if !status.Idle || status.Tick != 1 {
		t.Errorf("status = %+v, want the first tick to be idle", status)
	}
}

func TestTickReportsBusyWhileItWorks(t *testing.T) {
	store := &fakeStore{requests: []relay.Request{{ID: "req-1", Body: "is the build green?"}}}
	rooms := &fakeRooms{}
	built, cfg := newTestBridge(t, rooms, map[string]ForgeStore{"/forges/forge-a": store}, "/forges/forge-a")

	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	if status := readStatus(t, filepath.Join(cfg.StateDir, StatusName)); status.Idle {
		t.Errorf("status = %+v, want the tick that posted a message to report work", status)
	}
}

func TestTickFailsWithoutADashboardQueue(t *testing.T) {
	rooms := &fakeRooms{}
	built, _ := newTestBridge(t, rooms, map[string]ForgeStore{}, "/forges/forge-a")

	if err := built.Tick(context.Background()); err == nil {
		t.Fatal("Tick succeeded without a dashboard queue")
	}
}

func TestRunKeepsRelayingUntilTheContextEnds(t *testing.T) {
	store := &fakeStore{requests: []relay.Request{{ID: "req-1", Body: "is the build green?"}}}
	rooms := &fakeRooms{}
	built, _ := newTestBridge(t, rooms, map[string]ForgeStore{"/forges/forge-a": store}, "/forges/forge-a")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- built.Run(ctx, time.Millisecond) }()

	waitForSent(t, rooms)
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Errorf("Run = %v, want the cancelled context", err)
	}
}

func TestRunSurvivesAFailingTick(t *testing.T) {
	rooms := &fakeRooms{drainErr: errors.New("the homeserver is away")}
	built, cfg := newTestBridge(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}}, "/forges/forge-a")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- built.Run(ctx, time.Millisecond) }()
	time.Sleep(20 * time.Millisecond)
	cancel()

	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Errorf("Run = %v, want a failing tick to leave the bridge running", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.StateDir, StatusName)); err == nil {
		t.Error("a failed tick wrote a status")
	}
}

func TestRunTicksOnceWithoutWaitingForTheInterval(t *testing.T) {
	store := &fakeStore{requests: []relay.Request{{ID: "req-1", Body: "is the build green?"}}}
	rooms := &fakeRooms{}
	built, _ := newTestBridge(t, rooms, map[string]ForgeStore{"/forges/forge-a": store}, "/forges/forge-a")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- built.Run(ctx, time.Hour) }()

	waitForSent(t, rooms)
	cancel()
	<-done
}

// waitForSent waits for the bridge to post its first chat message.
func waitForSent(t *testing.T, rooms *fakeRooms) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if len(rooms.sentMessages()) > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the bridge never posted a chat message")
		}
		time.Sleep(time.Millisecond)
	}
}

func readStatus(t *testing.T, path string) Status {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	var status Status
	if err := json.Unmarshal(data, &status); err != nil {
		t.Fatalf("parse status: %v", err)
	}
	return status
}
