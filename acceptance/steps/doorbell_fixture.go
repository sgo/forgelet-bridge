package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// projectIsMasteredByTheRole makes one role the master of a project: the role
// the dashboard would type the operator's chat requests into is that role, so
// the pane the doorbell has to ring is the one the scenario serves.
func projectIsMasteredByTheRole(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	project, forge, role := captures[1], captures[2], captures[3]
	root, err := w.forgeRootOf(forge)
	if err != nil {
		return err
	}
	// The forge's own roles file is the one the dashboard's fixture wrote, so
	// the role's pane and tree come from there.
	columns, err := roleRow(root, role)
	if err != nil {
		return err
	}
	for len(columns) < 8 {
		columns = append(columns, "")
	}
	columns[0], columns[1] = role, "master"
	row := strings.Join(columns, "\t") + "\n"
	for _, dir := range []string{root, filepath.Join(root, "projects", project)} {
		if err := writeFile(filepath.Join(dir, ".swarmforge", "roles.tsv"), row); err != nil {
			return err
		}
	}
	return nil
}

// roleRow is one role's own row in a roles file, as the columns it is written
// with.
func roleRow(root, role string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(root, ".swarmforge", "roles.tsv"))
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		columns := strings.Split(line, "\t")
		if len(columns) > 0 && columns[0] == role {
			return columns, nil
		}
	}
	return nil, fmt.Errorf("the forge root %s records no role %s", root, role)
}

// theForgeGivesAWorkingSession gives the role a session that is mid-turn: the
// pane shows the marker codex draws while it works, so a tool that waits for a
// window can see there is none.
func theForgeGivesAWorkingSession(_ context.Context, world any, captures []string) error {
	return forgeGivesSession(world.(*World), captures[1], captures[2], busyPaneCommand)
}

// theForgeGivesASessionThatLosesTheEnter gives the role the terminal the
// dashboard and the doorbell type into, with the Enter a ring sends dropped the
// way a pane still taking the paste drops it: the whole ring stays in the
// composer, and no session has seen it.
func theForgeGivesASessionThatLosesTheEnter(_ context.Context, world any, captures []string) error {
	return forgeGivesSession(world.(*World), captures[1], captures[2], composerPaneCommand+" --lose-enter")
}

// theDashboardTypedTheRequest types a pending request into the master role's
// pane the way the dashboard does - the id in brackets, then the words - which
// is the delivery evidence a later pass looks for. The fixture's dashboard
// writes its typing into a stub, so the typing that reached the real pane is
// made here.
func theDashboardTypedTheRequest(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	body := captures[1]
	root, request, err := w.requestByBody(body)
	if err != nil {
		return err
	}
	return w.typeIntoMasterPane(root, request.ID, body)
}

// theForgeHoldsTheApprovalTheBridgeWrote puts the chat request the bridge writes
// for an approval the operator forwarded into the forge's dashboard queue: the
// bridge writes the notification's first line, so the ring can tell what it is.
func theForgeHoldsTheApprovalTheBridgeWrote(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	return w.dashboardTakesRequest(approvalMessageStart(captures[1], captures[2]))
}

// theForgeHoldsTheClarificationTheBridgeWrote puts the chat request the bridge
// writes for a clarification an agent is blocked on into the same queue.
func theForgeHoldsTheClarificationTheBridgeWrote(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	return w.dashboardTakesRequest(clarificationMessageStart(captures[1]) + captures[2])
}

