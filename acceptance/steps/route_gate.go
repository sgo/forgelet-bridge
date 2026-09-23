package steps

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/internal/board"
)

// routeGateCommand is this repository's route gate: the tool the suite runs,
// and the tool a forge's own copy is a deployment of.
const routeGateCommand = "route_card.sh"

// theForgeRoot is the one fixture forge root the scenario is working with.
func (w *World) theForgeRoot() (string, error) {
	if len(w.forgeRoots) != 1 {
		return "", fmt.Errorf("the scenario works with %d fixture forge roots, want exactly one here", len(w.forgeRoots))
	}
	return w.forgeRoots[0], nil
}

// runGate runs the route gate from this repository's own copy, the way the
// forge's lieutenant does.
func (w *World) runGate(ctx context.Context, root string, args ...string) (string, error) {
	command := exec.CommandContext(ctx,
		filepath.Join(fixtures.ProjectRoot(), "swarmforge", "scripts", routeGateCommand),
		append(args, "--forge-root", root)...)
	command.Dir = root
	out, err := command.CombinedOutput()
	w.gateOutput = string(out)
	return w.gateOutput, err
}

// forgeHoldsProject gives a fixture forge root a project of its own: the roles
// it serves, and the board a card would land on.
func forgeHoldsProject(ctx context.Context, world any, captures []string) error {
	w := world.(*World)
	store, err := w.forge(ctx, captures[1])
	if err != nil {
		return err
	}
	return setUpProject(store.Root(), captures[2])
}

// setUpProject gives a fixture forge root a project of its own: the roles it
// serves with their panes, the board a card would land on, and the forge's own
// scripts beside it.
func setUpProject(root, project string) error {
	dir := filepath.Join(root, "projects", project)
	for _, where := range []string{root, dir} {
		if err := writeFile(filepath.Join(where, ".swarmforge", "roles.tsv"), strings.Join([]string{
			"master\tmaster\t" + where + "\tfixture-master\tMaster\tcodex\ttask\tforward-only",
			"coder\tcoder\t" + filepath.Join(where, "worktrees", "coder") + "\tfixture-coder\tCoder\tcodex\ttask\tforward-only",
		}, "\n")+"\n"); err != nil {
			return err
		}
		if err := writeFile(filepath.Join(where, ".swarmforge", "board", "tasks.tsv"), ""); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, "swarmforge"), 0o755); err != nil {
		return err
	}
	return markProjectOpen(root)
}

// gateHasProposal records a proposal through the tool.
func gateHasProposal(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	root, err := w.theForgeRoot()
	if err != nil {
		return err
	}
	card, project := captures[1], captures[2]
	note := filepath.Join(w.workDir, "gate-note-"+card+".md")
	if err := writeFile(note, "The operator asked for the "+card+" card.\n"); err != nil {
		return err
	}
	out, err := w.runGate(ctx, root, "propose", filepath.Join(root, "projects", project), card, note)
	if err != nil {
		return fmt.Errorf("the route gate could not propose %s: %v\n%s", card, err, out)
	}
	first, _, _ := strings.Cut(strings.TrimSpace(out), "\n")
	id := strings.TrimSpace(strings.TrimPrefix(first, "PROPOSED: "))
	if id == "" || strings.Contains(id, " ") {
		id, _, _ = strings.Cut(id, " ")
	}
	if id == "" {
		return fmt.Errorf("the route gate named no proposal:\n%s", out)
	}
	w.gateProposal = id
	return nil
}

// operatorAnswersProposal commits a proposal with the operator's own words.
func operatorAnswersProposal(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	root, err := w.theForgeRoot()
	if err != nil {
		return err
	}
	out, err := w.runGate(ctx, root, "commit", w.gateProposal, "--operator-said", captures[2])
	w.gateErr = err
	w.gateWords = captures[2]
	if err != nil && !strings.Contains(out, "already used") {
		return fmt.Errorf("the route gate could not commit %s: %v\n%s", captures[1], err, out)
	}
	return nil
}

