// Command acceptance-runner is the project's acceptance runner adapter. In
// worker mode it stays hot and answers newline-delimited JSON jobs from the
// Gherkin mutator, running the generated acceptance tests against each mutated
// JSON IR.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
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

	// defaultJobTimeout is how long one run may take when a job names no
	// timeout of its own.
	defaultJobTimeout = 10 * time.Minute
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

const usage = `usage: acceptance-runner --feature-json <ir> --generated-dir <dir> [--work-dir <dir>] [--timeout 10m]
   or: acceptance-runner --worker
`

// options are the runner's command-line options.
type options struct {
	worker       bool
	featureJSON  string
	generatedDir string
	workDir      string
	timeout      time.Duration
}

func main() {
	parsed, err := parseOptions(os.Args[1:])
	if err != nil {
		os.Exit(usageExit(err))
	}
	os.Exit(run(parsed, os.Stdin, os.Stdout))
}

// usageExit prints the usage text for a command line the runner cannot use and
// reports the exit status the process should end with. Asking for the usage
// text is not a failure.
func usageExit(err error) int {
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprint(os.Stdout, usage)
		return 0
	}
	fmt.Fprintln(os.Stderr, "acceptance-runner:", err)
	fmt.Fprint(os.Stderr, usage)
	return 2
}

// parseOptions reads the runner's command line.
func parseOptions(args []string) (options, error) {
	flags := flag.NewFlagSet("acceptance-runner", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	worker := flags.Bool("worker", false, "stay hot and answer mutation jobs on stdin/stdout")
	featureJSON := flags.String("feature-json", "", "JSON IR to run the generated tests against")
	generatedDir := flags.String("generated-dir", "", "directory holding the generated acceptance tests")
	workDir := flags.String("work-dir", "", "scratch directory for this run")
	timeout := flags.Duration("timeout", defaultJobTimeout, "how long one run may take")
	if err := flags.Parse(args); err != nil {
		return options{}, err
	}
	return options{
		worker:       *worker,
		featureJSON:  *featureJSON,
		generatedDir: *generatedDir,
		workDir:      *workDir,
		timeout:      *timeout,
	}, nil
}

// run answers jobs in worker mode, or runs the one job the options name, and
// returns the exit status of the run.
func run(parsed options, in io.Reader, out io.Writer) int {
	if parsed.worker {
		if err := serveReader(in, out); err != nil {
			fmt.Fprintln(os.Stderr, "acceptance-runner:", err)
			return 1
		}
		return 0
	}
	if parsed.featureJSON == "" || parsed.generatedDir == "" {
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
	result := evaluate(job{
		ID:           "run",
		FeatureJSON:  parsed.featureJSON,
		GeneratedDir: parsed.generatedDir,
		WorkDir:      parsed.workDir,
		Timeout:      parsed.timeout.String(),
	})
	fmt.Fprint(out, result.Output)
	if result.Outcome != outcomeSuccess {
		return 1
	}
	return 0
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
	timeout := jobTimeout(incoming)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	result := response{ID: incoming.ID}
	if incoming.GeneratedDir == "" || incoming.FeatureJSON == "" {
		result.Outcome = outcomeInfrastructureError
		result.Error = "job needs feature_json and generated_dir"
		return result
	}

	output, err := jobCommand(ctx, incoming).CombinedOutput()

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

// jobTimeout is how long one job may run: the job's own timeout when it names
// a usable one, the default otherwise.
func jobTimeout(incoming job) time.Duration {
	if incoming.Timeout == "" {
		return defaultJobTimeout
	}
	parsed, err := time.ParseDuration(incoming.Timeout)
	if err != nil || parsed <= 0 {
		return defaultJobTimeout
	}
	return parsed
}

// jobCommand builds the test command that runs one mutated IR against the
// generated acceptance tests.
func jobCommand(ctx context.Context, incoming job) *exec.Cmd {
	// The mutator names its work files relative to the project, while the
	// generated tests run inside the generated directory.
	base := projectRoot(incoming.GeneratedDir)
	workDir := absolute(base, incoming.WorkDir)

	cmd := exec.CommandContext(ctx, "go", "test", "-tags", "goolm", "-count=1", ".")
	cmd.Dir = incoming.GeneratedDir
	cmd.Env = append(os.Environ(), "FORGELET_ACCEPTANCE_IR="+absolute(base, incoming.FeatureJSON))
	if workDir != "" {
		cmd.Env = append(cmd.Env, "FORGELET_ACCEPTANCE_WORK_DIR="+filepath.Join(workDir, "acceptance-run"))
	}
	return cmd
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

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-21T23:33:11+02:00","module_hash":"86807706cd8196ff240cf526338eb08a18422696c5cef90f009bec3150b028c2","functions":[{"id":"func/main","name":"main","line":63,"end_line":69,"hash":"3689ecf72872518ff04976e93f85fc27c598b7b8933aedeeee993e7460964455"},{"id":"func/usageExit","name":"usageExit","line":74,"end_line":82,"hash":"dc27e28a72f7419674321fcf9eb7428c477568298bd1b02698c4d378aa14c7de"},{"id":"func/parseOptions","name":"parseOptions","line":85,"end_line":103,"hash":"ff91d520f94694270e30f9893c537583caa4ddabe1b7993f13e8886c3c5522d9"},{"id":"func/run","name":"run","line":107,"end_line":131,"hash":"e5da38934f5608a118d9a4d4a27d5ea173e04ddab55ed199f33914b89658f092"},{"id":"func/serveReader","name":"serveReader","line":134,"end_line":155,"hash":"073597d7920ee79f5d4ae488224dd57d98c9d2a7cced385bc18353f47b8b1822"},{"id":"func/evaluate","name":"evaluate","line":158,"end_line":183,"hash":"867601d106328aea8aa295229d645db0c7dd58250b7b1efa013bc55e5bdc4fdf"},{"id":"func/jobTimeout","name":"jobTimeout","line":187,"end_line":196,"hash":"faca9af4f82387d10a3574de66133f28af84c2c1de5b96c52e99fbe851101c51"},{"id":"func/jobCommand","name":"jobCommand","line":200,"end_line":213,"hash":"665a66ad876d9fd46c368c54024ae03fa7040581ea8e73eef726f8a96b7d6716"},{"id":"func/absolute","name":"absolute","line":216,"end_line":221,"hash":"9bd04c4be9972bcd5693365af0b07a68e37ee50ceed890f9c794296d4376ddf4"},{"id":"func/projectRoot","name":"projectRoot","line":224,"end_line":237,"hash":"227d720227a69c454e81fa4cad4b08533bb285f770ab50537e89c83ad33e6e2a"},{"id":"func/classify","name":"classify","line":242,"end_line":260,"hash":"996662d9e87bb3628b8cb49bdcba63633bad646ea4b9d431ba1c4d1703a4d9eb"}]}
// mutate4go-manifest-end
