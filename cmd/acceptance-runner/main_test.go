package main

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClassifySeparatesKilledFromInfrastructure(t *testing.T) {
	cases := []struct {
		name    string
		output  string
		err     error
		context error
		want    string
	}{
		{"passing tests", "ok  \tgenerated\t0.5s\n", nil, nil, outcomeSuccess},
		{"failing tests", "--- FAIL: TestChat (0.1s)\nFAIL\n", os.ErrInvalid, nil, outcomeFailure},
		{"build failure", "# generated\n[build failed]\n", os.ErrInvalid, nil, outcomeInfrastructureError},
		{"unknown failure", "", os.ErrInvalid, nil, outcomeInfrastructureError},
		{"timeout", "", os.ErrInvalid, os.ErrDeadlineExceeded, outcomeInfrastructureError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classify(tc.output, tc.err, tc.context); got != tc.want {
				t.Errorf("classify = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestServeRunsTheSameGeneratedTestAgainstDifferentIR(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module generatedprobe\n\ngo 1.24\n")
	writeFile(t, filepath.Join(dir, "acceptance_test.go"), `package generatedprobe

import (
	"os"
	"strings"
	"testing"
)

func TestGeneratedFeature(t *testing.T) {
	data, err := os.ReadFile(os.Getenv("FORGELET_ACCEPTANCE_IR"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "mutated") {
		t.Fatal("the mutated example value was detected")
	}
}
`)
	passingIR := filepath.Join(dir, "passing.json")
	writeFile(t, passingIR, `{"name":"probe"}`)
	mutatedIR := filepath.Join(dir, "mutated.json")
	writeFile(t, mutatedIR, `{"name":"mutated"}`)

	input := strings.NewReader(
		`{"id":"m1","feature_json":"` + passingIR + `","generated_dir":"` + dir + `","work_dir":"` + dir + `","timeout":"60s"}` + "\n" +
			`{"id":"m2","feature_json":"` + mutatedIR + `","generated_dir":"` + dir + `","work_dir":"` + dir + `","timeout":"60s"}` + "\n")
	output := &strings.Builder{}

	if err := serveReader(input, output); err != nil {
		t.Fatalf("serve: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("responses = %d, want one per job:\n%s", len(lines), output.String())
	}
	if !strings.Contains(lines[0], `"outcome":"`+outcomeSuccess+`"`) {
		t.Errorf("first response = %s, want the unmutated IR to pass", lines[0])
	}
	if !strings.Contains(lines[1], `"outcome":"`+outcomeFailure+`"`) {
		t.Errorf("second response = %s, want the mutated IR to fail the test", lines[1])
	}
	if !strings.Contains(lines[1], `"id":"m2"`) {
		t.Errorf("second response = %s, want the job id echoed", lines[1])
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunNeedsAJobOrWorkerMode(t *testing.T) {
	if code := run(options{}, strings.NewReader(""), io.Discard); code != 2 {
		t.Errorf("exit code = %d, want the usage error", code)
	}
}

func TestRunAnswersWorkerJobs(t *testing.T) {
	input := strings.NewReader("{\"id\":\"m1\"}\n")
	output := &strings.Builder{}

	if code := run(options{worker: true}, input, output); code != 0 {
		t.Fatalf("exit code = %d, want the worker to end cleanly", code)
	}
	if !strings.Contains(output.String(), `"outcome":"`+outcomeInfrastructureError+`"`) {
		t.Errorf("response = %s, want the malformed job reported", output.String())
	}
}

func TestRunReportsAFailedJob(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module failingprobe\n\ngo 1.24\n")
	writeFile(t, filepath.Join(dir, "acceptance_test.go"), `package failingprobe

import "testing"

func TestGeneratedFeature(t *testing.T) {
	t.Fatal("the generated feature failed")
}
`)
	ir := filepath.Join(dir, "feature.json")
	writeFile(t, ir, `{"name":"probe"}`)
	output := &strings.Builder{}

	code := run(options{featureJSON: ir, generatedDir: dir, workDir: dir, timeout: time.Minute}, strings.NewReader(""), output)

	if code != 1 {
		t.Errorf("exit code = %d, want a failed run to report failure", code)
	}
	if !strings.Contains(output.String(), "--- FAIL") {
		t.Errorf("output = %q, want the test output", output.String())
	}
}

func TestParseOptionsRejectsAnUnknownFlag(t *testing.T) {
	_, err := parseOptions([]string{"--nope"})
	if err == nil {
		t.Fatal("parseOptions accepted an unknown flag")
	}
	if code := usageExit(err); code != 2 {
		t.Errorf("exit code = %d, want an unusable command line to report the usage error", code)
	}
}

func TestUsageExitAsksAreNotFailures(t *testing.T) {
	if code := usageExit(flag.ErrHelp); code != 0 {
		t.Errorf("exit code = %d, want asking for the usage text to succeed", code)
	}
}

func TestParseOptionsReadsTheOneShotJob(t *testing.T) {
	parsed, err := parseOptions([]string{"--feature-json", "ir.json", "--generated-dir", "generated", "--work-dir", "work", "--timeout", "30s"})
	if err != nil {
		t.Fatalf("parseOptions: %v", err)
	}
	want := options{featureJSON: "ir.json", generatedDir: "generated", workDir: "work", timeout: 30 * time.Second}
	if parsed != want {
		t.Errorf("options = %+v, want %+v", parsed, want)
	}
}