// operatorAnswersAgain answers the same proposal once more, in the same words:
// one approval is single-use.
func operatorAnswersAgain(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	if w.gateWords == "" {
		return fmt.Errorf("no earlier answer to the proposal for %s", captures[1])
	}
	return operatorAnswersProposal(context.Background(), w, []string{"", captures[1], w.gateWords, ""})
}

// boardHasNoCard checks the forge's board holds no such card yet.
func boardHasNoCard(_ context.Context, world any, captures []string) error {
	held, err := fixtureBoardHolds(world.(*World), captures[2], captures[1])
	if err != nil {
		return err
	}
	if held {
		return fmt.Errorf("the board already holds %s", captures[1])
	}
	return nil
}

// boardNowHoldsCard checks the forge's board holds the card.
func boardNowHoldsCard(_ context.Context, world any, captures []string) error {
	held, err := fixtureBoardHolds(world.(*World), captures[2], captures[1])
	if err != nil {
		return err
	}
	if !held {
		return fmt.Errorf("the board does not hold %s", captures[1])
	}
	return nil
}

// boardHoldsExactlyOneCard checks one approval made one card, not two.
func boardHoldsExactlyOneCard(_ context.Context, world any, captures []string) error {
	rows, err := fixtureBoardRows(world.(*World), approvalProject)
	if err != nil {
		return err
	}
	found := 0
	for _, row := range rows {
		if name, _, ok := board.ParseRow(row); ok && name == captures[1] {
			found++
		}
	}
	if found != 1 {
		return fmt.Errorf("the board holds %d cards named %s, want exactly one", found, captures[1])
	}
	return nil
}

// fixtureBoardHolds reports whether a fixture project's board holds a card.
func fixtureBoardHolds(w *World, project, card string) (bool, error) {
	rows, err := fixtureBoardRows(w, project)
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		if name, _, ok := board.ParseRow(row); ok && name == card {
			return true, nil
		}
	}
	return false, nil
}

// fixtureBoardRows reads one fixture project's board, empty when it holds
// nothing yet.
func fixtureBoardRows(w *World, project string) ([]string, error) {
	root, err := w.theForgeRoot()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(root, "projects", project, filepath.FromSlash(board.TasksFile)))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, nil
	}
	return strings.Split(trimmed, "\n"), nil
}

// gateStillHasProposal checks the proposal is waiting, unconsumed.
func gateStillHasProposal(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := w.theForgeRoot()
	if err != nil {
		return err
	}
	path := filepath.Join(root, ".swarmforge", "route-proposals", w.gateProposal+".edn")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("the gate lost the proposal for %s: %v", captures[1], err)
	}
	if strings.Contains(string(data), ":consumed-at") {
		return fmt.Errorf("the proposal for %s was consumed: %s", captures[1], data)
	}
	return nil
}

// gateRefusedTheSecondCard checks the tool refused a proposal it had already
// made a card from.
func gateRefusedTheSecondCard(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	if w.gateErr == nil {
		return fmt.Errorf("the gate made a second card on one approval:\n%s", w.gateOutput)
	}
	if !strings.Contains(w.gateOutput, "already used") {
		return fmt.Errorf("the gate refused for another reason:\n%s", w.gateOutput)
	}
	return nil
}

// forgePromptAsksLess gives a fixture forge a lieutenant prompt whose gate is
// looser than another forge's: the tool must behave the same.
func forgePromptAsksLess(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := w.theForgeRoot()
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(root, "swarmforge", captureRolePrompt(captures[1])),
		"Ask the operator only about fleet-wide or hard-to-reverse actions.\n")
}

// captureRolePrompt is the prompt file one role answers to, in the forge the
// role serves.
func captureRolePrompt(role string) string {
	return filepath.Join("roles", role+".prompt")
}
