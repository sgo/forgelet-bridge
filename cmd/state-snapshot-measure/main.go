// Command state-snapshot-measure compares the old independent state readers
// with one SnapshotSource read per dashboard over a fixed synthetic interval.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
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
	ticks := flag.Int("ticks", 32, "number of synthetic bridge ticks")
	payloadBytes := flag.Int("payload-bytes", 1024*1024, "bytes in each state response")
	output := flag.String("output", "", "optional JSON report path")
	flag.Parse()

	result, err := run(context.Background(), *ticks, *payloadBytes)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoded = append(encoded, '\n')
	if *output != "" {
		if err := os.WriteFile(*output, encoded, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	_, _ = os.Stdout.Write(encoded)
}

func run(ctx context.Context, ticks, payloadBytes int) (report, error) {
	if ticks <= 0 {
		return report{}, fmt.Errorf("ticks must be positive")
	}
	if payloadBytes < 1024 {
		return report{}, fmt.Errorf("payload-bytes must be at least 1024")
	}
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
	state := map[string]any{
		"approvals": []map[string]any{{
			"id":      "approval-1",
			"project": "project",
			"task":    "snapshot-card",
			"gate":    "coder → refactorer",
		}},
		"resolved_approvals": []map[string]any{{
			"id":         "approval-old",
			"project":    "project",
			"resolution": "approved",
		}},
		"clarifications": []map[string]any{{
			"id":      "clarification-1",
			"status":  "pending",
			"project": "project",
			"role":    "coder",
			"body":    strings.Repeat("x", payloadBytes),
		}},
	}
	body, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	served := &stateServer{}
	served.server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/api/state" {
			http.NotFound(writer, request)
			return
		}
		served.mu.Lock()
		served.count.Requests++
		served.count.Bytes += uint64(len(body))
		served.mu.Unlock()
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(body)
	}))
	return served, nil
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
