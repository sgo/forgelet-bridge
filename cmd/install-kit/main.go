// Command install-kit installs the tools that make a forge behave - the route
// gate, the idler check and the stall watch - into a forge's own scripts, with
// the watch's agent, and self-checks each one against that forge.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/unclebob/forgelet-bridge/internal/kit"
)

func main() {
	forgeRoot := flag.String("forge-root", "", "the forge to install the kit into")
	kitDir := flag.String("kit", "swarmforge/scripts", "the directory the kit ships its tools from")
	flag.Parse()

	if err := run(*forgeRoot, *kitDir, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "install-kit:", err)
		os.Exit(1)
	}
}

// run installs the kit and writes what it did to out: the report is the tool's
// contract with whoever runs it, so it is a value the caller hands in rather
// than the process's own output. A self-check that read nothing is a failure,
// and the report says so before this returns.
func run(forgeRoot, kitDir string, out io.Writer) error {
	if forgeRoot == "" {
		return fmt.Errorf("--forge-root names the forge to install into")
	}
	report, err := kit.Install(forgeRoot, kitDir)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, report)
	if report.Failed() {
		return fmt.Errorf("a self-check failed: a tool that reads nothing is not an install")
	}
	return nil
}
