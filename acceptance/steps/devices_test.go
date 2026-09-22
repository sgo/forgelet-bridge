package steps

import (
	"reflect"
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

// TestSameDeviceRejectsADifferentListing checks every way a restart can leave
// the operator looking at a different device: a new one appears, one is lost,
// an identity key changes, or one of the two listings is empty.
func TestSameDeviceRejectsADifferentListing(t *testing.T) {
	one := []fixtures.DeviceKey{{DeviceID: "FORGELETBRIDGE", Ed25519: "key-one"}}
	two := append(append([]fixtures.DeviceKey(nil), one...), fixtures.DeviceKey{DeviceID: "FORGELETBRIDGE2", Ed25519: "key-two"})
	renamed := []fixtures.DeviceKey{{DeviceID: "FORGELETBRIDGE", Ed25519: "key-two"}}

	cases := map[string]struct {
		before []fixtures.DeviceKey
		after  []fixtures.DeviceKey
		want   string
	}{
		"a new device appears":         {before: one, after: two, want: "new bridge device"},
		"one device is lost":           {before: two, after: one, want: "no longer sees"},
		"the identity key changes":     {before: one, after: renamed, want: "identity key"},
		"no device after the restart":  {before: one, want: "no bridge device after"},
		"no device before the restart": {after: one, want: "no bridge device before"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := sameDevice(tc.before, tc.after)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("sameDevice = %v, want it to report %q", err, tc.want)
			}
		})
	}
}

func TestKeysOfListsDeviceIDsInAStableOrder(t *testing.T) {
	devices := map[string]string{"DEVICE-B": "key-b", "DEVICE-A": "key-a", "DEVICE-C": "key-c"}

	if got, want := keysOf(devices), []string{"DEVICE-A", "DEVICE-B", "DEVICE-C"}; !reflect.DeepEqual(got, want) {
		t.Errorf("keysOf = %v, want %v", got, want)
	}
}

func TestShortKeyShortensOnlyAKeyOverTwelveCharacters(t *testing.T) {
	cases := map[string]string{
		"key-one":       "key-one",
		"0123456789ab":  "0123456789ab",
		"0123456789abc": "0123456789ab…",
	}
	for key, want := range cases {
		if got := shortKey(key); got != want {
			t.Errorf("shortKey(%q) = %q, want %q", key, got, want)
		}
	}
}
