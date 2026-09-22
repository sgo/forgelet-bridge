package steps

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/internal/dashboard"
)

func dashboardHoldsRequest(_ context.Context, world any, captures []string) error {
	return world.(*World).dashboardTakesRequest(captures[1])
}

// namedDashboardHoldsRequest seeds a chat request in one named forge's
// dashboard, for scenarios that serve more than one forge.
func namedDashboardHoldsRequest(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	store, err := w.declaredForge(captures[1])
	if err != nil {
		return err
	}
	return askDashboard(w, filepath.Base(store.Root()), captures[2])
}

// dashboardTakesRequestStep gives the dashboard a chat request from the suite.
func dashboardTakesRequestStep(_ context.Context, world any, captures []string) error {
	return world.(*World).dashboardTakesRequest(captures[1])
}

// dashboardTakesRequest gives a chat request to the forge's dashboard the way
// its clients do, which is what wakes the lieutenant.
func (w *World) dashboardTakesRequest(text string) error {
	root, err := singleForge(w)
	if err != nil {
		return err
	}
	return askDashboard(w, filepath.Base(root), text)
}

// askDashboard hands a chat request to one fixture forge's dashboard.
func askDashboard(w *World, name, text string) error {
	dashboard, ok := w.running[name]
	if !ok {
		return fmt.Errorf("the fixture forge root %s does not have its dashboard running", name)
	}
	ctx, cancel := stepContext()
	defer cancel()
	return dashboard.Ask(ctx, text)
}

// dashboardWoke waits for the dashboard to have typed a chat request into the
// lieutenant's pane.
func dashboardWoke(_ context.Context, world any, captures []string) error {
	return world.(*World).wokeWithTheRequest(captures[1])
}

// wokeWithTheRequest waits until the forge's dashboard typed the chat request
// into the lieutenant's pane, which is what tells a request the dashboard took
// apart from one the bridge wrote into the queue itself.
func (w *World) wokeWithTheRequest(text string) error {
	ctx, cancel := stepContext()
	defer cancel()
	root, err := singleForge(w)
	if err != nil {
		return err
	}
	dashboard, ok := w.running[filepath.Base(root)]
	if !ok {
		return fmt.Errorf("the fixture forge root %s does not have its dashboard running", root)
	}
	return waitFor(ctx, fmt.Sprintf("the dashboard never typed the chat request %q into the lieutenant's pane", text), func() (bool, error) {
		return dashboard.WokeWith(text)
	})
}

func lieutenantAnswers(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	var request foundRequest
	err := waitFor(ctx, fmt.Sprintf("the forge holds no chat request reading %q", captures[1]), func() (bool, error) {
		found, ok, err := findRequest(w, captures[1])
		if err != nil {
			return false, err
		}
		request = found
		return ok, nil
	})
	if err != nil {
		return err
	}
	return request.store.Answer(request.id, captures[2])
}

type foundRequest struct {
	store *dashboard.Store
	id    string
}

// findRequest is the chat request reading body that some configured forge is
// still waiting for an answer to. The dashboard queue answers which request
// that is; this only asks every configured forge in turn.
func findRequest(w *World, body string) (foundRequest, bool, error) {
	for _, root := range w.configured {
		store := w.dashboards[filepath.Base(root)]
		if store == nil {
			continue
		}
		request, found, err := store.RequestForBody(body)
		if err != nil {
			return foundRequest{}, false, err
		}
		if found {
			return foundRequest{store: store, id: request.ID}, true, nil
		}
	}
	return foundRequest{}, false, nil
}

func forgeHoldsRequest(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	return waitFor(ctx, fmt.Sprintf("the forge never held a chat request reading %q", captures[1]), func() (bool, error) {
		count, err := countRequests(w, captures[1])
		return count >= 1, err
	})
}

func forgeHoldsOneRequest(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	if err := waitFor(ctx, fmt.Sprintf("the forge never held a chat request reading %q", captures[1]), func() (bool, error) {
		count, err := countRequests(w, captures[1])
		return count >= 1, err
	}); err != nil {
		return err
	}
	if err := fixtures.Sleep(ctx, settle); err != nil {
		return err
	}
	count, err := countRequests(w, captures[1])
	if err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("the forge holds %d chat requests reading %q, want exactly one", count, captures[1])
	}
	return nil
}

func forgeHoldsRequests(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()
	want := captures[1]
	return waitFor(ctx, fmt.Sprintf("the forge never held %s chat requests", want), func() (bool, error) {
		count, err := countRequests(w, "")
		if err != nil {
			return false, err
		}
		return fmt.Sprintf("%d", count) == want, nil
	})
}

func lieutenantWoken(_ context.Context, world any, captures []string) error {
	return dashboardWoke(context.Background(), world, []string{"", captures[1]})
}

// countRequests counts the chat requests every configured forge holds, or the
// ones reading body when body is not empty.
func countRequests(w *World, body string) (int, error) {
	count := 0
	for _, root := range w.configured {
		store := w.dashboards[filepath.Base(root)]
		if store == nil {
			continue
		}
		requests, err := store.Requests()
		if err != nil {
			return 0, err
		}
		for _, request := range requests {
			if body == "" || request.Body == body {
				count++
			}
		}
	}
	return count, nil
}

// forgeHoldsRequestTheDashboardTook checks both halves of who wrote a request:
// the forge holds it, and the dashboard is the one that typed it into the
// lieutenant's pane. A bridge that wrote the queue itself would show the first
// without the second.
func forgeHoldsRequestTheDashboardTook(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	text := captures[1]
	if err := w.holdsRequest(text); err != nil {
		return err
	}
	return w.wokeWithTheRequest(text)
}

// holdsRequest waits until the forge holds a chat request reading text.
func (w *World) holdsRequest(text string) error {
	ctx, cancel := stepContext()
	defer cancel()
	return waitFor(ctx, fmt.Sprintf("the forge never held a chat request reading %q", text), func() (bool, error) {
		count, err := countRequests(w, text)
		return count >= 1, err
	})
}
