// Package dashboard reads and writes one forge's dashboard chat-request
// queue, the same files the SwarmForge dashboard uses. The queue's mechanics
// live here; the request file format lives in request.go.
package dashboard

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	requestsDir = ".swarmforge/dashboard/requests"
	pendingDir  = "pending"
	doneDir     = "done"
	fileSuffix  = ".request"
)

// Store is the file-backed dashboard request queue of one forge root.
type Store struct {
	root string
	now  func() time.Time
	seq  int
}

// New opens the dashboard request queue of a forge root.
func New(root string) *Store {
	return &Store{root: root, now: time.Now}
}

// Root is the forge root this queue belongs to.
func (s *Store) Root() string {
	return s.root
}

// Requests returns every request the dashboard holds, pending first and each
// group in stable file order.
func (s *Store) Requests() ([]Request, error) {
	var requests []Request
	for _, dir := range []string{pendingDir, doneDir} {
		entries, err := s.readDir(dir)
		if err != nil {
			return nil, err
		}
		requests = append(requests, entries...)
	}
	return requests, nil
}

// Pending returns only the requests the lieutenant has not answered yet.
func (s *Store) Pending() ([]Request, error) {
	return s.readDir(pendingDir)
}

// CreateRequest queues a chat request for the lieutenant and returns its id.
func (s *Store) CreateRequest(body string) (string, error) {
	if strings.TrimSpace(body) == "" {
		return "", errors.New("dashboard: chat request body is empty")
	}
	dir := s.dir(pendingDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	id := s.newID()
	request := Request{
		ID:        id,
		Status:    StatusPending,
		Body:      body,
		CreatedAt: timestamp(s.now()),
	}
	if err := os.WriteFile(filepath.Join(dir, id+fileSuffix), []byte(render(request)), 0o644); err != nil {
		return "", err
	}
	return id, nil
}

// Answer moves a pending request into the done queue with the lieutenant's
// response, the way the dashboard's answer action does.
func (s *Store) Answer(id, response string) error {
	src := filepath.Join(s.dir(pendingDir), id+fileSuffix)
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	request := Parse(string(data))
	request.Status = StatusDone
	request.Response = strings.TrimSpace(response)
	request.UpdatedAt = timestamp(s.now())

	dir := s.dir(doneDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	dst := filepath.Join(dir, id+fileSuffix)
	if err := os.WriteFile(dst, []byte(render(request)), 0o644); err != nil {
		return err
	}
	return os.Remove(src)
}

// RequestForBody answers with the newest chat request reading text that the
// lieutenant still has to answer. An answered request is not open work, so it
// is not the answer to this question.
func (s *Store) RequestForBody(text string) (Request, bool, error) {
	pending, err := s.Pending()
	if err != nil {
		return Request{}, false, err
	}
	for i := len(pending) - 1; i >= 0; i-- {
		if pending[i].Body == text {
			return pending[i], true, nil
		}
	}
	return Request{}, false, nil
}

func (s *Store) dir(kind string) string {
	return filepath.Join(s.root, filepath.FromSlash(requestsDir), kind)
}

func (s *Store) readDir(kind string) ([]Request, error) {
	entries, err := os.ReadDir(s.dir(kind))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var requests []Request
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), fileSuffix) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir(kind), entry.Name()))
		if err != nil {
			return nil, err
		}
		request := Parse(string(data))
		if request.ID == "" {
			request.ID = strings.TrimSuffix(entry.Name(), fileSuffix)
		}
		requests = append(requests, request)
	}
	sort.SliceStable(requests, func(i, j int) bool { return requests[i].ID < requests[j].ID })
	return requests, nil
}

// newID names a new request file: the moment it was queued, and a counter when
// the same moment names a request that already exists.
func (s *Store) newID() string {
	for {
		s.seq++
		id := "req-" + compactTimestamp(s.now())
		if s.seq > 1 {
			id = fmt.Sprintf("%s-%d", id, s.seq)
		}
		if _, err := os.Stat(filepath.Join(s.dir(pendingDir), id+fileSuffix)); os.IsNotExist(err) {
			return id
		}
	}
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-21T23:27:31+02:00","module_hash":"2fbbae35cc897a19c01f97dc8612d229af196de9f86c71079dabedb69112f17e","functions":[{"id":"func/New","name":"New","line":31,"end_line":33,"hash":"dfc5d4ae80c4a49694b7d6b824c51a6415279d23ebba7a22172ea7455c83821c"},{"id":"func/Store.Root","name":"Store.Root","line":36,"end_line":38,"hash":"7f73a9a5e6d2aef8766d17d51b1ae034bbbace14b9e07921512147c967f9ae82"},{"id":"func/Store.Requests","name":"Store.Requests","line":42,"end_line":52,"hash":"f0dee75203751b9ef42bf014efb6ac367fa0a08ff4d15f559d855756093f4809"},{"id":"func/Store.Pending","name":"Store.Pending","line":55,"end_line":57,"hash":"15579a38c43b18395b257a2568a558794a911faa29cf8dfb899f4b8d4c17d2e0"},{"id":"func/Store.CreateRequest","name":"Store.CreateRequest","line":60,"end_line":80,"hash":"688514cea0e3e87270f3e3c9ef6dc54c845e25d190426f799a602314751249c9"},{"id":"func/Store.Answer","name":"Store.Answer","line":84,"end_line":104,"hash":"422d2b7bc960a49d2cb458380e9aa16e05fc224c977ad22af7ad75404565aaed"},{"id":"func/Store.RequestForBody","name":"Store.RequestForBody","line":109,"end_line":120,"hash":"7714ad755dd1a9fa343ef3ea2a21ff622fe1c3a42d5120213a85337be0bcf97a"},{"id":"func/Store.dir","name":"Store.dir","line":122,"end_line":124,"hash":"4a673b1069f098acb177a7c214e0e6209dfd4c50ec8aa39d24cfcb2feae984b3"},{"id":"func/Store.readDir","name":"Store.readDir","line":126,"end_line":152,"hash":"c572aa2a73faac0ad18d9cbe424956ea0340c3e26d44b62e2ff30cdfa20171d3"},{"id":"func/Store.newID","name":"Store.newID","line":156,"end_line":167,"hash":"3b5dad5f971c2dc61f0674404ff4ed0274cb1ccf5ba20b8eb1657464cee22020"}]}
// mutate4go-manifest-end
