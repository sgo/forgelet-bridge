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
			t.Run(exampleName(scenario.Name, index), func(t *testing.T) {
				world := r.newWorld()
				defer r.closeWorld(world)
				if err := r.Execute(context.Background(), world, feature, scenario, example); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

// exampleName names one scenario execution: the scenario, and which of its
// example rows this is, counted from one the way a reader counts them.
func exampleName(scenarioName string, index int) string {
	return fmt.Sprintf("%s/example_%d", scenarioName, index+1)
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

// Expand fills a step's placeholders from one example row. The name between
// the angle brackets runs up to the next ">"; a step with no name to fill
// ("<>") is looked up like any other, so it fails loudly rather than passing
// through unexpanded.
func Expand(text string, example map[string]string) (string, error) {
	var expanded strings.Builder
	remaining := text
	for {
		open := strings.Index(remaining, "<")
		if open < 0 {
			expanded.WriteString(remaining)
			return expanded.String(), nil
		}
		nameLength := strings.Index(remaining[open+1:], ">")
		if nameLength < 0 {
			expanded.WriteString(remaining)
			return expanded.String(), nil
		}
		expanded.WriteString(remaining[:open])
		name := remaining[open+1 : open+1+nameLength]
		value, ok := example[name]
		if !ok {
			return "", fmt.Errorf("example row has no value for <%s>", name)
		}
		expanded.WriteString(value)
		remaining = remaining[open+1+nameLength+1:]
	}
}

func examplesOf(scenario Scenario) []map[string]string {
	if len(scenario.Examples) == 0 {
		return []map[string]string{{}}
	}
	return scenario.Examples
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-21T23:32:44+02:00","module_hash":"5766cf02d463aa72119f9de4e5d775ada90b6aa0f48b0703607e78ef6a8c8613","functions":[{"id":"func/LoadFeature","name":"LoadFeature","line":41,"end_line":51,"hash":"7c853e14a70a30b4f6874b3313c136a98097919a9962765bc9767f19006788c6"},{"id":"func/NewRegistry","name":"NewRegistry","line":71,"end_line":73,"hash":"280e607208ebb4baa29712f0a0a213fba606ec8c707a7113ca55335ca6f35710"},{"id":"func/Registry.OnClose","name":"Registry.OnClose","line":76,"end_line":78,"hash":"65b515b7f8123da4cf10c14ec13f842da838c9e5a127b1ebc90972923dc1c30c"},{"id":"func/Registry.Step","name":"Registry.Step","line":81,"end_line":88,"hash":"1e27eb95618d3e41d849c3368101de1a4bc028e8fd58bdf17afc6f726e022c76"},{"id":"func/Registry.Run","name":"Registry.Run","line":91,"end_line":104,"hash":"15ff8c1d272f64daa320a1db669c40c7297b66928e3d1ca0f9bf1b3d04c5cd8f"},{"id":"func/exampleName","name":"exampleName","line":108,"end_line":110,"hash":"e26d551186eaab56939b68712ec76128053e04701793ce43e7981b82b28ecfd9"},{"id":"func/Registry.Execute","name":"Registry.Execute","line":113,"end_line":132,"hash":"d541659625529b035dc74c60b3f08d787ecdad280b43ae56b6cb93b3b7bd4fa5"},{"id":"func/Registry.match","name":"Registry.match","line":134,"end_line":141,"hash":"4edf9b9f565a795bfa202fb7f4167e62c307d7b129769ea088d0e32ccb345410"},{"id":"func/Registry.Match","name":"Registry.Match","line":145,"end_line":150,"hash":"0e49e58e1d7a9296270e4ebedd0e044e6ad45eb1d0f51245c0ad7c0785e29ea5"},{"id":"func/Expand","name":"Expand","line":156,"end_line":179,"hash":"862839279b6bc4074c765c582ec1a4b7aa7b1dd95374eac0331e7a1848f15d23"},{"id":"func/examplesOf","name":"examplesOf","line":181,"end_line":186,"hash":"2efcbc3398a9d9bd0df87a62f076b7ba9f4dc702f89a79570efa537a3605a0a8"}]}
// mutate4go-manifest-end
