package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/internal/dashboard"
)

func dashboardHoldsRequest(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	root, err := singleForge(w)
	if err != nil {
		return err
	}
	store := w.dashboards[filepath.Base(root)]
	if store == nil {
		return fmt.Errorf("no fixture dashboard for %s", root)
	}
	_, err = store.CreateRequest(captures[1])
	return err
}

// namedDashboardHoldsRequest seeds a chat request in one named forge's
// dashboard, for scenarios that serve more than one forge.
func namedDashboardHoldsRequest(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	store, err := w.declaredForge(captures[1])
	if err != nil {
		return err
	}
	_, err = store.CreateRequest(captures[2])
	return err
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
	w := world.(*World)
	root, err := singleForge(w)
	if err != nil {
		return err
	}
	ctx, cancel := stepContext()
	defer cancel()
	wakeLog := filepath.Join(root, ".swarmforge", "dashboard", "wake.log")
	body := captures[1]
	return waitFor(ctx, fmt.Sprintf("the lieutenant was never woken with %q", body), func() (bool, error) {
		data, err := os.ReadFile(wakeLog)
		if err != nil {
			if os.IsNotExist(err) {
				return false, nil
			}
			return false, err
		}
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			if _, recorded, found := strings.Cut(line, "\t"); found && recorded == body {
				return true, nil
			}
		}
		return false, nil
	})
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
