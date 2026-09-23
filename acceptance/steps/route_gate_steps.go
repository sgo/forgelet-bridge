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
	return w.answerProposal(ctx, captures[1], captures[2])
}

// operatorAnswersAgain answers the same proposal once more, in the same words:
// one approval is single-use.
func operatorAnswersAgain(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	if w.gateWords == "" {
		return fmt.Errorf("no earlier answer to the proposal for %s", captures[1])
	}
	ctx, cancel := stepContext()
	defer cancel()
	return w.answerProposal(ctx, captures[1], w.gateWords)
}

// answerProposal commits the gate's proposal with the operator's words,
// remembering what they said so the same words can be tried again.
func (w *World) answerProposal(ctx context.Context, card, words string) error {
	root, err := w.theForgeRoot()
	if err != nil {
		return err
	}
	out, err := w.runGate(ctx, root, "commit", w.gateProposal, "--operator-said", words)
	w.gateErr = err
	w.gateWords = words
	if err != nil && !strings.Contains(out, "already used") {
		return fmt.Errorf("the route gate could not commit %s: %v\n%s", card, err, out)
	}
	return nil
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
