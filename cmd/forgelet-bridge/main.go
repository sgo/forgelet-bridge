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

	client, err := matrix.Connect(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer client.Close()

	stores := make(map[string]bridge.ForgeStore, len(cfg.ForgeRoots))
	for _, root := range cfg.ForgeRoots {
		stores[root] = dashboard.Queue{Store: dashboard.New(root)}
	}
	relay, err := bridge.New(cfg, client, stores, log)
	if err != nil {
		return err
	}
	if err := relay.Run(ctx, interval); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("run bridge: %w", err)
	}
	return nil
}
