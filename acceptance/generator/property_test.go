//go:build property

package generator

import (
	"regexp"
	"strings"
	"testing"
	"testing/quick"
)

var (
	goTestName   = regexp.MustCompile(`^Test[A-Za-z0-9]*$`)
	metadataName = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*\.json$`)
	asciiWord    = "abcdefghijklmnopqrstuvwxyz0123456789"
)

// TestPropertyTestNameIsAGoTestName checks the name the generated entry point
// gets: whatever the feature is called, it is a Go test function name.
func TestPropertyTestNameIsAGoTestName(t *testing.T) {
	property := func(featureName string) bool {
		return goTestName.MatchString(testName(featureName))
	}
	if err := quick.Check(property, nil); err != nil {
		t.Error(err)
	}
}

// TestPropertyMetadataNameIsAFileName checks the metadata file name the
// mutator reads: lowercased, hyphen separated, and always a json name.
func TestPropertyMetadataNameIsAFileName(t *testing.T) {
	property := func(featurePath string) bool {
		if !strings.ContainsAny(strings.ToLower(featurePath), asciiWord) {
			return true
		}
		return metadataName.MatchString(MetadataName(featurePath))
	}
	if err := quick.Check(property, nil); err != nil {
		t.Error(err)
	}
}

// TestPropertyMetadataNameIgnoresCase checks that a feature path's case cannot
// name a second metadata file, so one feature has one metadata file.
func TestPropertyMetadataNameIgnoresCase(t *testing.T) {
	property := func(featurePath string) bool {
		return MetadataName(featurePath) == MetadataName(strings.ToLower(featurePath))
	}
	if err := quick.Check(property, nil); err != nil {
		t.Error(err)
	}
}
