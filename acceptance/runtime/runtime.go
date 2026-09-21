// Package runtime runs generated acceptance tests: it expands a feature's JSON
// IR into scenario executions and routes every step to a project step handler.
package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
)

// IRPathEnv lets a runner point the generated entry points at another JSON IR,
// which is how acceptance mutation runs the same tests against mutated values.
const IRPathEnv = "FORGELET_ACCEPTANCE_IR"

// Feature is the parser's JSON IR for one feature file.
type Feature struct {
	Name       string     `json:"name"`
	Background []Step     `json:"background"`
	Scenarios  []Scenario `json:"scenarios"`
}

// Scenario is one scenario with its example rows.
type Scenario struct {
	Name     string              `json:"name"`
	Steps    []Step              `json:"steps"`
	Examples []map[string]string `json:"examples"`
}

// Step is one Gherkin step.
type Step struct {
	Keyword    string   `json:"keyword"`
	Text       string   `json:"text"`
	Parameters []string `json:"parameters"`
}

// LoadFeature reads parser JSON IR.
func LoadFeature(path string) (Feature, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Feature{}, err
	}
	var feature Feature
	if err := json.Unmarshal(data, &feature); err != nil {
		return Feature{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return feature, nil
}

// Handler answers one step. Captures are the regular expression groups the
// step pattern matched.
type Handler func(ctx context.Context, world any, captures []string) error

type registered struct {
	pattern *regexp.Regexp
	handler Handler
}

// Registry holds the project's step handlers and builds a fresh world for
// every scenario execution.
type Registry struct {
	newWorld   func() any
	closeWorld func(any)
	handlers   []registered
}

// NewRegistry builds a registry whose worlds come from newWorld.
func NewRegistry(newWorld func() any) *Registry {
	return &Registry{newWorld: newWorld, closeWorld: func(any) {}}
}

// OnClose registers the teardown of a world.
func (r *Registry) OnClose(close func(any)) {
	r.closeWorld = close
}

// Step registers a step handler for a regular expression.
func (r *Registry) Step(pattern string, handler Handler) error {
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("step pattern %q: %w", pattern, err)
	}
	r.handlers = append(r.handlers, registered{pattern: compiled, handler: handler})
	return nil
}

// Run executes every scenario execution in the feature as a subtest.
func (r *Registry) Run(t *testing.T, feature Feature) {
	t.Helper()
	for _, scenario := range feature.Scenarios {
		for index, example := range examplesOf(scenario) {
			name := fmt.Sprintf("%s/example_%d", scenario.Name, index+1)
			t.Run(name, func(t *testing.T) {
				world := r.newWorld()
				defer r.closeWorld(world)
				if err := r.Execute(context.Background(), world, feature, scenario, example); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

// Execute runs the background and the scenario's steps against one world.
func (r *Registry) Execute(ctx context.Context, world any, feature Feature, scenario Scenario, example map[string]string) error {
	steps := make([]Step, 0, len(feature.Background)+len(scenario.Steps))
	steps = append(steps, feature.Background...)
	steps = append(steps, scenario.Steps...)

	for _, step := range steps {
		text, err := Expand(step.Text, example)
		if err != nil {
			return fmt.Errorf("%s %s: %w", step.Keyword, step.Text, err)
		}
		handler, captures, found := r.match(text)
		if !found {
			return fmt.Errorf("unsupported step: %s %s", step.Keyword, text)
		}
		if err := handler(ctx, world, captures); err != nil {
			return fmt.Errorf("%s %s: %w", step.Keyword, text, err)
		}
	}
	return nil
}

func (r *Registry) match(text string) (Handler, []string, bool) {
	for _, entry := range r.handlers {
		if captures := entry.pattern.FindStringSubmatch(text); captures != nil {
			return entry.handler, captures, true
		}
	}
	return nil, nil, false
}

// Match reports whether a step text is handled, and which captures it matched
// with. A missing handler is an error naming the step.
func (r *Registry) Match(text string) ([]string, error) {
	if _, captures, found := r.match(text); found {
		return captures, nil
	}
	return nil, fmt.Errorf("unsupported step: %s", text)
}

// Expand fills a step's placeholders from one example row.
func Expand(text string, example map[string]string) (string, error) {
	var expanded strings.Builder
	remaining := text
	for {
		open := strings.Index(remaining, "<")
		if open < 0 {
			expanded.WriteString(remaining)
			return expanded.String(), nil
		}
		close := strings.Index(remaining[open:], ">")
		if close < 0 {
			expanded.WriteString(remaining)
			return expanded.String(), nil
		}
		expanded.WriteString(remaining[:open])
		name := remaining[open+1 : open+close]
		value, ok := example[name]
		if !ok {
			return "", fmt.Errorf("example row has no value for <%s>", name)
		}
		expanded.WriteString(value)
		remaining = remaining[open+close+1:]
	}
}

func examplesOf(scenario Scenario) []map[string]string {
	if len(scenario.Examples) == 0 {
		return []map[string]string{{}}
	}
	return scenario.Examples
}
