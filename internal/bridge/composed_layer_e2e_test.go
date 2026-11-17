package bridge

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/unclebob/forgelet-bridge/internal/board"
	"github.com/unclebob/forgelet-bridge/internal/config"
	"github.com/unclebob/forgelet-bridge/internal/dashboard"
)

// TestComposedCurrentLayerReachesEveryFamilyInOneTick is the card E2E probe.
// It uses the configured dashboards and the same adapters as the production
// bridge, while the test room boundary keeps the check off the live homeserver.
func TestComposedCurrentLayerReachesEveryFamilyInOneTick(t *testing.T) {
	configPath := os.Getenv("FORGELET_COMPOSED_BRIDGE_CONFIG")
	if configPath == "" {
		t.Skip("FORGELET_COMPOSED_BRIDGE_CONFIG is set by the composed-layer E2E procedure")
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load composed bridge configuration: %v", err)
	}
	if len(cfg.Forges) != 4 {
		t.Fatalf("configured Families = %d, want the card's four", len(cfg.Forges))
	}
	cfg.StateDir = t.TempDir()
	stores := map[string]ForgeStore{}
	approvals := map[string]ApprovalStore{}
	clarifications := map[string]ClarificationStore{}
	boards := map[string]BoardStore{}
	wantReached := make([]string, 0, len(cfg.Forges))
	for _, forge := range cfg.Forges {
		stores[forge.Root] = dashboard.Queue{
			Store: dashboard.New(forge.Root), Root: forge.Root, ConfiguredURL: forge.DashboardURL,
		}
		approvals[forge.Root] = dashboard.Approvals{Root: forge.Root, ConfiguredURL: forge.DashboardURL}
		clarifications[forge.Root] = dashboard.Clarifications{Root: forge.Root, ConfiguredURL: forge.DashboardURL}
		boards[forge.Root] = board.Queue{Store: board.New(forge.Root), Forge: forge.Root}
		wantReached = append(wantReached, cfg.ForgeName(forge.Root))
	}
	built, err := New(
		cfg, &fakeRooms{}, stores, approvals, clarifications, boards,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	if err != nil {
		t.Fatalf("build composed bridge: %v", err)
	}
	if err := built.Tick(context.Background()); err != nil {
		t.Fatalf("composed Tick: %v", err)
	}
	status := readStatus(t, filepath.Join(cfg.StateDir, StatusName))
	if !reflect.DeepEqual(status.ReachedForges, wantReached) {
		t.Errorf("reached Families = %v, want %v", status.ReachedForges, wantReached)
	}
	if len(status.UnhappyForges) != 0 {
		t.Errorf("unhappy Families = %v (%s), want none", status.UnhappyForges, status.LastError)
	}
	if len(status.ProjectionFallbacks) != 0 {
		t.Errorf("projection fallbacks = %v, want none", status.ProjectionFallbacks)
	}
	t.Logf(
		"composed tick %d reached %d Families: %v; owed items: %v",
		status.Tick, len(status.ReachedForges), status.ReachedForges, status.Owed,
	)
}
