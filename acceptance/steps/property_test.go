//go:build property

package steps

import (
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
)

// TestPropertySameDeviceAcceptsTheListingItWasGiven checks the restart rule
// against itself: the device listing the operator saw before the restart always
// passes as the listing after it. An empty listing is not a device listing, so
// it is out of the property's domain — the step reports it instead.
func TestPropertySameDeviceAcceptsTheListingItWasGiven(t *testing.T) {
	property := func(before []fixtures.DeviceKey) bool {
		if len(before) == 0 {
			return true
		}
		return sameDevice(before, before) == nil
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 300,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randomDevices(rnd))
		},
	}); err != nil {
		t.Error(err)
	}
}

// TestPropertySameDeviceIgnoresTheOrderOfTheListing checks that the verdict
// does not depend on the order the homeserver happened to answer in.
func TestPropertySameDeviceIgnoresTheOrderOfTheListing(t *testing.T) {
	property := func(before []fixtures.DeviceKey, seed int64) bool {
		shuffled := append([]fixtures.DeviceKey(nil), before...)
		rnd := rand.New(rand.NewSource(seed))
		rnd.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })

		return (sameDevice(before, shuffled) == nil) == (sameDevice(before, before) == nil)
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 300,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randomDevices(rnd))
			values[1] = reflect.ValueOf(rnd.Int63())
		},
	}); err != nil {
		t.Error(err)
	}
}

// TestPropertySameDeviceRejectsADifferentListing checks the rule bites: a
// listing that gains a device, loses one, or renames an identity key is never
// the same as the one before the restart.
func TestPropertySameDeviceRejectsADifferentListing(t *testing.T) {
	property := func(before []fixtures.DeviceKey) bool {
		if len(before) == 0 {
			return true
		}

		gained := append(append([]fixtures.DeviceKey(nil), before...), fixtures.DeviceKey{DeviceID: "FORGELETBRIDGE-NEW", Ed25519: "key-new"})
		lost := before[:len(before)-1]
		renamed := append([]fixtures.DeviceKey(nil), before...)
		renamed[0].Ed25519 += "-other"

		return sameDevice(before, gained) != nil &&
			sameDevice(before, lost) != nil &&
			sameDevice(before, renamed) != nil
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 300,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randomDevices(rnd))
		},
	}); err != nil {
		t.Error(err)
	}
}

// randomDevices builds the device listing a client would answer with: distinct
// device ids, sorted the way the fixture sorts them.
func randomDevices(rnd *rand.Rand) []fixtures.DeviceKey {
	ids := []string{"FORGELETBRIDGE", "FORGELETBRIDGE2", "AAAABBBB", "device with spaces"}
	count := rnd.Intn(4)
	devices := make([]fixtures.DeviceKey, 0, count)
	for _, deviceID := range ids[:count] {
		devices = append(devices, fixtures.DeviceKey{
			DeviceID:   deviceID,
			Ed25519:    "key-" + deviceID,
			Curve25519: "curve-" + deviceID,
		})
	}
	if len(devices) == 0 {
		return nil
	}
	return devices
}
