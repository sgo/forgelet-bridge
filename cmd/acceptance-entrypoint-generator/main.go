// Command acceptance-entrypoint-generator turns parser JSON IR into thin
// executable acceptance test entry points.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/unclebob/forgelet-bridge/acceptance/generator"
)

const usage = `usage: acceptance-entrypoint-generator <json-ir> <generated-test-output> [--feature <feature-file>] [--project-root <dir>]
`

func main() {
	options, positionals, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "acceptance-entrypoint-generator:", err)
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if len(positionals) != 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	irPath, outputDir := positionals[0], positionals[1]
	projectRoot := options["project-root"]
	if projectRoot == "" {
		if root, err := findProjectRoot(outputDir); err == nil {
			projectRoot = root
		}
	}

	metadata, err := generator.Generate(generator.Request{
		IRPath:      irPath,
		FeaturePath: options["feature"],
		OutputDir:   outputDir,
		ProjectRoot: projectRoot,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "acceptance-entrypoint-generator:", err)
		os.Exit(1)
	}
	fmt.Printf("generated %s (%s)\n", metadata.FeaturePath, metadata.ImplementationHash)
}

// parseArgs accepts the two positional arguments in any position relative to
// the optional flags.
func parseArgs(args []string) (map[string]string, []string, error) {
	options := map[string]string{}
	var positionals []string
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--feature", "--project-root":
			if index+1 >= len(args) {
				return nil, nil, fmt.Errorf("%s needs a value", arg)
			}
			index++
			options[arg[2:]] = args[index]
		default:
			if len(arg) > 2 && arg[0] == '-' {
				return nil, nil, fmt.Errorf("unknown option %s", arg)
			}
			positionals = append(positionals, arg)
		}
	}
	return options, positionals, nil
}

// findProjectRoot walks up from a directory until it finds a go.mod.
func findProjectRoot(dir string) (string, error) {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for current := absolute; ; current = filepath.Dir(current) {
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			return current, nil
		}
		if filepath.Dir(current) == current {
			return "", fmt.Errorf("no go.mod above %s", dir)
		}
	}
}
