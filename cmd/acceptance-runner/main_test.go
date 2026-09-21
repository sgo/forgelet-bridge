package main

import (
	"context"
	"errors"
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

func TestRunNeedsBothPathsOfAOneShotJob(t *testing.T) {
	for name, parsed := range map[string]options{
		"no generated dir": {featureJSON: "ir.json"},
		"no feature json":  {generatedDir: "generated"},
	} {
		t.Run(name, func(t *testing.T) {
			if code := run(parsed, strings.NewReader(""), io.Discard); code != 2 {
				t.Errorf("exit code = %d, want the usage error", code)
			}
		})
	}
}

func TestRunAnswersASuccessfulOneShotJob(t *testing.T) {
	dir := passingGeneratedDir(t)
	ir := filepath.Join(dir, "feature.json")
	writeFile(t, ir, `{"name":"probe"}`)
	output := &strings.Builder{}

	code := run(options{featureJSON: ir, generatedDir: dir, workDir: dir, timeout: time.Minute}, strings.NewReader(""), output)

	if code != 0 {
		t.Errorf("exit code = %d, want a passing run to succeed", code)
	}
	if !strings.Contains(output.String(), "ok") {
		t.Errorf("output = %q, want the test output of the job", output.String())
	}
}

func TestServeReaderReportsAMalformedJobAndKeepsAnswering(t *testing.T) {
	output := &strings.Builder{}
	input := strings.NewReader("not a job\n" + `{"id":"m1"}` + "\n")

	if err := serveReader(input, output); err != nil {
		t.Fatalf("serve: %v", err)
	}

	responses := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(responses) != 2 {
		t.Fatalf("responses = %d, want one per job:\n%s", len(responses), output.String())
	}
	if !strings.Contains(responses[0], `"outcome":"`+outcomeInfrastructureError+`"`) ||
		!strings.Contains(responses[0], "bad job") {
		t.Errorf("first response = %s, want the malformed job reported", responses[0])
	}
	if !strings.Contains(responses[1], `"id":"m1"`) {
		t.Errorf("second response = %s, want the worker to answer the next job too", responses[1])
	}
}

// errReader fails the way a broken job stream does.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("the mutator closed the stream") }

func TestRunReportsAWorkerThatCouldNotReadItsJobs(t *testing.T) {
	if code := run(options{worker: true}, errReader{}, io.Discard); code != 1 {
		t.Errorf("exit code = %d, want a worker that could not read its jobs to report failure", code)
	}
}

func TestEvaluateNeedsBothPathsOfAJob(t *testing.T) {
	for name, incoming := range map[string]job{
		"no generated dir": {ID: "m1", FeatureJSON: "ir.json"},
		"no feature json":  {ID: "m2", GeneratedDir: passingGeneratedDir(t)},
	} {
		t.Run(name, func(t *testing.T) {
			result := evaluate(incoming)

			if result.Outcome != outcomeInfrastructureError {
				t.Errorf("outcome = %q, want an unusable job reported", result.Outcome)
			}
			if !strings.Contains(result.Error, "feature_json") {
				t.Errorf("error = %q, want it to name the missing job fields", result.Error)
			}
		})
	}
}

func TestEvaluateReportsTheErrorOfAFailingJob(t *testing.T) {
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

	result := evaluate(job{ID: "m1", FeatureJSON: ir, GeneratedDir: dir, WorkDir: dir, Timeout: "60s"})

	if result.Outcome != outcomeFailure {
		t.Errorf("outcome = %q, want a failing job reported as a killed mutation", result.Outcome)
	}
	if result.Error == "" {
		t.Error("error is empty, want the reason the job failed")
	}
}

func TestEvaluateReportsATimedOutJob(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module slowprobe\n\ngo 1.24\n")
	writeFile(t, filepath.Join(dir, "acceptance_test.go"), `package slowprobe

import (
	"testing"
	"time"
)

func TestGeneratedFeature(t *testing.T) {
	time.Sleep(30 * time.Second)
}
`)
	ir := filepath.Join(dir, "feature.json")
	writeFile(t, ir, `{"name":"probe"}`)

	result := evaluate(job{ID: "m1", FeatureJSON: ir, GeneratedDir: dir, WorkDir: dir, Timeout: "50ms"})

	if result.Outcome != outcomeInfrastructureError {
		t.Errorf("outcome = %q, want a job that ran out of time reported as infrastructure", result.Outcome)
	}
	if !strings.Contains(result.Error, "timed out") {
		t.Errorf("error = %q, want it to report the timeout", result.Error)
	}
}

func TestJobTimeoutIsTheJobsOwnWhenItNamesOne(t *testing.T) {
	cases := map[string]time.Duration{
		"":         defaultJobTimeout,
		"30s":      30 * time.Second,
		"5m":       5 * time.Minute,
		"1ns":      time.Nanosecond,
		"0s":       defaultJobTimeout,
		"-5s":      defaultJobTimeout,
		"nonsense": defaultJobTimeout,
	}
	for text, want := range cases {
		if got := jobTimeout(job{Timeout: text}); got != want {
			t.Errorf("jobTimeout(%q) = %s, want %s", text, got, want)
		}
	}
}

func TestJobCommandPointsTheRunAtTheJobWorkDirectory(t *testing.T) {
	dir := t.TempDir()

	cmd := jobCommand(context.Background(), job{GeneratedDir: dir, WorkDir: "work"})

	want := "FORGELET_ACCEPTANCE_WORK_DIR=" + filepath.Join(dir, "work", "acceptance-run")
	if value, ok := envValue(cmd.Env, "FORGELET_ACCEPTANCE_WORK_DIR"); !ok || "FORGELET_ACCEPTANCE_WORK_DIR="+value != want {
		t.Errorf("work directory = %q, %v, want %q", value, ok, want)
	}
}

func TestJobCommandLeavesTheWorkDirectoryUnsetWithoutOne(t *testing.T) {
	cmd := jobCommand(context.Background(), job{GeneratedDir: t.TempDir()})

	if value, ok := envValue(cmd.Env, "FORGELET_ACCEPTANCE_WORK_DIR"); ok {
		t.Errorf("work directory = %q, want no work directory for a job without one", value)
	}
}

func TestAbsoluteResolvesRelativePathsAndKeepsTheRestAlone(t *testing.T) {
	base := filepath.Join(string(filepath.Separator), "project")
	cases := map[string]string{
		"":                        "",
		"ir/feature.json":         filepath.Join(base, "ir", "feature.json"),
		"/elsewhere/feature.json": "/elsewhere/feature.json",
	}
	for path, want := range cases {
		if got := absolute(base, path); got != want {
			t.Errorf("absolute(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestProjectRootFindsTheModuleAboveADirectory(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module probe\n\ngo 1.24\n")
	nested := filepath.Join(root, "build", "generated")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	if got := projectRoot(nested); got != root {
		t.Errorf("projectRoot(%q) = %q, want the module root %q", nested, got, root)
	}
}

func TestProjectRootStopsAtTheTopWithoutAModule(t *testing.T) {
	top := string(filepath.Separator)
	if got := projectRoot(top); got != top {
		t.Errorf("projectRoot(%q) = %q, want %q", top, got, top)
	}
}

// passingGeneratedDir is a directory holding one generated acceptance test
// that passes whatever IR it is given.
func passingGeneratedDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module passingprobe\n\ngo 1.24\n")
	writeFile(t, filepath.Join(dir, "acceptance_test.go"), `package passingprobe

import "testing"

func TestGeneratedFeature(t *testing.T) {}
`)
	return dir
}

// envValue reads one variable out of a command environment.
func envValue(env []string, name string) (string, bool) {
	prefix := name + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return strings.TrimPrefix(entry, prefix), true
		}
	}
	return "", false
}
