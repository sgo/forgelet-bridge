package steps

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/internal/bridge"
)

// bridgePublishedDevice records the Matrix device the bridge reports it is
// using, and checks that the operator sees it: that is the device the phone
// would show.
func bridgePublishedDevice(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()

	if err := w.waitForPublishedDevice(ctx); err != nil {
		return err
	}
	w.devices, _ = w.bridgeDevices(ctx)
	return nil
}

// operatorSeesSameDevice checks that the restart left the bridge on the same
// Matrix device: the device the bridge reports now must still be the one the
// operator sees, with the same identity key, and no new device may appear.
func operatorSeesSameDevice(_ context.Context, world any, _ []string) error {
	w := world.(*World)
	ctx, cancel := stepContext()
	defer cancel()

	if len(w.devices) == 0 {
		return fmt.Errorf("the operator has not seen the bridge's Matrix device yet")
	}
	if err := w.waitForPublishedDevice(ctx); err != nil {
		return err
	}
	after, err := w.bridgeDevices(ctx)
	if err != nil {
		return err
	}
	return sameDevice(w.devices, after)
}

// waitForPublishedDevice waits until the operator sees the device the bridge
// reports it is using, with the identity key the bridge reports.
func (w *World) waitForPublishedDevice(ctx context.Context) error {
	device, err := w.reportedDevice()
	if err != nil {
		return err
	}
	return waitFor(ctx, fmt.Sprintf("the operator never saw the bridge's Matrix device %s", device.ID), func() (bool, error) {
		seen, err := w.bridgeDevices(ctx)
		if err != nil {
			return false, err
		}
		for _, candidate := range seen {
			if candidate.DeviceID == device.ID {
				if device.Fingerprint != "" && candidate.Ed25519 != device.Fingerprint {
					return false, fmt.Errorf("the bridge device %s has identity key %s on the homeserver but the bridge reports %s",
						device.ID, candidate.Ed25519, device.Fingerprint)
				}
				return true, nil
			}
		}
		return false, nil
	})
}

// reportedDevice is the Matrix device the bridge says it is using.
func (w *World) reportedDevice() (bridge.Device, error) {
	status, err := readStatus(filepath.Join(w.stateDir, bridge.StatusName))
	if err != nil {
		return bridge.Device{}, err
	}
	if status.DeviceID == "" {
		return bridge.Device{}, fmt.Errorf("the bridge has not reported its Matrix device yet")
	}
	return bridge.Device{ID: status.DeviceID, Fingerprint: status.DeviceFingerprint}, nil
}

// bridgeDevices asks the operator's client which devices the bridge has.
func (w *World) bridgeDevices(ctx context.Context) ([]fixtures.DeviceKey, error) {
	operator, err := w.operator(ctx)
	if err != nil {
		return nil, err
	}
	return operator.Devices(ctx, w.bridgeUserID)
}

// sameDevice reports whether the operator still sees exactly the devices it
// saw before, each with the same identity key.
func sameDevice(before, after []fixtures.DeviceKey) error {
	if len(before) == 0 {
		return fmt.Errorf("the operator saw no bridge device before the restart")
	}
	if len(after) == 0 {
		return fmt.Errorf("the operator sees no bridge device after the restart")
	}

	previous := make(map[string]string, len(before))
	for _, device := range before {
		previous[device.DeviceID] = device.Ed25519
	}
	for _, device := range after {
		identity, seen := previous[device.DeviceID]
		if !seen {
			return fmt.Errorf("the operator sees a new bridge device %s after the restart", device.DeviceID)
		}
		if identity != device.Ed25519 {
			return fmt.Errorf("the bridge device %s changed its identity key after the restart: %s -> %s",
				device.DeviceID, shortKey(identity), shortKey(device.Ed25519))
		}
		delete(previous, device.DeviceID)
	}
	if len(previous) > 0 {
		return fmt.Errorf("the operator no longer sees the bridge device(s) %s after the restart", strings.Join(keysOf(previous), ", "))
	}
	return nil
}

func shortKey(key string) string {
	if len(key) <= 12 {
		return key
	}
	return key[:12] + "…"
}

func keysOf(devices map[string]string) []string {
	ids := make([]string, 0, len(devices))
	for deviceID := range devices {
		ids = append(ids, deviceID)
	}
	return ids
}
