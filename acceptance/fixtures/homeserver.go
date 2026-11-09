package fixtures

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// MatrixBackend names the homeserver implementation an acceptance run uses.
type MatrixBackend string

const (
	MatrixBackendFresh  MatrixBackend = "fresh"
	MatrixBackendShared MatrixBackend = "shared"
	MatrixBackendMemory MatrixBackend = "memory"
)

// MatrixBackendFromEnv reads the opt-in backend selector.
func MatrixBackendFromEnv() (MatrixBackend, error) {
	value := strings.TrimSpace(os.Getenv("FORGELET_MATRIX_BACKEND"))
	if value == "" {
		return MatrixBackendFresh, nil
	}
	backend := MatrixBackend(value)
	switch backend {
	case MatrixBackendFresh, MatrixBackendShared, MatrixBackendMemory:
		return backend, nil
	default:
		return "", fmt.Errorf("unknown Matrix fixture backend %q", value)
	}
}

// Homeserver is the fixture boundary shared by real Synapse and the in-memory
// Matrix server.
type Homeserver interface {
	BaseURL() string
	Register(context.Context, string, string) (userID, accessToken, deviceID string, err error)
	Backend() MatrixBackend
	Stop()
}
