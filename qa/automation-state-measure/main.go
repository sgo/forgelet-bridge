// Command automation-state-measure compares the full dashboard state read with
// the bounded automation projection over the same synthetic bridge interval.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime/metrics"
	"strings"
	"sync"
	"time"

	"github.com/unclebob/forgelet-bridge/internal/dashboard"
	"github.com/unclebob/forgelet-bridge/internal/relay"
)

type counters struct {
	Requests uint64 `json:"requests"`
	Bytes    uint64 `json:"bytes"`
}

type measurement struct {
	Requests uint64  `json:"requests"`
	Bytes    uint64  `json:"bytes"`
	CPU      float64 `json:"cpu_seconds"`
	Wall     float64 `json:"wall_seconds"`
}

type report struct {
	Ticks          int         `json:"ticks"`
	FullBytes      int         `json:"full_payload_bytes"`
	Before         measurement `json:"before"`
	After          measurement `json:"after"`
	RequestDelta   int64       `json:"request_delta"`
	ByteDelta      int64       `json:"byte_delta"`
	CPUDelta       float64     `json:"cpu_delta_seconds"`
	RequestPercent float64     `json:"request_reduction_percent"`
	BytePercent    float64     `json:"byte_reduction_percent"`
	CPUPercent     float64     `json:"cpu_reduction_percent"`
}

type endpoint struct {
	server *httptest.Server
	mu     sync.Mutex
	count  counters
}

func main() {
	if err := runCommand(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runCommand(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("automation-state-measure", flag.ContinueOnError)
	ticks := flags.Int("ticks", 32, "number of synthetic bridge ticks")
	fullBytes := flags.Int("full-payload-bytes", 1024*1024, "bytes in each full state response")
	output := flags.String("output", "", "optional JSON report path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	result, err := run(context.Background(), *ticks, *fullBytes)
	if err != nil {
		return err
	}
	return writeReport(result, *output, stdout)
}

func writeReport(result report, output string, stdout io.Writer) error {
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if output != "" {
		if err := os.WriteFile(output, encoded, 0o644); err != nil {
			return err
		}
	}
	_, err = stdout.Write(encoded)
	return err
}

func run(ctx context.Context, ticks, fullBytes int) (report, error) {
	if ticks <= 0 || fullBytes < 1024 {
		return report{}, fmt.Errorf("ticks must be positive and full-payload-bytes at least 1024")
	}
	forge, err := newEndpoint("family", "forgelet-bridge", fullBytes, true)
	if err != nil {
		return report{}, err
	}
	defer forge.close()
	project, err := newEndpoint("project", "forgelet-bridge", fullBytes, false)
	if err != nil {
		return report{}, err
	}
	defer project.close()

	before, err := measure(ctx, ticks, func() error {
		return fullReads(ctx, forge.server.URL, project.server.URL)
	}, func() counters { return add(forge.counts(), project.counts()) })
	if err != nil {
		return report{}, err
	}
	forge.reset()
	project.reset()
	after, err := measure(ctx, ticks, func() error {
		return automationReads(ctx, forge.server.URL, project.server.URL)
	}, func() counters { return add(forge.counts(), project.counts()) })
	if err != nil {
		return report{}, err
	}
	return report{
		Ticks:          ticks,
		FullBytes:      fullBytes,
		Before:         before,
		After:          after,
		RequestDelta:   int64(before.Requests) - int64(after.Requests),
		ByteDelta:      int64(before.Bytes) - int64(after.Bytes),
		CPUDelta:       before.CPU - after.CPU,
		RequestPercent: reduction(float64(before.Requests), float64(after.Requests)),
		BytePercent:    reduction(float64(before.Bytes), float64(after.Bytes)),
		CPUPercent:     reduction(before.CPU, after.CPU),
	}, nil
}

func fullReads(ctx context.Context, forgeURL, projectURL string) error {
	if _, err := dashboard.NewForgeAPI("/synthetic/forge", forgeURL).State(ctx); err != nil {
		return err
	}
	_, err := dashboard.NewForgeAPI("/synthetic/project", projectURL).State(ctx)
	return err
}

func automationReads(ctx context.Context, forgeURL, projectURL string) error {
	if _, err := (dashboard.SnapshotSource{Root: "/synthetic/forge", ConfiguredURL: forgeURL}).Snapshot(ctx); err != nil {
		return err
	}
	_, err := (dashboard.SnapshotSource{Root: "/synthetic/project", ConfiguredURL: projectURL}).Snapshot(ctx)
	return err
}

func measure(ctx context.Context, ticks int, read func() error, counts func() counters) (measurement, error) {
	startCPU := processCPU()
	start := time.Now()
	for range ticks {
		if err := read(); err != nil {
			return measurement{}, err
		}
	}
	elapsed := time.Since(start)
	served := counts()
	return measurement{
		Requests: served.Requests, Bytes: served.Bytes,
		CPU: processCPU() - startCPU, Wall: elapsed.Seconds(),
	}, nil
}

func newEndpoint(kind, name string, fullBytes int, family bool) (*endpoint, error) {
	full, err := json.Marshal(map[string]any{
		"approvals":          []any{},
		"resolved_approvals": []any{},
		"clarifications":     []any{},
		"chat":               []map[string]string{{"body": strings.Repeat("x", fullBytes)}},
	})
	if err != nil {
		return nil, err
	}
	automation, err := json.Marshal(measurementAutomation(kind, name, family))
	if err != nil {
		return nil, err
	}
	served := &endpoint{}
	served.server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body []byte
		switch request.URL.Path {
		case "/api/state":
			body = full
		case "/api/automation-state":
			body = automation
			writer.Header().Set("X-SwarmForge-Projection", dashboard.AutomationProjection)
		default:
			http.NotFound(writer, request)
			return
		}
		served.record(body)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(body)
	}))
	return served, nil
}

