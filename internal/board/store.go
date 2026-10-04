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
	// RolesFile is the lane list inside a project: one lane per role.
	RolesFile = ".swarmforge/roles.tsv"
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
		name, lane, ok := ParseRow(line)
		if !ok {
			continue
		}
		cards = append(cards, Card{Project: project, Name: name, Lane: lane})
	}
	return cards, nil
}

// LanesFor lists the lanes one project runs: the roles its board moves cards
// between, in the order the project names them.
func (s *Store) LanesFor(project string) ([]string, error) {
	path := filepath.Join(forge.ProjectDir(s.root, project), filepath.FromSlash(RolesFile))
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var lanes []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if name := strings.TrimSpace(strings.Split(line, "\t")[0]); name != "" {
			lanes = append(lanes, name)
		}
	}
	return lanes, nil
}

// ProjectBoard is one project's board: the lanes it runs and the cards each
// lane holds.
type ProjectBoard struct {
	Project string
	Lanes   []string
	Cards   []Card
}

// Boards lists the board of every open project, each with its lanes, in a
// stable order so a tick reads the same board the same way.
func (s *Store) Boards() ([]ProjectBoard, error) {
	projects, err := forge.OpenProjects(s.root)
	if err != nil {
		return nil, err
	}
	sort.Strings(projects)
	boards := make([]ProjectBoard, 0, len(projects))
	for _, project := range projects {
		lanes, err := s.LanesFor(project)
		if err != nil {
			return nil, err
		}
		cards, err := s.CardsFor(project)
		if err != nil {
			return nil, err
		}
		sort.Slice(cards, func(i, j int) bool { return cards[i].Name < cards[j].Name })
		boards = append(boards, ProjectBoard{Project: project, Lanes: lanes, Cards: cards})
	}
	return boards, nil
}

// ParseRow reads one board row: the card's name and the lane it is in. It is
// the way every reader of a board file, in this process or in the tools that
// write one, gets at a row.
func ParseRow(line string) (name, lane string, ok bool) {
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

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-10-04T12:12:50+02:00","module_hash":"cf6472d51f7ac75f095bc31580021853342f26344cbc68bf460420c7a88ccf96","functions":[{"id":"func/Card.Done","name":"Card.Done","line":31,"end_line":33,"hash":"d9d898c4e0320b2ab713f20c042cc0a9d749de73f21e943912c6bfa886b86517"},{"id":"func/New","name":"New","line":41,"end_line":43,"hash":"cfddd3fc0e14beafd02427cbc56e1a56001d902c02315aab35474bef7f5f0cee"},{"id":"func/Store.Cards","name":"Store.Cards","line":46,"end_line":66,"hash":"e1250ab1af6cd12b2b3f8bb4aa5214047588b9879cb4453536a797f810ac5731"},{"id":"func/Store.CardsFor","name":"Store.CardsFor","line":69,"end_line":87,"hash":"d0ce9742acfc27a3608a9109ebbca7724103372f87d8028a2fd5d17900b3fe03"},{"id":"func/Store.LanesFor","name":"Store.LanesFor","line":91,"end_line":110,"hash":"fe7568c1bbbfee4bd75d47847f91e530059f039f85fa3394ef313521d673915a"},{"id":"func/Store.Boards","name":"Store.Boards","line":122,"end_line":142,"hash":"966f02370805455f1a0630a83b3b7abe2b69027386696be3ab6f3afed6e15ca6"},{"id":"func/ParseRow","name":"ParseRow","line":147,"end_line":158,"hash":"a162108e8c6e9bc4297edfdd22822ab390ba893d907024f3851253f9b0cbcf13"}]}
// mutate4go-manifest-end
