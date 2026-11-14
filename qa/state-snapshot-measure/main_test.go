package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSyntheticMeasurementHalvesRequestsAndBytes(t *testing.T) {
	result, err := run(context.Background(), 3, 4096)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Before.Requests != 12 || result.After.Requests != 6 {
		t.Fatalf("requests before/after = %d/%d, want 12/6", result.Before.Requests, result.After.Requests)
	}
	if result.After.Bytes != result.Before.Bytes/2 {
		t.Fatalf("bytes before/after = %d/%d, want half", result.Before.Bytes, result.After.Bytes)
	}
	if result.RequestDelta != 6 || result.ByteDelta <= 0 {
		t.Errorf("reduction = %+v, want the removed duplicate reads", result)
	}
}

func TestRunCommandWritesTheReportAndValidatesInput(t *testing.T) {
	var stdout bytes.Buffer
	output := filepath.Join(t.TempDir(), "measurement.json")
	if err := runCommand([]string{"--ticks", "1", "--payload-bytes", "1024", "--output", output}, &stdout); err != nil {
		t.Fatalf("runCommand: %v", err)
	}
	if stdout.Len() == 0 {
		t.Fatal("runCommand wrote no report")
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatalf("output report: %v", err)
	}
	if err := runCommand([]string{"--ticks", "0"}, &stdout); err == nil {
		t.Fatal("runCommand accepted zero ticks")
	}
}
