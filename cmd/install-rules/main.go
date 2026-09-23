// Command install-rules installs the prompt rules the bridge's rooms rely on
// into a forge's own constitution and the packs new projects come from.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/unclebob/forgelet-bridge/internal/rules"
)

func main() {
	forgeRoot := flag.String("forge-root", "", "the forge to install the rules into")
	rulesDir := flag.String("rules", "rules", "the directory of marked rule blocks to install")
	flag.Parse()

	if err := run(*forgeRoot, *rulesDir, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "install-rules:", err)
		os.Exit(1)
	}
}

// run installs every rule and writes what it did to out: the report is the
// tool's contract with whoever runs it, so it is a value the caller hands in
// rather than the process's own output.
func run(forgeRoot, rulesDir string, out io.Writer) error {
	if forgeRoot == "" {
		return fmt.Errorf("--forge-root names the forge to install into")
	}
	loaded, err := rules.Load(rulesDir)
	if err != nil {
		return err
	}
	report, err := rules.Install(forgeRoot, loaded)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, report)
	return nil
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-23T15:22:08+02:00","module_hash":"1bd0ef2d8b40aebc131fa5bd20423411e4f05f84739039df343a6bd90f014f48","functions":[{"id":"func/main","name":"main","line":14,"end_line":23,"hash":"db7b9aaa1a3e1f508521a984130a27c199a0b68181257769ef9c388f95187cba"},{"id":"func/run","name":"run","line":28,"end_line":42,"hash":"4571093775585765a05531e6a65f0f976af471ca9d4f5acf84e703722d8432fe"}]}
// mutate4go-manifest-end
