// Package board reads the forge's project boards: which cards exist, and which
// lane each one is in.
package board

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/unclebob/forgelet-bridge/internal/forge"
)

const (
	// TasksFile is the board's card list inside a project.
	TasksFile = ".swarmforge/board/tasks.tsv"
	// DoneLane is the lane a finished card sits in.
	DoneLane = "done"
)

// Card is one card on a project's board.
type Card struct {
	Project string
	Name    string
	Lane    string
}

// Done reports whether the card has finished.
func (c Card) Done() bool {
	return c.Lane == DoneLane
}

// Store is the boards of one forge's open projects.
type Store struct {
	root string
}

// New opens a forge's boards.
func New(root string) *Store {
	return &Store{root: root}
}

// Cards lists the cards of every open project.
func (s *Store) Cards() ([]Card, error) {
	projects, err := forge.OpenProjects(s.root)
	if err != nil {
		return nil, err
	}
	var cards []Card
	for _, project := range projects {
		projectCards, err := s.CardsFor(project)
		if err != nil {
			return nil, err
		}
		cards = append(cards, projectCards...)
	}
	sort.Slice(cards, func(i, j int) bool {
		if cards[i].Project != cards[j].Project {
			return cards[i].Project < cards[j].Project
		}
		return cards[i].Name < cards[j].Name
	})
	return cards, nil
}

// CardsFor lists one project's cards.
func (s *Store) CardsFor(project string) ([]Card, error) {
	path := filepath.Join(forge.ProjectDir(s.root, project), filepath.FromSlash(TasksFile))
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cards []Card
	for _, line := range strings.Split(string(data), "\n") {
		name, lane, ok := cardRow(line)
		if !ok {
			continue
		}
		cards = append(cards, Card{Project: project, Name: name, Lane: lane})
	}
	return cards, nil
}

// cardRow reads one board row: name, lane, and the rest of the columns.
func cardRow(line string) (name, lane string, ok bool) {
	columns := strings.Split(line, "\t")
	if len(columns) < 2 {
		return "", "", false
	}
	name = strings.TrimSpace(columns[0])
	lane = strings.TrimSpace(columns[1])
	if name == "" || lane == "" {
		return "", "", false
	}
	return name, lane, true
}
