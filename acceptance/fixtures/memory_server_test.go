package fixtures

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestMemoryHomeserverRejectsUnknownTokens(t *testing.T) {
	server, err := StartMemoryHomeserver(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(server.Stop)

	req, err := http.NewRequest(http.MethodGet, server.BaseURL()+"/_matrix/client/v3/joined_rooms", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer not-a-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["errcode"] != "M_UNKNOWN_TOKEN" {
		t.Fatalf("errcode = %q, want M_UNKNOWN_TOKEN", body["errcode"])
	}
}

func TestMemoryHomeserverRegistrationIsStrict(t *testing.T) {
	server, err := StartMemoryHomeserver(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(server.Stop)

	userID, token, deviceID, err := server.Register(context.Background(), "operator", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if userID != UserID("operator") || token == "" || deviceID == "" {
		t.Fatalf("registration = %q %q %q", userID, token, deviceID)
	}
	if _, _, _, err := server.Register(context.Background(), "operator", "secret"); err == nil {
		t.Fatal("memory homeserver accepted a duplicate registration")
	}
}

func TestMatrixBackendSelectorIsOptIn(t *testing.T) {
	t.Setenv("FORGELET_MATRIX_BACKEND", "")
	if backend, err := MatrixBackendFromEnv(); err != nil || backend != MatrixBackendFresh {
		t.Fatalf("default backend = %q, %v", backend, err)
	}
	t.Setenv("FORGELET_MATRIX_BACKEND", "memory")
	if backend, err := MatrixBackendFromEnv(); err != nil || backend != MatrixBackendMemory {
		t.Fatalf("selected backend = %q, %v", backend, err)
	}
	t.Setenv("FORGELET_MATRIX_BACKEND", "surprise")
	if _, err := MatrixBackendFromEnv(); err == nil || !strings.Contains(err.Error(), "surprise") {
		t.Fatalf("unknown backend error = %v", err)
	}
}
