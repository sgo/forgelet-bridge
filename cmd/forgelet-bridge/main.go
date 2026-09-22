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

	approvalspkg "github.com/unclebob/forgelet-bridge/internal/approvals"
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
	relay, err := bridge.New(cfg, client, opened.chat, opened.approvals, log)
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
// through: each forge's dashboard chat-request queue and its approvals.
type adapters struct {
	chat      map[string]bridge.ForgeStore
	approvals map[string]bridge.ApprovalStore
}

// openAdapters opens the dashboard queue and the approvals of every configured
// forge, keyed by the forge's root.
func openAdapters(cfg config.Config) adapters {
	opened := adapters{
		chat:      make(map[string]bridge.ForgeStore, len(cfg.Forges)),
		approvals: make(map[string]bridge.ApprovalStore, len(cfg.Forges)),
	}
	for _, forge := range cfg.Forges {
		opened.chat[forge.Root] = dashboard.Queue{Store: dashboard.New(forge.Root)}
		opened.approvals[forge.Root] = approvalspkg.Queue{Store: approvalspkg.New(forge.Root)}
	}
	return opened
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-21T23:18:32+02:00","module_hash":"62f93dab599792b55d7395e27014e148e6edcabda045a05375c7fc677964f36b","functions":[{"id":"func/main","name":"main","line":22,"end_line":35,"hash":"5066fd9e175ab9e6c5e67a472b92bb8efcc47a63d1edbc09eb0cb1f0ba67bfae"},{"id":"func/run","name":"run","line":37,"end_line":43,"hash":"6e71599705ecb1059bc12818e5e38148a22f6e81349cdf5f0cc1d40affe65b92"},{"id":"func/serve","name":"serve","line":47,"end_line":62,"hash":"f7638e66e2ee40dc9647f24c878555bb73961f3596816718e002499976984024"},{"id":"func/stores","name":"stores","line":65,"end_line":71,"hash":"2dfbfa985e246e245fc7c58c5061b59d98c56d25880917c593868c3751c5de55"}]}
// mutate4go-manifest-end
