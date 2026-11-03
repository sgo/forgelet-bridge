package pipeline

import (
	"strings"
	"testing"
)

func TestVerifySingleAgentGoProject(t *testing.T) {
	module := "module github.com/unclebob/forgelet-bridge\n"
	tests := []struct {
		name     string
		marker   string
		language string
		module   string
		ok       bool
	}{
		{name: "valid", marker: "forgelet-bridge\n", language: "Go\n", module: module, ok: true},
		{name: "not marked", marker: "other\n", language: "Go\n", module: module},
		{name: "wrong language", marker: "forgelet-bridge\n", language: "Java\n", module: module},
		{name: "wrong module", marker: "forgelet-bridge\n", language: "Go\n", module: "module example.test\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := VerifySingleAgentGoProject(test.marker, test.language, test.module)
			if test.ok && err != nil {
				t.Fatal(err)
			}
			if !test.ok && err == nil {
				t.Fatal("expected the pipeline declaration to be refused")
			}
		})
	}
}

func TestVerifyProjectStageKeepsTheCommandAndLogTogether(t *testing.T) {
	stages := `{:stages [{:name "code" :runs [["scripts/stage-gate.sh" "code"]]}
                       {:name "deploy" :runs [["scripts/stage-gate.sh" "deploy"]]}]}`
	gate := `case "$stage" in
  code)
    run scripts/test.sh
    ;;
  deploy)
    run scripts/build.sh
    run scripts/deploy-check.sh
    ;;
esac
`
	if err := VerifyProjectStage(stages, gate, "code", "scripts/test.sh", "build/stages/code.log"); err != nil {
		t.Fatal(err)
	}
	if err := VerifyProjectStage(stages, gate, "deploy", "scripts/build.sh", "build/stages/deploy.log"); err != nil {
		t.Fatal(err)
	}
	firstCase := `
  code)
    run scripts/test.sh
    ;;
esac
`
	if err := VerifyProjectStage(stages, firstCase, "code", "scripts/test.sh", "build/stages/code.log"); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		stages  string
		gate    string
		stage   string
		command string
		log     string
		want    string
	}{
		{name: "missing stage", stages: stages, gate: gate, stage: "qa", command: "scripts/acceptance.sh", log: "build/stages/qa.log", want: "no qa stage"},
		{name: "missing gate case", stages: stages, gate: "case \"$stage\" in\nesac\n", stage: "code", command: "scripts/test.sh", log: "build/stages/code.log", want: "no code case"},
		{name: "wrong command", stages: stages, gate: gate, stage: "code", command: "scripts/acceptance.sh", log: "build/stages/code.log", want: "does not run"},
		{name: "wrong log", stages: stages, gate: gate, stage: "code", command: "scripts/test.sh", log: "target/code.log", want: "log ="},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := VerifyProjectStage(test.stages, test.gate, test.stage, test.command, test.log)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want it to contain %q", err, test.want)
			}
		})
	}
}
