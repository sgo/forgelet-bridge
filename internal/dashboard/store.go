// Package dashboard reads and writes one forge's dashboard chat-request
// queue, the same files the SwarmForge dashboard uses.
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

// Statuses a dashboard request can carry.
const (
	StatusPending = "pending"
	StatusDone    = "done"
)

const (
	requestsDir  = ".swarmforge/dashboard/requests"
	pendingDir   = "pending"
	doneDir      = "done"
	fileSuffix   = ".request"
	bodyFallback = ""
)

// Request is one chat request in a forge's dashboard.
type Request struct {
	ID        string
	Status    string
	Role      string
	Body      string
	Response  string
	CreatedAt string
	UpdatedAt string
}

// Done reports whether the lieutenant has answered this request.
func (r Request) Done() bool {
	return r.Status == StatusDone
}

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

// Pending returns only the unanswered requests.
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

// RequestForBody answers with the newest request whose body matches text.
func (s *Store) RequestForBody(text string) (Request, bool, error) {
	requests, err := s.Requests()
	if err != nil {
		return Request{}, false, err
	}
	for i := len(requests) - 1; i >= 0; i-- {
		if requests[i].Body == text {
			return requests[i], true, nil
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

// render writes a request in the dashboard's on-disk format.
func render(r Request) string {
	var b strings.Builder
	b.WriteString("id: " + r.ID + "\n")
	b.WriteString("status: " + r.Status + "\n")
	if strings.TrimSpace(r.Role) != "" {
		b.WriteString("role: " + r.Role + "\n")
	}
	b.WriteString("created_at: " + r.CreatedAt + "\n")
	if r.UpdatedAt != "" {
		b.WriteString("updated_at: " + r.UpdatedAt + "\n")
	}
	if r.Response != "" {
		b.WriteString("response: " + strings.ReplaceAll(r.Response, "\n", `\n`) + "\n")
	}
	body := r.Body
	if body == "" {
		body = bodyFallback
	}
	b.WriteString("\n" + body)
	if !strings.HasSuffix(body, "\n") {
		b.WriteString("\n")
	}
	return b.String()
}

// Parse reads a request file: header lines, a blank line, then the body. It is
// the way every reader of a dashboard request file, in this process or the
// dashboard's own tools, gets at the request's fields.
func Parse(text string) Request {
	header, body, found := strings.Cut(text, "\n\n")
	if !found {
		header, body = text, ""
	}

	request := Request{}
	// The same fields render writes, so the two stay in step.
	fields := map[string]*string{
		"id":         &request.ID,
		"status":     &request.Status,
		"role":       &request.Role,
		"created_at": &request.CreatedAt,
		"updated_at": &request.UpdatedAt,
		"response":   &request.Response,
	}
	for _, line := range strings.Split(header, "\n") {
		key, value, ok := strings.Cut(line, ": ")
		if !ok {
			continue
		}
		if field, known := fields[key]; known {
			*field = strings.TrimSpace(value)
		}
	}
	request.Response = strings.ReplaceAll(request.Response, `\n`, "\n")
	request.Body = strings.TrimRight(body, "\n")
	return request
}

func timestamp(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.999999999Z")
}

func compactTimestamp(t time.Time) string {
	return t.UTC().Format("20060102T150405.000000000Z")
}
