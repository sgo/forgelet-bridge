package bridge

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// StatusName is the file the bridge keeps its progress in, so that an operator
// or a test can tell when it has caught up.
const StatusName = "status.json"

// Status is what the bridge writes after every tick. The device fields tell
// the operator (and the acceptance suite) which Matrix device the bridge is
// using, so a restart that changes it is visible instead of silent.
type Status struct {
	Tick              uint64 `json:"tick"`
	Idle              bool   `json:"idle"`
	DeviceID          string `json:"device_id,omitempty"`
	DeviceFingerprint string `json:"device_fingerprint,omitempty"`
	// Pending is how many actions the forge has not carried out yet, and
	// LastError is the most recent refusal: a bridge that is stuck says so
	// instead of looking quiet.
	Pending   int    `json:"pending,omitempty"`
	LastError string `json:"last_error,omitempty"`
}

// Device is the Matrix device the bridge is using.
type Device struct {
	ID          string
	Fingerprint string
}

// ReportDevice records the Matrix device the bridge is using, so that its
// status shows which device the operator is talking to.
func (b *Bridge) ReportDevice(device Device) {
	b.device = device
}

// writeStatus writes the bridge's progress report where an operator or a test
// can read it.
func (b *Bridge) writeStatus(status Status) error {
	data, err := json.Marshal(status)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(b.statusDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(b.statusDir, StatusName), append(data, '\n'), 0o644)
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-22T12:38:07+02:00","module_hash":"990268654ffe17ace31266d04d84f475e32abb8dc19a61f46fa1c94ebba37bdc","functions":[{"id":"func/Bridge.ReportDevice","name":"Bridge.ReportDevice","line":31,"end_line":33,"hash":"9abf9e292aafe2f10a8204dd1970ebe3416ad531653c59425a7f93968de761a4"},{"id":"func/Bridge.writeStatus","name":"Bridge.writeStatus","line":37,"end_line":46,"hash":"79a205547a958b3a5f9d1cc48f96e9549e6f42304bed9bf643585c7433bfa3dd"}]}
// mutate4go-manifest-end
