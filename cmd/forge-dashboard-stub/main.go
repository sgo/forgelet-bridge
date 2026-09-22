// Command forge-dashboard-stub stands in for a forge's dashboard. It serves the
// endpoints the bridge has to call, applies the same file effects the desktop
// dashboard applies, watches the request queue, and records the wake it would
// give the lieutenant when a new chat request lands, the way the real dashboard
// opens the request in the agent's pane.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/internal/dashboard"
)

// pollInterval is how often the stub asks the dashboard queue for new work.
const pollInterval = 100 * time.Millisecond

func main() {
	root := flag.String("root", "", "forge root whose dashboard this stub is")
	flag.Parse()
	if *root == "" {
		fmt.Fprintln(os.Stderr, "usage: forge-dashboard-stub --root <forge-root>")
		os.Exit(2)
	}
	dashboardServer, err := fixtures.StartDashboard(*root)
	if err != nil {
		log.Fatal(err)
	}
	defer dashboardServer.Stop()
	log.Printf("dashboard listening at %s", dashboardServer.URL)
	if err := run(*root); err != nil {
		log.Fatal(err)
	}
}

func run(root string) error {
	watcher := &watcher{
		queue:   dashboard.New(root),
		wakeLog: filepath.Join(root, ".swarmforge", "dashboard", "wake.log"),
		woken:   map[string]bool{},
	}
	if err := os.MkdirAll(filepath.Dir(watcher.wakeLog), 0o755); err != nil {
		return err
	}

	for {
		if err := watcher.wakeNewRequests(); err != nil {
			// A queue that cannot be read this moment — the dashboard may be
			// answering a request as this poll lists it — is read again on the
			// next poll, the way the bridge keeps serving a failing tick.
			log.Println("forge-dashboard-stub:", err)
		}
		time.Sleep(pollInterval)
	}
}

// watcher records one wake per chat request that lands in a forge's dashboard.
type watcher struct {
	queue   *dashboard.Store
	wakeLog string
	woken   map[string]bool
}

// wakeNewRequests records the wake the dashboard would give the lieutenant for
// every chat request it holds and has not been woken for yet. The queue is the
// dashboard module's answer to what the lieutenant still has to answer.
func (w *watcher) wakeNewRequests() error {
	pending, err := w.queue.Pending()
	if err != nil {
		return err
	}
	for _, request := range pending {
		if w.woken[request.ID] {
			continue
		}
		if err := recordWake(w.wakeLog, request.ID, request.Body); err != nil {
			return err
		}
		w.woken[request.ID] = true
	}
	return nil
}

func recordWake(path, id, body string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = fmt.Fprintf(file, "%s\t%s\n", id, body)
	return err
}
