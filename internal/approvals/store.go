// Package approvals reads and writes the approvals a forge's projects are
// waiting for: the same pending handoffs and decisions the desktop dashboard
// works with.
package approvals

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/unclebob/forgelet-bridge/internal/forge"
)

const (
	pendingDir    = ".swarmforge/handoffs/pending_approval"
	outboxDir     = ".swarmforge/handoffs/outbox"
	reviewsDir    = ".swarmforge/rejected-tasks"
	handoffSuffix = ".handoff"
	reviewsSuffix = ".reviews.json"

	// specRole is the role the desktop dashboard credits when a handoff does
	// not say which role handed the work over.
	specRole = "spec"
)

// Approval is one handoff waiting for the operator's decision.
type Approval struct {
	Project   string
	ID        string
	Card      string
	TaskID    string
	Gate      string
	Artifacts []string
	File      string
}

// Store is the approvals side of one forge.
type Store struct {
	root string
	now  func() time.Time
}

// New opens the approvals of a forge root.
func New(root string) *Store {
	return &Store{root: root, now: time.Now}
}

// Pending lists the approvals every open project is waiting for.
func (s *Store) Pending() ([]Approval, error) {
	projects, err := forge.OpenProjects(s.root)
	if err != nil {
		return nil, err
	}
	var pending []Approval
	for _, project := range projects {
		approvals, err := s.PendingFor(project)
		if err != nil {
			return nil, err
		}
		pending = append(pending, approvals...)
	}
	return pending, nil
}

// PendingFor lists one project's pending approvals.
func (s *Store) PendingFor(project string) ([]Approval, error) {
	dir := s.projectDir(project, pendingDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var pending []Approval
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), handoffSuffix) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		pending = append(pending, parse(project, path, string(data)))
	}
	sort.Slice(pending, func(i, j int) bool {
		if pending[i].Project != pending[j].Project {
			return pending[i].Project < pending[j].Project
		}
		return pending[i].ID < pending[j].ID
	})
	return pending, nil
}

// Approve approves an approval the way the desktop dashboard does: the pending
// handoff moves to the project's outbox carrying approved: true, and its
// review notes are dropped.
func (s *Store) Approve(project, id string) error {
	source, err := s.pendingFile(project, id)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}

	destination := filepath.Join(s.projectDir(project, outboxDir), filepath.Base(source))
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(destination, []byte(approved(string(data))), 0o644); err != nil {
		return err
	}
	if err := os.Remove(source); err != nil {
		return err
	}
	return os.RemoveAll(s.reviewsFile(project, id))
}

// SendBack records the operator's feedback and hands the card back the way the
// desktop's retry does: the feedback lands in the card's review history, and
// the approval stops being pending. The repository work the desktop also does
// (rewinding to the task's base commit and re-seeding the lane) stays on the
// desktop.
func (s *Store) SendBack(project, id, feedback string) error {
	source, err := s.pendingFile(project, id)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	approval := parse(project, source, string(data))
	if err := s.appendReview(project, approval, feedback); err != nil {
		return err
	}
	if err := os.Remove(source); err != nil {
		return err
	}
	return os.RemoveAll(s.reviewsFile(project, id))
}

// appendReview records the feedback against every file the approval is about,
// in the card's review history, the same store the desktop's comments use.
func (s *Store) appendReview(project string, approval Approval, feedback string) error {
	text := strings.TrimSpace(feedback)
	if text == "" || approval.TaskID == "" {
		return nil
	}
	path := filepath.Join(s.projectDir(project, reviewsDir), approval.TaskID, "reviews.json")
	history := map[string][]review{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &history)
	}
	entry := review{At: s.now().UTC().Format(time.RFC3339Nano), Text: text}
	for _, artifact := range approval.Artifacts {
		history[artifact] = append(history[artifact], entry)
	}
	if len(approval.Artifacts) == 0 {
		history[approval.Card] = append(history[approval.Card], entry)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(history)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

type review struct {
	At   string `json:"at"`
	Text string `json:"text"`
}

func (s *Store) pendingFile(project, id string) (string, error) {
	if strings.TrimSpace(id) == "" {
		return "", fmt.Errorf("approvals: no approval id")
	}
	path := filepath.Join(s.projectDir(project, pendingDir), id+handoffSuffix)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("approvals: unknown approval %s in %s", id, project)
	}
	return path, nil
}

func (s *Store) reviewsFile(project, id string) string {
	return filepath.Join(s.projectDir(project, pendingDir), id+reviewsSuffix)
}

func (s *Store) projectDir(project, dir string) string {
	return filepath.Join(forge.ProjectDir(s.root, project), filepath.FromSlash(dir))
}

// parse reads a pending handoff: headers, a blank line, then the body.
func parse(project, path, content string) Approval {
	header, _, found := strings.Cut(content, "\n\n")
	if !found {
		header = content
	}
	headers := map[string]string{}
	for _, line := range strings.Split(header, "\n") {
		key, value, ok := strings.Cut(line, ": ")
		if !ok {
			continue
		}
		headers[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}

	id := strings.TrimSuffix(filepath.Base(path), handoffSuffix)
	to := firstOf(headers["to"])
	from := headers["role"]
	if from == "" {
		from = specRole
	}
	taskID := headers["task_id"]
	if taskID == "" {
		taskID = headers["task"]
	}
	return Approval{
		Project:   project,
		ID:        id,
		Card:      headers["task"],
		TaskID:    taskID,
		Gate:      from + " → " + to,
		Artifacts: commaList(headers["artifacts"]),
		File:      path,
	}
}

// approved inserts the header the desktop's approve adds.
func approved(content string) string {
	if strings.Contains(content, "\napproved: ") || strings.HasPrefix(content, "approved: ") {
		return content
	}
	if header, body, found := strings.Cut(content, "\n\n"); found {
		return header + "\napproved: true\n\n" + body
	}
	return content + "\napproved: true\n"
}

func firstOf(list string) string {
	items := commaList(list)
	if len(items) == 0 {
		return ""
	}
	return items[0]
}

func commaList(text string) []string {
	var items []string
	for _, item := range strings.Split(text, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}
