package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type recorder struct {
	steps []string
}

func (r *recorder) record(captures []string) error {
	r.steps = append(r.steps, strings.Join(captures[1:], "|"))
	return nil
}

func testRegistry(rec *recorder) *Registry {
	registry := NewRegistry(func() any { return rec })
	registry.OnClose(func(any) {})
	must := func(err error) {
		if err != nil {
			panic(err)
		}
	}
	must(registry.Step(`^the bridge is started$`, func(_ context.Context, world any, captures []string) error {
		return world.(*recorder).record(captures)
	}))
	must(registry.Step(`^the operator sends the message "(.+)" into chat room (.+)$`, func(_ context.Context, world any, captures []string) error {
		return world.(*recorder).record(captures)
	}))
	must(registry.Step(`^the forge holds (\d+) chat requests$`, func(_ context.Context, world any, captures []string) error {
		return world.(*recorder).record(captures)
	}))
	return registry
}

func TestExecuteRunsBackgroundThenScenarioSteps(t *testing.T) {
	recorder := &recorder{}
	registry := testRegistry(recorder)
	feature := Feature{Background: []Step{{Text: "the bridge is started"}}}
	scenario := Scenario{Steps: []Step{{Text: `the operator sends the message "is the build green?" into chat room Chat`}}}

	if err := registry.Execute(context.Background(), recorder, feature, scenario, map[string]string{}); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	want := []string{"", "is the build green?|Chat"}
	if strings.Join(recorder.steps, ",") != strings.Join(want, ",") {
		t.Errorf("steps = %v, want %v", recorder.steps, want)
	}
}

func TestExecuteExpandsExampleValues(t *testing.T) {
	recorder := &recorder{}
	registry := testRegistry(recorder)
	scenario := Scenario{Steps: []Step{{Text: "the forge holds <chat_requests> chat requests"}}}

	if err := registry.Execute(context.Background(), recorder, Feature{}, scenario, map[string]string{"chat_requests": "0"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if len(recorder.steps) != 1 || recorder.steps[0] != "0" {
		t.Errorf("steps = %v, want the expanded example value", recorder.steps)
	}
}

func TestExecuteFailsOnMissingExampleValue(t *testing.T) {
	registry := testRegistry(&recorder{})
	scenario := Scenario{Steps: []Step{{Text: "the forge holds <chat_requests> chat requests"}}}

	err := registry.Execute(context.Background(), &recorder{}, Feature{}, scenario, map[string]string{})
	if err == nil || !strings.Contains(err.Error(), "<chat_requests>") {
		t.Fatalf("error = %v, want a missing example value", err)
	}
}

func TestExecuteFailsOnUnsupportedStep(t *testing.T) {
	registry := testRegistry(&recorder{})
	scenario := Scenario{Steps: []Step{{Text: "the operator does something unknown"}}}

	err := registry.Execute(context.Background(), &recorder{}, Feature{}, scenario, map[string]string{})
	if err == nil || !strings.Contains(err.Error(), "unsupported step") {
		t.Fatalf("error = %v, want an unsupported step", err)
	}
}

func TestExecuteReportsHandlerFailure(t *testing.T) {
	registry := NewRegistry(func() any { return nil })
	if err := registry.Step(`^the bridge is started$`, func(context.Context, any, []string) error {
		return os.ErrPermission
	}); err != nil {
		t.Fatal(err)
	}
	scenario := Scenario{Steps: []Step{{Text: "the bridge is started"}}}

	err := registry.Execute(context.Background(), nil, Feature{}, scenario, map[string]string{})
	if err == nil || !strings.Contains(err.Error(), "permission") {
		t.Fatalf("error = %v, want the handler failure", err)
	}
}

func TestLoadFeatureReadsParserIR(t *testing.T) {
	path := filepath.Join(t.TempDir(), "feature.json")
	ir := `{
	  "name": "Chat Channel Relay",
	  "background": [{"keyword": "Given", "text": "the bridge is started", "parameters": []}],
	  "scenarios": [{
	    "name": "Relay 1",
	    "steps": [{"keyword": "Then", "text": "the forge holds <n> chat requests", "parameters": ["n"]}],
	    "examples": [{"n": "1"}]
	  }]
	}`
	if err := os.WriteFile(path, []byte(ir), 0o644); err != nil {
		t.Fatal(err)
	}

	feature, err := LoadFeature(path)
	if err != nil {
		t.Fatalf("LoadFeature: %v", err)
	}
	if feature.Name != "Chat Channel Relay" || len(feature.Background) != 1 || len(feature.Scenarios) != 1 {
		t.Fatalf("feature = %+v", feature)
	}
	if feature.Scenarios[0].Examples[0]["n"] != "1" {
		t.Errorf("examples = %v", feature.Scenarios[0].Examples)
	}
}

func TestRunExecutesEveryExampleRow(t *testing.T) {
	registry := NewRegistry(func() any { return &recorder{} })
	registry.OnClose(func(any) {})
	executions := 0
	if err := registry.Step(`^the forge holds (\d+) chat requests$`, func(context.Context, any, []string) error {
		executions++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	feature := Feature{Scenarios: []Scenario{{
		Name:  "Operator Allowlist 1",
		Steps: []Step{{Text: "the forge holds <chat_requests> chat requests"}},
		Examples: []map[string]string{
			{"chat_requests": "1"},
			{"chat_requests": "0"},
		},
	}}}

	registry.Run(t, feature)

	if executions != 2 {
		t.Errorf("executions = %d, want one per example row", executions)
	}
}

func TestStepRejectsBadPattern(t *testing.T) {
	registry := NewRegistry(func() any { return nil })
	if err := registry.Step(`^the bridge is (started$`, func(context.Context, any, []string) error { return nil }); err == nil {
		t.Fatal("Step accepted an invalid pattern")
	}
}