// theDashboardTypedTheRequestLongAgo types a request into the pane and then
// scrolls it out of sight, the way a request the role answered hours ago has
// left the visible screen while the pane's scrollback still proves it arrived.
func theDashboardTypedTheRequestLongAgo(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	body := captures[1]
	root, request, err := w.requestByBody(body)
	if err != nil {
		return err
	}
	if err := w.typeIntoMasterPane(root, request.ID, body); err != nil {
		return err
	}
	pane, socket, err := w.masterPane(root)
	if err != nil {
		return err
	}
	// A pane's screen is a few dozen lines; enough turns push what was typed up
	// into the history the pane keeps. A terminal takes a turn when its composer
	// holds something, so the fixture fills it the way a session that answered
	// several times would have.
	for line := 0; line < screenFullOfLines; line++ {
		if _, err := tmux(socket, "send-keys", "-t", pane, "-l", fillerTurn); err != nil {
			return err
		}
		if _, err := tmux(socket, "send-keys", "-t", pane, "C-m"); err != nil {
			return err
		}
	}
	// The terminal draws a turn as it takes it, so the screen is filled as the
	// pane catches up with what was typed: the request has scrolled off it once
	// the pane has drawn the turns behind it.
	ctx, cancel := stepContext()
	defer cancel()
	return waitFor(ctx, fmt.Sprintf("the request the dashboard typed %s is still on the pane's screen", request.ID), func() (bool, error) {
		text, err := paneText(socket, pane)
		if err != nil {
			return false, err
		}
		return !strings.Contains(text, "["+request.ID+"]"), nil
	})
}

// screenFullOfLines is how many blank lines the fixture sends to push what was
// typed off a pane's visible screen.
const screenFullOfLines = 60

// fillerTurn is what the fixture types to fill a pane's screen: a session that
// answered several times leaves turns behind it, and a turn is what pushes what
// came before it off the screen.
const fillerTurn = "a turn the pane took"

// requestByBody is the pending delivery request that reads body, and the forge
// whose queue is holding it: a scenario that serves several forges names the
// request, not the forge it belongs to.
func (w *World) requestByBody(body string) (root string, request deliveryRequest, err error) {
	roots := append(append([]string{}, w.configured...), w.forgeRoots...)
	for _, candidate := range roots {
		store := w.dashboards[filepath.Base(candidate)]
		if store == nil {
			continue
		}
		pending, found, err := store.RequestForBody(body)
		if err != nil {
			return "", deliveryRequest{}, err
		}
		if found {
			return candidate, deliveryRequest{ID: pending.ID}, nil
		}
	}
	return "", deliveryRequest{}, fmt.Errorf("no fixture forge root holds a pending chat request reading %q", body)
}

// deliveryRequest is the little of a dashboard request the doorbell needs.
type deliveryRequest struct {
	ID string
}

// longAgo is when the fixture says an earlier ring or an earlier request
// happened: far past any gap a doorbell might keep, so a request it wrote is
// due to be rung rather than merely recent.
var longAgo = func() time.Time { return time.Now().UTC().Add(-24 * time.Hour) }

// theDoorbellHasRungTheRequest writes the state a previous pass left: the
// doorbell has rung this request this many times, the last of them long ago,
// and nobody has answered it. The ledger is the tool's own record, so the
// fixture writes it in the shape the tool reads.
func theDoorbellHasRungTheRequest(_ context.Context, world any, captures []string) error {
	return world.(*World).writeLedgerOfRings(captures[1], 1)
}

// theRequestWasRungItsFill writes a ledger that has rung one request more times
// than any fill allows, which is a request the doorbell has already tried its
// last ring on.
func theRequestWasRungItsFill(_ context.Context, world any, captures []string) error {
	return world.(*World).writeLedgerOfRings(captures[1], rungItsFill)
}

// rungItsFill is more rings than a doorbell keeps: the fixture names a number
// past any fill rather than the fill itself, so a tool that changed its fill is
// still asked the question the scenario asks.
const rungItsFill = 99

// writeLedgerOfRings leaves the forge a ledger saying the doorbell rang one
// request this many times, the last of them a day ago.
func (w *World) writeLedgerOfRings(body string, times int) error {
	root, request, err := w.requestByBody(body)
	if err != nil {
		return err
	}
	at := longAgo().Format(time.RFC3339Nano)
	ledger := fmt.Sprintf(`{:delivered [] :owed [] :rings {"%s" {:count %d :at "%s"}} :rung ["%s"]}`,
		request.ID, times, at, request.ID)
	return writeFile(filepath.Join(root, ".swarmforge", "doorbell.edn"), ledger)
}

