// Package rules installs the prompt rules the bridge's rooms rely on into a
// forge's own constitution and the packs new projects are scaffolded from.
//
// A rule is one marked block: the installer owns the block and nothing else, so
// running it again is safe, a forge that has changed its own constitution keeps
// its own wording, and what drifted shows up as a diff.
package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// opening and closing mark a block this installer owns.
const (
	markerPrefix = "<!-- bridge-rule: "
	markerEnd    = "<!-- end: "
)

// Rule is one prompt rule: the block it installs, and the subject it belongs to.
type Rule struct {
	Subject string
	Block   string
}

// Load reads every rule in a directory of marked blocks, in name order.
func Load(dir string) ([]Rule, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read the rules in %s: %w", dir, err)
	}
	var loaded []Rule
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		rule, err := Parse(string(data))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		loaded = append(loaded, rule)
	}
	sort.Slice(loaded, func(i, j int) bool { return loaded[i].Subject < loaded[j].Subject })
	return loaded, nil
}

// Parse reads one marked block.
func Parse(text string) (Rule, error) {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, markerPrefix) {
		return Rule{}, fmt.Errorf("a rule starts with %q", markerPrefix)
	}
	subject, _, found := strings.Cut(strings.TrimPrefix(trimmed, markerPrefix), " -->")
	if !found || strings.TrimSpace(subject) == "" {
		return Rule{}, fmt.Errorf("a rule names its subject in %q", markerPrefix)
	}
	subject = strings.TrimSpace(subject)
	if !strings.HasSuffix(trimmed, markerEnd+subject+" -->") {
		return Rule{}, fmt.Errorf("the rule %s ends with %q", subject, markerEnd+subject+" -->")
	}
	return Rule{Subject: subject, Block: trimmed + "\n"}, nil
}

// Target is one place a rule is installed: a constitution's articles.
type Target struct {
	// Path is the articles directory.
	Path string
	// Description names the target the way the report should read: the forge's
	// own constitution, or the pack a project is scaffolded from.
	Description string
}

// Report is what an install changed, found current, and left alone.
type Report struct {
	Changed   []string
	Current   []string
	LeftAlone []string
}

// add files one result in the list it belongs to.
func (r *Report) add(result installResult) {
	line := fmt.Sprintf("%s in %s", result.subject, result.target)
	switch result.outcome {
	case outcomeChanged:
		r.Changed = append(r.Changed, line)
	case outcomeCurrent:
		r.Current = append(r.Current, line)
	default:
		r.LeftAlone = append(r.LeftAlone, result.detail)
	}
}

// String is the report the operator reads.
func (r Report) String() string {
	var lines []string
	for _, line := range r.Changed {
		lines = append(lines, "changed "+line)
	}
	for _, line := range r.Current {
		lines = append(lines, "already current "+line)
	}
	for _, line := range r.LeftAlone {
		lines = append(lines, "left alone "+line)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// Install puts every rule into the forge's own constitution and into every pack
// the forge scaffolds projects from. A project's own tracked copy is never
// touched: a project picks a rule up the way it picks up any other change,
// through its specifier, so a forge whose projects are mid-card is safe to
// install into.
func Install(forgeRoot string, loaded []Rule) (Report, error) {
	var report Report
	targets, err := targets(forgeRoot)
	if err != nil {
		return report, err
	}
	for _, rule := range loaded {
		for _, target := range targets {
			result, err := installInto(target, rule)
			if err != nil {
				return report, err
			}
			report.add(result)
		}
	}
	projects := filepath.Join(forgeRoot, "projects")
	report.LeftAlone = append(report.LeftAlone,
		fmt.Sprintf("%s: a project picks a new rule up through its specifier", projects))
	return report, nil
}

// targets are the places one forge's rules live: its own constitution first,
// then the packs.
func targets(forgeRoot string) ([]Target, error) {
	found := []Target{{
		Path:        filepath.Join(forgeRoot, "swarmforge", "constitution", "articles"),
		Description: "the forge's own constitution",
	}}
	packs := filepath.Join(forgeRoot, "packs")
	entries, err := os.ReadDir(packs)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read the packs in %s: %w", packs, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		found = append(found, Target{
			Path:        filepath.Join(packs, entry.Name(), "swarmforge", "constitution", "articles"),
			Description: "the pack " + entry.Name(),
		})
	}
	return found, nil
}

// outcome is what an install did with one rule in one target.
type outcome int

const (
	outcomeChanged outcome = iota
	outcomeCurrent
	outcomeLeftAlone
)

// installResult is one rule's fate in one target.
type installResult struct {
	subject string
	target  string
	outcome outcome
	// detail is what the report says about a file the installer left alone.
	detail string
}

// installInto writes one rule into one target, and says what it did.
func installInto(target Target, rule Rule) (installResult, error) {
	result := installResult{subject: rule.Subject, target: target.Description}
	path := filepath.Join(target.Path, rule.Subject+".prompt")
	existing, err := os.ReadFile(path)
	switch {
	case err == nil && sameBlock(string(existing), rule.Block):
		result.outcome = outcomeCurrent
		return result, nil
	case err == nil && !owned(string(existing), rule.Subject):
		// A file the installer did not write is the forge's own wording, and
		// keeps it.
		result.outcome = outcomeLeftAlone
		result.detail = fmt.Sprintf("%s: %s does not carry this installer's marker", path, target.Description)
		return result, nil
	}
	if err != nil && !os.IsNotExist(err) {
		return result, err
	}
	if err := os.MkdirAll(target.Path, 0o755); err != nil {
		return result, err
	}
	if err := os.WriteFile(path, []byte(rule.Block), 0o644); err != nil {
		return result, err
	}
	result.outcome = outcomeChanged
	return result, nil
}

// sameBlock reports whether a file already holds the rule, word for word.
func sameBlock(existing, block string) bool {
	return strings.TrimSpace(existing) == strings.TrimSpace(block)
}

// owned reports whether a file is this installer's block for a subject: it
// opens and closes with the markers it writes.
func owned(text, subject string) bool {
	trimmed := strings.TrimSpace(text)
	return strings.HasPrefix(trimmed, markerPrefix+subject+" -->") &&
		strings.HasSuffix(trimmed, markerEnd+subject+" -->")
}
