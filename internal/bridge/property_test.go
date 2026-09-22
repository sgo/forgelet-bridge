//go:build property

package bridge

import (
	"context"
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/quick"
)

// TestPropertyStatusKeepsTheReportedDevice checks the progress file an operator
// (and the acceptance suite) reads: whatever Matrix device the bridge reports
// it is using, the file names that device, so a restart that changed it shows
// up instead of staying silent.
func TestPropertyStatusKeepsTheReportedDevice(t *testing.T) {
	rooms := &fakeRooms{}
	built, cfg := newTestBridge(t, rooms, map[string]ForgeStore{"/forges/forge-a": &fakeStore{}}, "/forges/forge-a")

	property := func(device Device) bool {
		built.ReportDevice(device)
		if err := built.Tick(context.Background()); err != nil {
			return false
		}
		data, err := os.ReadFile(filepath.Join(cfg.StateDir, StatusName))
		if err != nil {
			return false
		}

		var status Status
		if err := json.Unmarshal(data, &status); err != nil {
			return false
		}
		if status.DeviceID != device.ID || status.DeviceFingerprint != device.Fingerprint {
			return false
		}

		raw := map[string]any{}
		if err := json.Unmarshal(data, &raw); err != nil {
			return false
		}
		return reportsDevice(raw, device)
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 200,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randomDevice(rnd))
		},
	}); err != nil {
		t.Error(err)
	}
}

// reportsDevice reports whether a status file read as raw JSON names the device
// the bridge is using, leaving the field out when there is nothing to report.
func reportsDevice(raw map[string]any, device Device) bool {
	named, present := raw["device_id"]
	if device.ID == "" {
		return !present
	}
	if !present || named != device.ID {
		return false
	}
	fingerprint, present := raw["device_fingerprint"]
	return device.Fingerprint == "" || (present && fingerprint == device.Fingerprint)
}

// randomDevice is a device id and fingerprint of the shape the bridge reports,
// with the empty device the bridge has before it connects.
func randomDevice(rnd *rand.Rand) Device {
	ids := []string{"", "FORGELETBRIDGE", "DEVICE2", "device with spaces"}
	fingerprints := []string{
		"",
		"Ed25519:abc123",
		"qT4H1VvJdZ0zPqXl8mNbRcSgUkWoYhEiArTlMnOpQs=",
		"key with spaces",
	}
	return Device{
		ID:          ids[rnd.Intn(len(ids))],
		Fingerprint: fingerprints[rnd.Intn(len(fingerprints))],
	}
}
