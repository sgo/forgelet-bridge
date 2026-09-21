package steps

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unclebob/forgelet-bridge/acceptance/fixtures"
	"github.com/unclebob/forgelet-bridge/acceptance/runtime"
)

// TestEveryParsedStepHasAHandler guards the vocabulary: after an acceptance
// run has parsed the features, every step in the IR must be handled by exactly
// the wording the features use.
func TestEveryParsedStepHasAHandler(t *testing.T) {
	irDir := filepath.Join(fixtures.ProjectRoot(), "build", "acceptance", "ir")
	entries, err := os.ReadDir(irDir)
	if err != nil {
		t.Skipf("no parsed acceptance IR yet (%v); run scripts/acceptance.sh", err)
	}

	registry := Registry()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		feature, err := runtime.LoadFeature(filepath.Join(irDir, entry.Name()))
		if err != nil {
			t.Fatalf("load %s: %v", entry.Name(), err)
		}
		for _, step := range feature.Background {
			assertHandled(t, registry, entry.Name(), step.Text)
		}
		for _, scenario := range feature.Scenarios {
			for _, step := range scenario.Steps {
				assertHandled(t, registry, entry.Name(), step.Text)
			}
		}
	}
}

// assertHandled fails when no handler claims a step text. Example placeholders
// are filled with a stand-in value, because the handler has to match the
// expanded wording.
func assertHandled(t *testing.T, registry *runtime.Registry, file, text string) {
	t.Helper()
	expanded, err := runtime.Expand(text, exampleFor(text))
	if err != nil {
		t.Fatalf("%s: %s: %v", file, text, err)
	}
	if _, err := registry.Match(expanded); err != nil {
		t.Errorf("%s: %s: %v", file, text, err)
	}
}

func exampleFor(text string) map[string]string {
	values := map[string]string{}
	for {
		open := strings.Index(text, "<")
		if open < 0 {
			return values
		}
		text = text[open+1:]
		close := strings.Index(text, ">")
		if close < 0 {
			return values
		}
		values[text[:close]] = "1"
		text = text[close+1:]
	}
}
