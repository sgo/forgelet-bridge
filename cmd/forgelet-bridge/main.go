// Command forgelet-bridge relays one forge's operator chat channel to a Matrix
// room the bridge creates and keeps for that forge.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/unclebob/forgelet-bridge/internal/board"
	"github.com/unclebob/forgelet-bridge/internal/bridge"
	"github.com/unclebob/forgelet-bridge/internal/config"
	"github.com/unclebob/forgelet-bridge/internal/dashboard"
	"github.com/unclebob/forgelet-bridge/internal/matrix"
)

func main() {
	configPath := flag.String("config", "forgelet-bridge.json", "path to the bridge configuration")
	interval := flag.Duration("interval", time.Second, "how often the bridge checks the forge and its chat room")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, *configPath, *interval, log); err != nil {
		log.Error("bridge stopped", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, configPath string, interval time.Duration, log *slog.Logger) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	return serve(ctx, cfg, interval, log)
}

// serve connects to the homeserver and relays every configured forge until the
// context ends.
func serve(ctx context.Context, cfg config.Config, interval time.Duration, log *slog.Logger) error {
	client, err := matrix.Connect(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer client.Close()

	opened := openAdapters(cfg)
	relay, err := bridge.New(cfg, client, opened.chat, opened.approvals, opened.clarifications, opened.boards, log)
	if err != nil {
		return err
	}
	deviceID, fingerprint := client.DeviceIdentity()
	relay.ReportDevice(bridge.Device{ID: deviceID, Fingerprint: fingerprint})
	if err := relay.Run(ctx, interval); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("run bridge: %w", err)
	}
	return nil
}

// adapters are the forge-side adapters the bridge serves the configured forges
// through: each forge's dashboard chat-request queue, its approvals, its
// clarifications, and its project boards.
type adapters struct {
	chat           map[string]bridge.ForgeStore
	approvals      map[string]bridge.ApprovalStore
	clarifications map[string]bridge.ClarificationStore
	boards         map[string]bridge.BoardStore
}

// openAdapters opens the chat queue, the approvals and the boards of every
// configured forge, keyed by the forge's root. The approvals are the forge's
// dashboard's own API, at the address the configuration gives that forge or,
// when it gives none, the one the dashboard announces in the forge.
func openAdapters(cfg config.Config) adapters {
	opened := adapters{
		chat:           make(map[string]bridge.ForgeStore, len(cfg.Forges)),
		approvals:      make(map[string]bridge.ApprovalStore, len(cfg.Forges)),
		clarifications: make(map[string]bridge.ClarificationStore, len(cfg.Forges)),
		boards:         make(map[string]bridge.BoardStore, len(cfg.Forges)),
	}
	for _, forge := range cfg.Forges {
		opened.chat[forge.Root] = dashboard.Queue{Store: dashboard.New(forge.Root), Root: forge.Root, ConfiguredURL: forge.DashboardURL}
		opened.approvals[forge.Root] = dashboard.Approvals{Root: forge.Root, ConfiguredURL: forge.DashboardURL}
		opened.clarifications[forge.Root] = dashboard.Clarifications{Root: forge.Root, ConfiguredURL: forge.DashboardURL}
		opened.boards[forge.Root] = board.Queue{Store: board.New(forge.Root)}
	}
	return opened
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-23T13:46:00+02:00","module_hash":"6874ee79f34e6c83eeb91749afeda0334bc326800861d3e0bad933c00b5d1f6b","functions":[{"id":"func/main","name":"main","line":23,"end_line":36,"hash":"5066fd9e175ab9e6c5e67a472b92bb8efcc47a63d1edbc09eb0cb1f0ba67bfae"},{"id":"func/run","name":"run","line":38,"end_line":44,"hash":"6e71599705ecb1059bc12818e5e38148a22f6e81349cdf5f0cc1d40affe65b92"},{"id":"func/serve","name":"serve","line":48,"end_line":66,"hash":"c7e9a80d2a9a430bbbfbab9b2b96bf2120ed25f3722cde767d21e2660a37cfa3"},{"id":"func/openAdapters","name":"openAdapters","line":82,"end_line":96,"hash":"4230a79a02592682d3b03daa40772d255fedbfa060758b33d7a19eff62cb8a46"}]}
// mutate4go-manifest-end
