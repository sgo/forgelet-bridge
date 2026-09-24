package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	// A pane's screen is a few dozen lines; enough blank lines push what was
	// typed up into the history the pane keeps.
	for line := 0; line < screenFullOfLines; line++ {
		if _, err := tmux(socket, "send-keys", "-t", pane, "C-m"); err != nil {
			return err
		}
	}
	text, err := paneText(socket, pane)
	if err != nil {
		return err
	}
	if strings.Contains(text, "["+request.ID+"]") {
		return fmt.Errorf("the request the dashboard typed is still on the screen, so the scenario cannot tell the two apart:\n%s", text)
	}
	return nil
}

// screenFullOfLines is how many blank lines the fixture sends to push what was
// typed off a pane's visible screen.
const screenFullOfLines = 60

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
	for _, key := range []string{"seen", "rung"} {
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
