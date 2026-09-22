// Package approvals reads and writes the approvals a forge's projects are
// waiting for: the same pending handoffs and decisions the desktop dashboard
// works with.
package approvals

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	// os.ReadDir lists a directory by file name, and an approval's id is the
	// name of its file, so the approvals already come back in the order the
	// operator's room should show them.
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

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-22T15:49:45+02:00","module_hash":"bdac9d216d65196e13bb3c2a9a8d5a5184d799b109199b1f63ea66121e8f45e4","functions":[{"id":"func/New","name":"New","line":47,"end_line":49,"hash":"dfc5d4ae80c4a49694b7d6b824c51a6415279d23ebba7a22172ea7455c83821c"},{"id":"func/Store.Projects","name":"Store.Projects","line":53,"end_line":68,"hash":"fa4aaf87b6aa3cc91d6479e33aca17c25c246dd2756ad982b178f396e610533c"},{"id":"func/Store.Pending","name":"Store.Pending","line":71,"end_line":85,"hash":"04033c28ccb8f986eab6b7f55ebd0b5e99eb403c0fab43a2775e3776e2532a9b"},{"id":"func/Store.PendingFor","name":"Store.PendingFor","line":88,"end_line":113,"hash":"b2c3c9aa4f2449a53ac2c4de85f5e8d9c32b8b6aee8c348e303f3841b1faa3c1"},{"id":"func/Store.Approve","name":"Store.Approve","line":118,"end_line":139,"hash":"2468d08a40693a5af5c6e677e98bfe2c63148f01d2bc54d36e74279266c58003"},{"id":"func/Store.SendBack","name":"Store.SendBack","line":146,"end_line":163,"hash":"b130eae71c35296bf9b8ef9904e9fabd51e8ed5fdc77770eeecdf08702924cc2"},{"id":"func/Store.appendReview","name":"Store.appendReview","line":167,"end_line":193,"hash":"588dc780b0de9c54184600fd83ad57a2f09362e73a6e32a3b0195774f3b9507b"},{"id":"func/Store.pendingFile","name":"Store.pendingFile","line":200,"end_line":209,"hash":"3140ff0f0cb1ebe7f3c3fc6dbe95f183fcc9e5ae7633860805d5da2bfa2fb115"},{"id":"func/Store.reviewsFile","name":"Store.reviewsFile","line":211,"end_line":213,"hash":"00ce01befea2bade36633333f87ffaab828cf32fe9ee59169287fc38a1224a14"},{"id":"func/Store.projectDir","name":"Store.projectDir","line":215,"end_line":217,"hash":"7217b718448acaf76f3bd1aa6ead2327d9fba56fe416841ad5558f80d72e9cbd"},{"id":"func/parse","name":"parse","line":220,"end_line":253,"hash":"bdeede12cd4c4ee4755b40cc60f1661d1efa6270463ddc414e6f0cf1346c5634"},{"id":"func/approved","name":"approved","line":256,"end_line":264,"hash":"c493307c1b1c260f36ffb022e5941fc7abb5bc9613d4c6818aae2ef7c863a989"},{"id":"func/firstOf","name":"firstOf","line":266,"end_line":272,"hash":"f17289c996f9ec61c1d22d2c614d4e387873663c1bc8ed33bb0be4967b67c425"},{"id":"func/commaList","name":"commaList","line":274,"end_line":282,"hash":"d6788f35ca14f91bc2c1003da431ca81f0083e31569c58847793c6d4047d49ae"}]}
// mutate4go-manifest-end
