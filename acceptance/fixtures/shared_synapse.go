package fixtures

import (
	"context"
	"strings"
	"sync"
)

type sharedSynapse struct {
	*Synapse
}

// Backend names this homeserver implementation.
func (s *sharedSynapse) Backend() MatrixBackend {
	return MatrixBackendShared
}

// Stop is a no-op because the shared server lives until the test process ends.
func (s *sharedSynapse) Stop() {}

// Register keeps account identities stable while giving every scenario a new
// device session on the process-scoped server.
func (s *sharedSynapse) Register(ctx context.Context, localpart, password string) (userID, accessToken, deviceID string, err error) {
	userID, accessToken, deviceID, err = s.Synapse.Register(ctx, localpart, password)
	if err == nil {
		return userID, accessToken, deviceID, nil
	}
	if !strings.Contains(err.Error(), "M_USER_IN_USE") {
		return "", "", "", err
	}
	return s.Synapse.Login(ctx, localpart, password)
}

var (
	sharedMu      sync.Mutex
	sharedServer  *sharedSynapse
	sharedWorkDir string
)

// SharedSynapse starts one real Synapse per acceptance test process.
func SharedSynapse(ctx context.Context, dir string) (Homeserver, error) {
	sharedMu.Lock()
	defer sharedMu.Unlock()
	if sharedServer != nil {
		return sharedServer, nil
	}
	server, err := StartSynapse(ctx, dir)
	if err != nil {
		return nil, err
	}
	sharedServer = &sharedSynapse{Synapse: server}
	sharedWorkDir = dir
	return sharedServer, nil
}

// CloseSharedHomeserver stops the process-scoped shared server, if one ran.
func CloseSharedHomeserver() {
	sharedMu.Lock()
	defer sharedMu.Unlock()
	if sharedServer == nil {
		return
	}
	sharedServer.Synapse.Stop()
	sharedServer = nil
	sharedWorkDir = ""
}
