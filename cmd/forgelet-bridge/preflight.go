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

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-11-17T15:24:43+01:00","module_hash":"45f462a324d4069e34152e85e4a01d11be1c62ed9aadaad8208ceefa083ffec9","functions":[{"id":"func/typedClarificationPreflightFlag.String","name":"typedClarificationPreflightFlag.String","line":17,"end_line":17,"hash":"2aafad38606651c3fe076ac5415708627402944f9e3ad14cdf6d3e7a3a1d1221"},{"id":"func/typedClarificationPreflightFlag.IsBoolFlag","name":"typedClarificationPreflightFlag.IsBoolFlag","line":18,"end_line":20,"hash":"0081b2fa3f92a41f34523011d069cde3a7dfc301e9b675673110151345aa6aa3"},{"id":"func/typedClarificationPreflightFlag.Set","name":"typedClarificationPreflightFlag.Set","line":22,"end_line":25,"hash":"30969d072089dbd10f7e6c8af8f5b3369baaf6af062ab9726d6f8f3524d5d6e5"},{"id":"func/init","name":"init","line":27,"end_line":31,"hash":"f382b44c7b49a0f44492e284110ca378db775c04f86d22c851b2e87e453b7401"},{"id":"func/configPath","name":"configPath","line":33,"end_line":38,"hash":"5cb738bd1300c21a32cbdb8ff59c3c6ada16a73ba14ef9f65e3d3f8d8aef2924"},{"id":"func/typedPreflightExit","name":"typedPreflightExit","line":40,"end_line":50,"hash":"0462d8b2c8e92348a1fcf47387daea397e938e280d9ac50785e279550d19875f"},{"id":"func/checkTypedClarifications","name":"checkTypedClarifications","line":56,"end_line":68,"hash":"df04fa1fc0a5f510aab5c161d201735ae2a1860811f4a6d7368c150fef46d9dd"},{"id":"func/checkTypedClarificationFamily","name":"checkTypedClarificationFamily","line":70,"end_line":84,"hash":"b5ce0edce151dc6a55fade9887876049ae9c4e28dd737c78c93e9df5bf89c4ac"}]}
// mutate4go-manifest-end
