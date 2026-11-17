package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/unclebob/forgelet-bridge/internal/config"
	"github.com/unclebob/forgelet-bridge/internal/dashboard"
)

// typedClarificationPreflightFlag runs during flag parsing, before the bridge
// opens Matrix, so the deployment gate cannot start a mixed-authority service.
type typedClarificationPreflightFlag struct{}

func (typedClarificationPreflightFlag) String() string { return "false" }
func (typedClarificationPreflightFlag) IsBoolFlag() bool {
	return true
}

func (typedClarificationPreflightFlag) Set(value string) error {
	os.Exit(typedPreflightExit(value, configPath()))
	return nil
}

func init() {
	flag.Var(typedClarificationPreflightFlag{},
		"typed-clarification-preflight",
		"verify every configured Family serves the typed clarification contract and exit")
}

func configPath() string {
	if config := flag.Lookup("config"); config != nil {
		return config.Value.String()
	}
	return "forgelet-bridge.json"
}

func typedPreflightExit(value, configPath string) int {
	if value != "true" {
		fmt.Fprintln(os.Stderr, "typed clarification preflight is a boolean flag")
		return 2
	}
	if err := checkTypedClarifications(configPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// checkTypedClarifications is the rollout gate for retiring the bridge's
// legacy clarification authority. It reads every configured Family's bounded
// projection and prints only a count; callers turn an error into a bounded
// deployment refusal.
func checkTypedClarifications(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	for _, forge := range cfg.Forges {
		if err := checkTypedClarificationFamily(cfg.ForgeName(forge.Root), forge); err != nil {
			return err
		}
	}
	fmt.Printf("typed clarification preflight passed for %d Families\n", len(cfg.Forges))
	return nil
}

func checkTypedClarificationFamily(name string, forge config.Forge) error {
	address, err := dashboard.URL(forge.Root, forge.DashboardURL)
	if err != nil {
		return fmt.Errorf("typed clarification preflight for Family %s: %w", name, err)
	}
	state, err := dashboard.NewForgeAPI(forge.Root, address).AutomationState(context.Background())
	if err != nil {
		return fmt.Errorf("typed clarification preflight for Family %s: %w", name, err)
	}
	if state.Identity.Kind != "family" {
		return fmt.Errorf("typed clarification preflight for Family %s: identity is %q, want family",
			name, state.Identity.Kind)
	}
	return nil
}
