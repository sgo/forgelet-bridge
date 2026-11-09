package fixtures

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePolicy(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "matrix-backends.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMatrixBackendPolicyUsesExactFeatureAndFreshFallback(t *testing.T) {
	policy := writePolicy(t, `{"features":{"features/waiting-facts-as-state.feature":"memory"}}`)
	backend, err := ResolveMatrixBackend(policy, "features/waiting-facts-as-state.feature", "")
	if err != nil || backend != MatrixBackendMemory {
		t.Fatalf("selected backend = %q, %v", backend, err)
	}
	backend, err = ResolveMatrixBackend(policy, "features/label-app-originated-forge-requests.feature", "")
	if err != nil || backend != MatrixBackendFresh {
		t.Fatalf("unlisted backend = %q, %v", backend, err)
	}
}

func TestMatrixBackendPolicyForceWins(t *testing.T) {
	policy := writePolicy(t, `{"features":{"features/waiting-facts-as-state.feature":"memory"}}`)
	backend, err := ResolveMatrixBackend(policy, "features/waiting-facts-as-state.feature", "fresh")
	if err != nil || backend != MatrixBackendFresh {
		t.Fatalf("forced backend = %q, %v", backend, err)
	}
	if _, err := ResolveMatrixBackend(policy, "features/waiting-facts-as-state.feature", "unsafe"); err == nil {
		t.Fatal("invalid forced backend was accepted")
	}
	backend, err = ResolveMatrixBackend(filepath.Join(t.TempDir(), "missing"), "features/waiting-facts-as-state.feature", "memory")
	if err != nil || backend != MatrixBackendMemory {
		t.Fatalf("force without policy = %q, %v", backend, err)
	}
}

func TestMatrixBackendPolicyRejectsUnsafeOrMalformedInput(t *testing.T) {
	cases := []string{
		`{`,
		`{"features":{}} trailing`,
		`{"features":{"features/waiting-facts-as-state.feature":"memory"}} {"features":{}}`,
		`{"features":{}}`,
		`{"features":{"../waiting-facts-as-state.feature":"memory"}}`,
		`{"features":{"features/nested/state.feature":"memory"}}`,
		`{"features":{"features/./waiting-facts-as-state.feature":"memory"}}`,
		`{"features":{"features\\waiting-facts-as-state.feature":"memory"}}`,
		`{"features":{"features/waiting-facts-as-state.feature":"surprise"}}`,
		`{"features":{},"extra":true}`,
	}
	for _, body := range cases {
		policy := writePolicy(t, body)
		if _, err := ResolveMatrixBackend(policy, "features/waiting-facts-as-state.feature", ""); err == nil {
			t.Errorf("policy %q was accepted", body)
		}
	}
	policy := writePolicy(t, `{"features":{"features/waiting-facts-as-state.feature":"memory"}}`)
	if _, err := ResolveMatrixBackend(policy, "../outside.feature", ""); err == nil {
		t.Fatal("unsafe feature path was accepted")
	}
	if _, err := ResolveMatrixBackend(policy, "/tmp/outside.feature", ""); err == nil {
		t.Fatal("absolute feature path was accepted")
	}
	if _, err := ResolveMatrixBackend(filepath.Join(t.TempDir(), "missing"), "features/waiting-facts-as-state.feature", ""); err == nil ||
		!strings.Contains(err.Error(), "read Matrix backend policy") {
		t.Fatalf("missing policy error = %v", err)
	}
}

func TestMatrixBackendPolicyCannotMoveTheRequiredFreshLane(t *testing.T) {
	for _, feature := range RequiredFreshFeatures() {
		policy := writePolicy(t, `{"features":{"`+feature+`":"memory"}}`)
		if _, err := ResolveMatrixBackend(policy, feature, ""); err == nil ||
			!strings.Contains(err.Error(), "required fresh coverage") {
			t.Errorf("%s policy error = %v", feature, err)
		}
	}
}
