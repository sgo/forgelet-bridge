// Command state-snapshot-measure compares the old independent state readers
// with one SnapshotSource read per dashboard over a fixed synthetic interval.
// It is QA tooling and is not part of the deployed bridge.
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
	PayloadBytes   int         `json:"payload_bytes"`
	Before         measurement `json:"before"`
	After          measurement `json:"after"`
	RequestDelta   int64       `json:"request_delta"`
	ByteDelta      int64       `json:"byte_delta"`
	CPUDelta       float64     `json:"cpu_delta_seconds"`
	RequestPercent float64     `json:"request_reduction_percent"`
	BytePercent    float64     `json:"byte_reduction_percent"`
	CPUPercent     float64     `json:"cpu_reduction_percent"`
}

type stateServer struct {
	server *httptest.Server
	mu     sync.Mutex
	count  counters
}

func main() {
	err := runCommand(os.Args[1:], os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runCommand(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("state-snapshot-measure", flag.ContinueOnError)
	ticks := flags.Int("ticks", 32, "number of synthetic bridge ticks")
	payloadBytes := flags.Int("payload-bytes", 1024*1024, "bytes in each state response")
	output := flags.String("output", "", "optional JSON report path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	result, err := run(context.Background(), *ticks, *payloadBytes)
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

func run(ctx context.Context, ticks, payloadBytes int) (report, error) {
	if err := validateMeasurement(ticks, payloadBytes); err != nil {
		return report{}, err
	}
	return compare(ctx, ticks, payloadBytes)
}

func validateMeasurement(ticks, payloadBytes int) error {
	if ticks <= 0 {
		return fmt.Errorf("ticks must be positive")
	}
	if payloadBytes < 1024 {
		return fmt.Errorf("payload-bytes must be at least 1024")
	}
	return nil
}

func compare(ctx context.Context, ticks, payloadBytes int) (report, error) {
	forge, err := newStateServer(payloadBytes)
	if err != nil {
		return report{}, err
	}
	defer forge.close()
	project, err := newStateServer(payloadBytes)
	if err != nil {
		return report{}, err
	}
	defer project.close()

	before, err := measure(ctx, ticks, func() error {
		return oldReads(ctx, forge.server.URL, project.server.URL)
	}, func() counters {
		return addCounters(forge.counts(), project.counts())
	})
	if err != nil {
		return report{}, err
	}
	forge.reset()
	project.reset()
	after, err := measure(ctx, ticks, func() error {
		return snapshotReads(ctx, forge.server.URL, project.server.URL)
	}, func() counters {
		return addCounters(forge.counts(), project.counts())
	})
	if err != nil {
		return report{}, err
	}

	result := report{
		Ticks:          ticks,
		PayloadBytes:   payloadBytes,
		Before:         before,
		After:          after,
		RequestDelta:   int64(before.Requests) - int64(after.Requests),
		ByteDelta:      int64(before.Bytes) - int64(after.Bytes),
		CPUDelta:       before.CPU - after.CPU,
		RequestPercent: reduction(float64(before.Requests), float64(after.Requests)),
		BytePercent:    reduction(float64(before.Bytes), float64(after.Bytes)),
		CPUPercent:     reduction(before.CPU, after.CPU),
	}
	return result, nil
}

func oldReads(ctx context.Context, forgeURL, projectURL string) error {
	approvals := dashboard.Approvals{Root: "/synthetic/forge", ConfiguredURL: forgeURL}
	if _, err := approvals.Pending(ctx); err != nil {
		return err
	}
	if _, err := approvals.Resolved(ctx); err != nil {
		return err
	}
	if _, err := (dashboard.Clarifications{Root: "/synthetic/forge", ConfiguredURL: forgeURL}).Pending(ctx); err != nil {
		return err
	}
	_, err := (dashboard.Clarifications{Root: "/synthetic/project", ConfiguredURL: projectURL, Project: "project"}).Pending(ctx)
	return err
}

func snapshotReads(ctx context.Context, forgeURL, projectURL string) error {
	if _, err := (dashboard.SnapshotSource{Root: "/synthetic/forge", ConfiguredURL: forgeURL}).Snapshot(ctx); err != nil {
		return err
	}
	_, err := (dashboard.SnapshotSource{Root: "/synthetic/project", ConfiguredURL: projectURL, Project: "project"}).Snapshot(ctx)
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
		Requests: served.Requests,
		Bytes:    served.Bytes,
		CPU:      processCPU() - startCPU,
		Wall:     elapsed.Seconds(),
	}, nil
}

func newStateServer(payloadBytes int) (*stateServer, error) {
	question := "which lane?"
	automation := dashboard.AutomationState{
		Projection: dashboard.AutomationProjection,
		Identity: &dashboard.AutomationIdentity{
			Kind: "family", Name: "project",
			AllProjects:  &dashboard.AutomationSection[string]{Count: 1, Items: []string{"project"}},
			OpenProjects: &dashboard.AutomationSection[string]{Count: 1, Items: []string{"project"}},
		},
		Approvals:         &dashboard.AutomationSection[dashboard.AutomationApproval]{Items: []dashboard.AutomationApproval{}},
		ResolvedApprovals: &dashboard.AutomationSection[dashboard.AutomationResolvedApproval]{Items: []dashboard.AutomationResolvedApproval{}},
		Clarifications: &dashboard.AutomationClarifications{
			Legacy: &dashboard.AutomationSection[dashboard.AutomationClarification]{Count: 1, Items: []dashboard.AutomationClarification{{
				ID: "clarification-1", Status: "pending", Project: "project",
				Role: "coder", QuestionExcerpt: &question,
			}}},
			Typed: &dashboard.AutomationSection[dashboard.AutomationClarification]{Items: []dashboard.AutomationClarification{}},
		},
		Projects: &dashboard.AutomationSection[dashboard.AutomationProject]{Items: []dashboard.AutomationProject{}},
	}
	state := struct {
		dashboard.AutomationState
		Padding string `json:"padding"`
	}{AutomationState: automation, Padding: strings.Repeat("x", payloadBytes)}
	automationBody, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	fullBody, err := json.Marshal(map[string]any{
		"approvals": []any{}, "resolved_approvals": []any{}, "clarifications": []any{},
		"padding": strings.Repeat("x", payloadBytes),
	})
	if err != nil {
		return nil, err
	}
	target := max(len(fullBody), len(automationBody))
	fullBody = append(fullBody, bytesOf(' ', target-len(fullBody))...)
	automationBody = append(automationBody, bytesOf(' ', target-len(automationBody))...)
	served := &stateServer{}
	served.server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body := fullBody
		if request.URL.Path == "/api/automation-state" {
			body = automationBody
		}
		if request.Method != http.MethodGet || (request.URL.Path != "/api/state" && request.URL.Path != "/api/automation-state") {
			http.NotFound(writer, request)
			return
		}
		served.mu.Lock()
		served.count.Requests++
		served.count.Bytes += uint64(len(body))
		served.mu.Unlock()
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/api/automation-state" {
			writer.Header().Set("X-SwarmForge-Projection", dashboard.AutomationProjection)
		}
		_, _ = writer.Write(body)
	}))
	return served, nil
}