// theRequestWaitedPastTheGap moves the request's own record back: the dashboard
// wrote down when it created it, and a request that has waited unanswered past
// the doorbell's gap is one whose delivery is a day old. It leaves the doorbell's
// own ledger alone, so a scenario can say what the pass already knows about the
// request rather than the shape saying it for them.
func theRequestWaitedPastTheGap(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, request, err := w.requestByBody(captures[1])
	if err != nil {
		return err
	}
	return ageRequest(root, request.ID, longAgo())
}

// ageRequest moves the moment a request's own record says it was created, which
// is when the dashboard typed it into the pane.
func ageRequest(root, id string, at time.Time) error {
	path := filepath.Join(root, ".swarmforge", "dashboard", "requests", "pending", id+".request")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var lines []string
	replaced := false
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if strings.HasPrefix(line, "created_at: ") {
			line = "created_at: " + at.UTC().Format(time.RFC3339Nano)
			replaced = true
		}
		lines = append(lines, line)
	}
	if !replaced {
		return fmt.Errorf("the request %s carries no created_at to move: %s", id, path)
	}
	return writeFile(path, strings.Join(lines, "\n")+"\n")
}

// theDashboardAnsweredTheRequest answers a chat request the way the lieutenant
// does, which takes it out of the dashboard's pending queue.
func theDashboardAnsweredTheRequest(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, request, err := w.requestByBody(captures[1])
	if err != nil {
		return err
	}
	store := w.dashboards[filepath.Base(root)]
	if store == nil {
		return fmt.Errorf("the fixture forge root %s has no dashboard queue", root)
	}
	return store.Answer(request.ID, "yes, it is green")
}

// masterPane is the pane the doorbell rings and the socket it lives on: the
// master row of the forge root, where the dashboard types the operator's own
// messages.
func (w *World) masterPane(root string) (pane, socket string, err error) {
	data, err := os.ReadFile(filepath.Join(root, ".swarmforge", "roles.tsv"))
	if err != nil {
		return "", "", err
	}
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		columns := strings.Split(line, "\t")
		if len(columns) >= 4 && columns[1] == "master" {
			pane = columns[3]
		}
	}
	if pane == "" {
		return "", "", fmt.Errorf("the forge root %s records no master role", root)
	}
	socket, err = w.projectSocket(filepath.Join(root, "projects", approvalProject))
	if err != nil {
		return "", "", err
	}
	return pane, socket, nil
}

// typeIntoMasterPane types a request into the master role's pane the way the
// dashboard does: the id in brackets, the words, then the two line feeds the
// dashboard sends.
func (w *World) typeIntoMasterPane(root, id, body string) error {
	pane, socket, err := w.masterPane(root)
	if err != nil {
		return err
	}
	wake := fmt.Sprintf("[%s] %s", id, body)
	if strings.Contains(body, "\n") {
		wake = fmt.Sprintf("[%s]\n%s", id, body)
	}
	for _, keys := range [][]string{{"-l", wake}, {"C-m"}, {"C-j"}} {
		if _, err := tmux(socket, append([]string{"send-keys", "-t", pane}, keys...)...); err != nil {
			return err
		}
	}
	return nil
}

// readDoorbellLedger reads the doorbell's own record of what it has seen and
// what it has rung.
func readDoorbellLedger(root string) (map[string][]string, error) {
	path := filepath.Join(root, ".swarmforge", "doorbell.edn")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("the doorbell kept no ledger: %w", err)
	}
	ledger := map[string][]string{}
	for _, key := range []string{"delivered", "rung", "owed"} {
		ledger[key] = ledgerIDs(string(data), ":"+key)
	}
	return ledger, nil
}

// ledgerIDs reads the ids out of one vector of a written ledger, which prints
// the whole map on one line.
func ledgerIDs(text, key string) []string {
	start := strings.Index(text, key+" [")
	if start < 0 {
		return nil
	}
	rest := text[start+len(key)+2:]
	end := strings.Index(rest, "]")
	if end < 0 {
		return nil
	}
	var ids []string
	for _, field := range strings.Fields(rest[:end]) {
		if id := strings.Trim(field, "\""); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}
