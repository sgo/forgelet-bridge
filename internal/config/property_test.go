//go:build property

package config

import (
	"math/rand"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/quick"
)

// TestPropertyDisplayNameIsTheConfiguredName checks the rule the operator
// relies on to tell forges apart: a forge the configuration names shows up
// under exactly that name, however the name is padded in the file.
func TestPropertyDisplayNameIsTheConfiguredName(t *testing.T) {
	property := func(root, name string) bool {
		if strings.TrimSpace(name) == "" {
			return true
		}
		return Forge{Root: root, Name: name}.DisplayName() == strings.TrimSpace(name)
	}
	if err := quick.Check(property, nil); err != nil {
		t.Error(err)
	}
}

// TestPropertyDisplayNameFallsBackToTheFolder checks that a forge the
// configuration does not name still has a name the operator can look for.
func TestPropertyDisplayNameFallsBackToTheFolder(t *testing.T) {
	property := func(root string) bool {
		got := Forge{Root: root}.DisplayName()
		return got != "" && got == filepath.Base(strings.TrimRight(root, string(filepath.Separator)))
	}
	if err := quick.Check(property, nil); err != nil {
		t.Error(err)
	}
}

// TestPropertyForgeNameFindsEachConfiguredForge checks that the name the
// bridge provisions a forge under is the name the configuration gave that
// forge, wherever in the list it sits.
func TestPropertyForgeNameFindsEachConfiguredForge(t *testing.T) {
	property := func(forges []Forge) bool {
		if !distinctRoots(forges) {
			return true // a configuration with a repeated root is rejected instead
		}
		cfg := Config{Forges: forges}
		for _, forge := range forges {
			if cfg.ForgeName(forge.Root) != forge.DisplayName() {
				return false
			}
		}
		return true
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 300,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randomForges(rnd))
		},
	}); err != nil {
		t.Error(err)
	}
}

// TestPropertyForgeNameFallsBackToTheFolder checks the same fallback through
// the configuration: a root the configuration does not list is named by its
// folder, never left nameless.
func TestPropertyForgeNameFallsBackToTheFolder(t *testing.T) {
	property := func(root string, forges []Forge) bool {
		cfg := Config{Forges: forges}
		for _, forge := range forges {
			if forge.Root == root {
				return true // a listed root is the previous property's business
			}
		}
		return cfg.ForgeName(root) == folderName(root)
	}
	if err := quick.Check(property, &quick.Config{
		MaxCount: 300,
		Values: func(values []reflect.Value, rnd *rand.Rand) {
			values[0] = reflect.ValueOf(randomRoot(rnd))
			values[1] = reflect.ValueOf(randomForges(rnd))
		},
	}); err != nil {
		t.Error(err)
	}
}

// distinctRoots reports whether every forge in a list lives at its own root.
func distinctRoots(forges []Forge) bool {
	seen := map[string]bool{}
	for _, forge := range forges {
		if seen[forge.Root] {
			return false
		}
		seen[forge.Root] = true
	}
	return true
}

// randomForges is a configuration's forge list: roots that differ, each named
// or left to its folder.
func randomForges(rnd *rand.Rand) []Forge {
	roots := []string{"/forges/sgo", "/forges/forgelet", "/forges/forge-a/", "relative/forge"}
	names := []string{"", "Saibill", "Forgelet", "  padded  "}

	rnd.Shuffle(len(roots), func(i, j int) { roots[i], roots[j] = roots[j], roots[i] })
	forges := make([]Forge, 0, len(roots))
	for _, root := range roots[:rnd.Intn(len(roots)+1)] {
		forges = append(forges, Forge{Root: root, Name: names[rnd.Intn(len(names))]})
	}
	if len(forges) == 0 {
		return nil
	}
	return forges
}

func randomRoot(rnd *rand.Rand) string {
	roots := []string{"", "/", "/forges/sgo", "/forges/saibill/", "forgelet", "../forge-a"}
	return roots[rnd.Intn(len(roots))]
}