func bytesOf(value byte, count int) []byte {
	return []byte(strings.Repeat(string(value), count))
}

func (s *stateServer) counts() counters {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.count
}

func (s *stateServer) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.count = counters{}
}

func (s *stateServer) close() {
	s.server.Close()
}

func addCounters(left, right counters) counters {
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

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-11-14T23:24:26+01:00","module_hash":"53b982a72de06e91a73ce50bbd1a3b3b169bf2290a950c4bfacd68b7f9b58ce5","functions":[{"id":"func/main","name":"main","line":53,"end_line":59,"hash":"800495c6d9069aa4e64f48642780a79837180c3a302aca3cc765366e0d527721"},{"id":"func/runCommand","name":"runCommand","line":61,"end_line":74,"hash":"93bd86e223b33b6f30c822e43e4533385c46a4e23d9d2171198f1dd577676739"},{"id":"func/writeReport","name":"writeReport","line":76,"end_line":89,"hash":"c01a454c60cb9c3de21f86268a3db46487f86a70196cf58ea13920565f953ea7"},{"id":"func/run","name":"run","line":91,"end_line":96,"hash":"ab08f9c6df9b8a8b5bbee3f00ec985aff60f1e13403c721d82002e11aebbd89e"},{"id":"func/validateMeasurement","name":"validateMeasurement","line":98,"end_line":106,"hash":"b3c76ca08c19266d49996d82a6f3fde509432a0205fb360e50c5b6cf6151e8d9"},{"id":"func/compare","name":"compare","line":108,"end_line":152,"hash":"196bab42b9b6725ebf33bbdd406e8e2bbd1d192e485426b2137b85ecc4e03cc5"},{"id":"func/oldReads","name":"oldReads","line":154,"end_line":167,"hash":"e1f10694ce28e3152774191a6c7ce167fcc86f52dac60d3b8e43faf63784b811"},{"id":"func/snapshotReads","name":"snapshotReads","line":169,"end_line":175,"hash":"362843fd3457f94bcbc56917e2e956ae06a820a684c8ce0cf17269ef88d5bced"},{"id":"func/measure","name":"measure","line":177,"end_line":193,"hash":"22dc8da702ae79d3f14f6ba2dc00197bb3cf70d1cc2b63e5dbe3a55ce33061f9"},{"id":"func/newStateServer","name":"newStateServer","line":195,"end_line":234,"hash":"12416afa88fdcd45d5b1d3bed2da858a6f100a0999eda6883b20fabc3efb0a43"},{"id":"func/stateServer.counts","name":"stateServer.counts","line":236,"end_line":240,"hash":"9c127346972cb4447475b8648bdbd2a080740bb0ea9699536de28e642464b21d"},{"id":"func/stateServer.reset","name":"stateServer.reset","line":242,"end_line":246,"hash":"cc51e666d39e35f84056f2c5eb63176b26279accedc99118e3476fe422c91821"},{"id":"func/stateServer.close","name":"stateServer.close","line":248,"end_line":250,"hash":"97b4d70869c7d1241b47dd2627b65588322f25fd8b118f1ab0737fc4e211045b"},{"id":"func/addCounters","name":"addCounters","line":252,"end_line":254,"hash":"15fbcba5b6d6377fa1bbf7845e6b04f79fa9e95a1d9df83908c6a25aa590bd1b"},{"id":"func/reduction","name":"reduction","line":256,"end_line":261,"hash":"c8be8e92df87acd969f46590e3ee88998c5c5cce154b7b0e0c223c569d62766d"},{"id":"func/processCPU","name":"processCPU","line":263,"end_line":267,"hash":"c66d3a6a7eaaa8374154615f5ae784cef60af248ecc7589b90c60cb51632922d"}]}
// mutate4go-manifest-end
