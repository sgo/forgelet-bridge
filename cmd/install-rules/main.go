// Command install-rules installs the prompt rules the bridge's rooms rely on
// into a forge's own constitution and the packs new projects come from.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/unclebob/forgelet-bridge/internal/rules"
)

func main() {
	forgeRoot := flag.String("forge-root", "", "the forge to install the rules into")
	rulesDir := flag.String("rules", "rules", "the directory of marked rule blocks to install")
	flag.Parse()

	if err := run(*forgeRoot, *rulesDir); err != nil {
		fmt.Fprintln(os.Stderr, "install-rules:", err)
		os.Exit(1)
	}
}

// run installs every rule and prints what it did.
func run(forgeRoot, rulesDir string) error {
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
	fmt.Println(report)
	return nil
}
