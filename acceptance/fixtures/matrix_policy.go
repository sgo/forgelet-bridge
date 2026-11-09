package fixtures

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const matrixPolicyPath = "acceptance/matrix-backends.json"

type matrixPolicy struct {
	Features map[string]MatrixBackend `json:"features"`
}

var requiredFreshFeatures = []string{
	"features/bridge-device-continuity.feature",
	"features/bridge-space-provisioning.feature",
	"features/bridge-restart-continuity.feature",
}

// RequiredFreshFeatures names the real-Synapse compatibility lane. The
// validator refuses a policy that would move any of these features off fresh.
func RequiredFreshFeatures() []string {
	return append([]string(nil), requiredFreshFeatures...)
}

// MatrixBackendForFeature resolves a feature's test backend. A valid forced
// backend wins; otherwise the feature policy is used, with fresh as fallback.
func MatrixBackendForFeature(featurePath string) (MatrixBackend, error) {
	forced := strings.TrimSpace(os.Getenv("FORGELET_MATRIX_BACKEND"))
	policyPath := filepath.Join(ProjectRoot(), matrixPolicyPath)
	return ResolveMatrixBackend(policyPath, featurePath, forced)
}

// ResolveMatrixBackend validates a per-feature policy and returns the selected
// backend. Empty policy files are invalid so missing migration policy cannot
// silently weaken coverage.
func ResolveMatrixBackend(policyPath, featurePath, forced string) (MatrixBackend, error) {
	forced = strings.TrimSpace(forced)
	if forced != "" {
		backend := MatrixBackend(forced)
		if !validMatrixBackend(backend) {
			return "", fmt.Errorf("forced Matrix backend %q is invalid", forced)
		}
		return backend, nil
	}

	data, err := os.ReadFile(policyPath)
	if err != nil {
		return "", fmt.Errorf("read Matrix backend policy: %w", err)
	}
	policy, err := decodeMatrixPolicy(data)
	if err != nil {
		return "", err
	}
	if len(policy.Features) == 0 {
		return "", fmt.Errorf("Matrix backend policy has no features map")
	}
	for path, backend := range policy.Features {
		if err := validateFeaturePath(path); err != nil {
			return "", fmt.Errorf("unsafe Matrix backend policy path %q: %w", path, err)
		}
		if !validMatrixBackend(backend) {
			return "", fmt.Errorf("Matrix backend %q for %s is invalid", backend, path)
		}
		if requiredFresh(path) && backend != MatrixBackendFresh {
			return "", fmt.Errorf("Matrix backend %q would weaken required fresh coverage for %s", backend, path)
		}
	}
	path := filepath.ToSlash(featurePath)
	if err := validateFeaturePath(path); err != nil {
		return "", fmt.Errorf("unsafe feature path %q: %w", featurePath, err)
	}
	if backend, listed := policy.Features[path]; listed {
		return backend, nil
	}
	return MatrixBackendFresh, nil
}

func decodeMatrixPolicy(data []byte) (matrixPolicy, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var policy matrixPolicy
	if err := decoder.Decode(&policy); err != nil {
		return matrixPolicy{}, fmt.Errorf("parse Matrix backend policy: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return matrixPolicy{}, fmt.Errorf("parse Matrix backend policy: trailing JSON")
	}
	return policy, nil
}

func validateFeaturePath(path string) error {
	if path == "" || filepath.IsAbs(path) || strings.Contains(path, "\\") ||
		filepath.ToSlash(filepath.Clean(path)) != path {
		return fmt.Errorf("path must be relative and remain under features/")
	}
	if !strings.HasPrefix(path, "features/") || !strings.HasSuffix(path, ".feature") {
		return fmt.Errorf("path must name one features/*.feature file")
	}
	if strings.Contains(strings.TrimPrefix(path, "features/"), "/") {
		return fmt.Errorf("path must not name a nested feature")
	}
	return nil
}

func validMatrixBackend(backend MatrixBackend) bool {
	switch backend {
	case MatrixBackendFresh, MatrixBackendShared, MatrixBackendMemory:
		return true
	default:
		return false
	}
}

func requiredFresh(path string) bool {
	for _, feature := range requiredFreshFeatures {
		if path == feature {
			return true
		}
	}
	return false
}
