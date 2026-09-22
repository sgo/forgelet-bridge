package steps

import (
	"strings"
	"testing"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
)

func TestSameDeviceAcceptsTheSameDeviceAfterARestart(t *testing.T) {
	before := []fixtures.DeviceKey{{DeviceID: "FORGELETBRIDGE", Ed25519: "key-one"}}
	after := []fixtures.DeviceKey{{DeviceID: "FORGELETBRIDGE", Ed25519: "key-one"}}

	if err := sameDevice(before, after); err != nil {
		t.Errorf("sameDevice = %v, want the restart to keep the same device", err)
	}
}

func TestSameDeviceRejectsANewDeviceAppearingAfterTheRestart(t *testing.T) {
	before := []fixtures.DeviceKey{{DeviceID: "FORGELETBRIDGE", Ed25519: "key-one"}}
	after := []fixtures.DeviceKey{
		{DeviceID: "FORGELETBRIDGE", Ed25519: "key-one"},
		{DeviceID: "FORGELETBRIDGE2", Ed25519: "key-two"},
	}

	err := sameDevice(before, after)
	if err == nil || !strings.Contains(err.Error(), "new bridge device") {
		t.Errorf("sameDevice = %v, want a new device to be reported", err)
	}
}

func TestSameDeviceRejectsAChangedIdentityKey(t *testing.T) {
	before := []fixtures.DeviceKey{{DeviceID: "FORGELETBRIDGE", Ed25519: "key-one"}}
	after := []fixtures.DeviceKey{{DeviceID: "FORGELETBRIDGE", Ed25519: "key-two"}}

	err := sameDevice(before, after)
	if err == nil || !strings.Contains(err.Error(), "identity key") {
		t.Errorf("sameDevice = %v, want a changed identity key to be reported", err)
	}
}

func TestSameDeviceRejectsAMissingDevice(t *testing.T) {
	before := []fixtures.DeviceKey{{DeviceID: "FORGELETBRIDGE", Ed25519: "key-one"}}

	if err := sameDevice(before, nil); err == nil || !strings.Contains(err.Error(), "no bridge device after") {
		t.Errorf("sameDevice = %v, want a missing device to be reported", err)
	}
}

func TestSameDeviceRejectsHavingSeenNoDeviceBefore(t *testing.T) {
	after := []fixtures.DeviceKey{{DeviceID: "FORGELETBRIDGE", Ed25519: "key-one"}}

	if err := sameDevice(nil, after); err == nil || !strings.Contains(err.Error(), "no bridge device before") {
		t.Errorf("sameDevice = %v, want an empty starting point to be reported", err)
	}
}
