// Command acceptance-runner is the project's acceptance runner adapter. In
// worker mode it stays hot and answers newline-delimited JSON jobs from the
// Gherkin mutator, running the generated acceptance tests against each mutated
// JSON IR.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	outcomeSuccess             = "test_success"
	outcomeFailure             = "test_failure"
	outcomeInfrastructureError = "infrastructure_error"
)

// job is one mutation the mutator asks the runner to evaluate.
type job struct {
	ID           string `json:"id"`
	FeatureJSON  string `json:"feature_json"`
	GeneratedDir string `json:"generated_dir"`
	WorkDir      string `json:"work_dir"`
	Timeout      string `json:"timeout"`
}

// response is the runner's answer for one job.
type response struct {
	ID       string `json:"id"`
	Outcome  string `json:"outcome"`
	Output   string `json:"output"`
	Error    string `json:"error"`
	Duration int64  `json:"duration"`
}

func main() {
	worker := flag.Bool("worker", false, "stay hot and answer mutation jobs on stdin/stdout")
	featureJSON := flag.String("feature-json", "", "JSON IR to run the generated tests against")
	generatedDir := flag.String("generated-dir", "", "directory holding the generated acceptance tests")
	workDir := flag.String("work-dir", "", "scratch directory for this run")
	timeout := flag.Duration("timeout", 10*time.Minute, "how long one run may take")
	flag.Parse()

	if *worker {
		if err := serveReader(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "acceptance-runner:", err)
			os.Exit(1)
		}
		return
	}
	if *featureJSON == "" || *generatedDir == "" {
		fmt.Fprintln(os.Stderr, "usage: acceptance-runner --feature-json <ir> --generated-dir <dir> [--work-dir <dir>] [--timeout 10m]")
		fmt.Fprintln(os.Stderr, "   or: acceptance-runner --worker")
		os.Exit(2)
	}
	result := evaluate(job{
		ID:           "run",
		FeatureJSON:  *featureJSON,
		GeneratedDir: *generatedDir,
		WorkDir:      *workDir,
		Timeout:      timeout.String(),
	})
	fmt.Print(result.Output)
	if result.Outcome != outcomeSuccess {
		os.Exit(1)
	}
}

// serveReader answers jobs until the input closes.
func serveReader(in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	encoder := json.NewEncoder(out)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var incoming job
		if err := json.Unmarshal([]byte(line), &incoming); err != nil {
			if err := encoder.Encode(response{Outcome: outcomeInfrastructureError, Error: "bad job: " + err.Error()}); err != nil {
				return err
			}
			continue
		}
		if err := encoder.Encode(evaluate(incoming)); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// evaluate runs one job and classifies the outcome.
func evaluate(incoming job) response {
	started := time.Now()
	timeout := 10 * time.Minute
	if incoming.Timeout != "" {
		if parsed, err := time.ParseDuration(incoming.Timeout); err == nil && parsed > 0 {
			timeout = parsed
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	result := response{ID: incoming.ID}
	if incoming.GeneratedDir == "" || incoming.FeatureJSON == "" {
		result.Outcome = outcomeInfrastructureError
		result.Error = "job needs feature_json and generated_dir"
		return result
	}

	// The mutator names its work files relative to the project, while the
	// generated tests run inside the generated directory.
	base := projectRoot(incoming.GeneratedDir)
	featureJSON := absolute(base, incoming.FeatureJSON)
	workDir := absolute(base, incoming.WorkDir)

	cmd := exec.CommandContext(ctx, "go", "test", "-tags", "goolm", "-count=1", ".")
	cmd.Dir = incoming.GeneratedDir
	env := append(os.Environ(),
		"FORGELET_ACCEPTANCE_IR="+featureJSON,
	)
	if workDir != "" {
		env = append(env, "FORGELET_ACCEPTANCE_WORK_DIR="+filepath.Join(workDir, "acceptance-run"))
	}
	cmd.Env = env
	output, err := cmd.CombinedOutput()

	result.Output = string(output)
	result.Duration = time.Since(started).Nanoseconds()
	result.Outcome = classify(string(output), err, ctx.Err())
	if err != nil && result.Error == "" {
		result.Error = err.Error()
	}
	if ctx.Err() != nil {
		result.Error = "timed out after " + timeout.String()
	}
	return result
}

// absolute resolves a job path against the base directory.
func absolute(base, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(base, path)
}

// projectRoot walks up from a directory until it finds the module.
func projectRoot(dir string) string {
	absoluteDir, err := filepath.Abs(dir)
	if err != nil {
		return "."
	}
	for current := absoluteDir; ; current = filepath.Dir(current) {
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			return current
		}
		if filepath.Dir(current) == current {
			return absoluteDir
		}
	}
}

// classify turns a test run into the mutator's vocabulary: the generated tests
// failing means the mutation was killed, the tests passing means it survived,
// and anything that stopped the tests from running is an infrastructure error.
func classify(output string, err error, contextErr error) string {
	if contextErr != nil {
		return outcomeInfrastructureError
	}
	if err == nil {
		return outcomeSuccess
	}
	switch {
	case strings.Contains(output, "[build failed]"),
		strings.Contains(output, "[setup failed]"),
		strings.Contains(output, "no such file or directory"),
		strings.Contains(output, "cannot find package"):
		return outcomeInfrastructureError
	case strings.Contains(output, "--- FAIL"), strings.Contains(output, "\nFAIL"), strings.HasPrefix(output, "FAIL"):
		return outcomeFailure
	default:
		return outcomeInfrastructureError
	}
}