func measurementAutomation(kind, name string, family bool) dashboard.AutomationState {
	question := "Should the bounded automation projection be used for this tick?"
	resolved := make([]dashboard.AutomationResolvedApproval, 0, 200)
	for index := range 200 {
		resolved = append(resolved, dashboard.AutomationResolvedApproval{
			ID: fmt.Sprintf("approval-%03d", index), Project: name,
			Card: fmt.Sprintf("card-%03d", index), Resolution: relay.ResolutionAutomaticSpecApproval,
		})
	}
	state := dashboard.AutomationState{
		Projection: dashboard.AutomationProjection,
		Identity: &dashboard.AutomationIdentity{
			Kind: kind, Name: name,
			AllProjects:  measurementSection([]string{name}),
			OpenProjects: measurementSection([]string{name}),
		},
		Approvals:         measurementSection([]dashboard.AutomationApproval{}),
		ResolvedApprovals: measurementSection(resolved),
		Clarifications: &dashboard.AutomationClarifications{
			Legacy: measurementSection([]dashboard.AutomationClarification{}),
			Typed: measurementSection([]dashboard.AutomationClarification{{
				ID: "typed-request", ClarificationID: "clarification-1", Status: "open",
				Project: name, Attention: "unseen", QuestionExcerpt: &question,
			}}),
		},
		Projects: measurementSection([]dashboard.AutomationProject{}),
	}
	if family {
		state.Projects = measurementSection([]dashboard.AutomationProject{{
			Name:     name,
			Lanes:    measurementSection([]string{"master", "coder"}),
			Cards:    measurementSection([]dashboard.AutomationCard{{Name: "card-current", Lane: "coder"}}),
			Activity: measurementSection([]dashboard.AutomationActivity{}),
		}})
	}
	return state
}

func measurementSection[T any](items []T) *dashboard.AutomationSection[T] {
	return &dashboard.AutomationSection[T]{Count: len(items), Items: items}
}

func (e *endpoint) record(body []byte) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.count.Requests++
	e.count.Bytes += uint64(len(body))
}

func (e *endpoint) counts() counters {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.count
}

func (e *endpoint) reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.count = counters{}
}

func (e *endpoint) close() { e.server.Close() }

func add(left, right counters) counters {
	return counters{Requests: left.Requests + right.Requests, Bytes: left.Bytes + right.Bytes}
}

func reduction(before, after float64) float64 {
	if before == 0 {
		return 0
	}
	return (before - after) / before * 100
}

func processCPU() float64 {
	sample := []metrics.Sample{{Name: "/cpu/classes/total:cpu-seconds"}}
	metrics.Read(sample)
	return sample[0].Value.Float64()
}
