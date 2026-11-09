package steps

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
)

const waitingFactsFeature = "features/waiting-facts-as-state.feature"

func matrixPolicyMaps(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	return w.writeMatrixPolicy(map[string]fixtures.MatrixBackend{
		captures[1]: fixtures.MatrixBackend(captures[2]),
	})
}

func malformedMatrixPolicy(_ context.Context, world any, _ []string) error {
	return world.(*World).writeRawMatrixPolicy("{\"features\":")
}

func unsafeMatrixPolicy(_ context.Context, world any, _ []string) error {
	return world.(*World).writeRawMatrixPolicy(
		`{"features":{"../outside.feature":"memory"}}`,
	)
}

func checkedInMatrixPolicy(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	w.matrixPolicyPath = filepath.Join(fixtures.ProjectRoot(), "acceptance", "matrix-backends.json")
	return nil
}

func resolveMatrixPolicy(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	w.resolveMatrixPolicy(captures[1], "")
	return nil
}

func resolveForcedMatrixPolicy(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	w.resolveMatrixPolicy(captures[1], captures[2])
	return nil
}

func resolveStoredMatrixPolicy(_ context.Context, world any, _ []string) error {
	world.(*World).resolveMatrixPolicy(waitingFactsFeature, "")
	return nil
}

func selectedMatrixBackend(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	if w.matrixPolicyErr != nil {
		return w.matrixPolicyErr
	}
	if w.matrixPolicyBackend != fixtures.MatrixBackend(captures[1]) {
		return fmt.Errorf("selected Matrix backend %q, want %s", w.matrixPolicyBackend, captures[1])
	}
	return nil
}

func featureSelectsMatrixBackend(_ context.Context, world any, captures []string) error {
	w := world.(*World)
	w.resolveMatrixPolicy(captures[1], "")
	if w.matrixPolicyErr != nil {
		return w.matrixPolicyErr
	}
	if w.matrixPolicyBackend != fixtures.MatrixBackend(captures[2]) {
		return fmt.Errorf("%s selected %s, want %s", captures[1], w.matrixPolicyBackend, captures[2])
	}
	return nil
}

func compatibilityFeaturesSelectFresh(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	for _, feature := range fixtures.RequiredFreshFeatures() {
		w.resolveMatrixPolicy(feature, "")
		if w.matrixPolicyErr != nil {
			return w.matrixPolicyErr
		}
		if w.matrixPolicyBackend != fixtures.MatrixBackendFresh {
			return fmt.Errorf("%s selected %s, want fresh Synapse", feature, w.matrixPolicyBackend)
		}
	}
	return nil
}

func matrixPolicyResolutionFailed(_ context.Context, world any, _ []string) error {
	if world.(*World).matrixPolicyErr == nil {
		return fmt.Errorf("Matrix backend policy resolution succeeded, want a visible failure")
	}
	return nil
}

func (w *World) writeMatrixPolicy(features map[string]fixtures.MatrixBackend) error {
	body, err := json.MarshalIndent(map[string]any{"features": features}, "", "  ")
	if err != nil {
		return err
	}
	return w.writeRawMatrixPolicy(string(body))
}

func (w *World) writeRawMatrixPolicy(body string) error {
	dir := filepath.Join(w.workDir, "matrix-policy")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	w.matrixPolicyPath = filepath.Join(dir, "matrix-backends.json")
	return os.WriteFile(w.matrixPolicyPath, []byte(body), 0o644)
}

func (w *World) resolveMatrixPolicy(featurePath, forced string) {
	w.matrixPolicyBackend, w.matrixPolicyErr = fixtures.ResolveMatrixBackend(
		w.matrixPolicyPath,
		featurePath,
		forced,
	)
}
