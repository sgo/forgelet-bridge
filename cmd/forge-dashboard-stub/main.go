// Command forge-dashboard-stub stands in for a forge's dashboard. It watches
// the dashboard request queue and records the wake it would give the
// lieutenant when a new chat request lands, the way the real dashboard opens
// the request in the agent's pane.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func main() {
	root := flag.String("root", "", "forge root whose dashboard this stub is")
	flag.Parse()
	if *root == "" {
		fmt.Fprintln(os.Stderr, "usage: forge-dashboard-stub --root <forge-root>")
		os.Exit(2)
	}
	if err := run(*root); err != nil {
		log.Fatal(err)
	}
}

func run(root string) error {
	pendingDir := filepath.Join(root, ".swarmforge", "dashboard", "requests", "pending")
	wakeLog := filepath.Join(root, ".swarmforge", "dashboard", "wake.log")
	if err := os.MkdirAll(filepath.Dir(wakeLog), 0o755); err != nil {
		return err
	}

	seen := map[string]bool{}
	for {
		entries, err := os.ReadDir(pendingDir)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			if entry.IsDir() || seen[entry.Name()] || !strings.HasSuffix(entry.Name(), ".request") {
				continue
			}
			seen[entry.Name()] = true
			request, err := os.ReadFile(filepath.Join(pendingDir, entry.Name()))
			if err != nil {
				continue
			}
			if err := recordWake(wakeLog, entry.Name(), bodyOf(string(request))); err != nil {
				return err
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
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

// bodyOf reads the request body that follows the blank line.
func bodyOf(request string) string {
	_, body, found := strings.Cut(request, "\n\n")
	if !found {
		return ""
	}
	return strings.TrimRight(body, "\n")
}
