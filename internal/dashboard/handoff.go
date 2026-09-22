package dashboard

import (
	"os"
	"path/filepath"
	"strings"
)

// handoff is the part of an approval the dashboard's state does not carry: who
// handed the work over, and the files the handoff says changed.
type handoff struct {
	from      string
	to        string
	artifacts []string
}

// readHandoff reads the pending handoff an approval came from. The bridge only
// reads it — the dashboard's endpoints are what decide anything.
func readHandoff(root, project, id string) (handoff, error) {
	path := filepath.Join(root, "projects", project, ".swarmforge", "handoffs", "pending_approval", id+".handoff")
	data, err := os.ReadFile(path)
	if err != nil {
		return handoff{}, err
	}
	header, _, _ := strings.Cut(string(data), "\n\n")
	headers := map[string]string{}
	for _, line := range strings.Split(header, "\n") {
		key, value, ok := strings.Cut(line, ": ")
		if ok {
			headers[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}

	var artifacts []string
	for _, artifact := range strings.Split(headers["artifacts"], ",") {
		if trimmed := strings.TrimSpace(artifact); trimmed != "" {
			artifacts = append(artifacts, trimmed)
		}
	}
	to := headers["to"]
	if first, _, found := strings.Cut(to, ","); found {
		to = strings.TrimSpace(first)
	}
	return handoff{from: headers["role"], to: to, artifacts: artifacts}, nil
}
