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
	data, err := os.ReadFile(handoffPath(root, project, id))
	if err != nil {
		return handoff{}, err
	}
	headers := handoffHeaders(string(data))
	return handoff{
		from:      headers["role"],
		to:        firstItem(headers["to"]),
		artifacts: commaItems(headers["artifacts"]),
	}, nil
}

// handoffPath is where a project keeps the pending handoff of an approval.
func handoffPath(root, project, id string) string {
	return filepath.Join(root, "projects", project, ".swarmforge", "handoffs", "pending_approval", id+".handoff")
}

// handoffHeaders reads a handoff's header lines: the part before its body.
func handoffHeaders(content string) map[string]string {
	header, _, _ := strings.Cut(content, "\n\n")
	headers := map[string]string{}
	for _, line := range strings.Split(header, "\n") {
		key, value, ok := strings.Cut(line, ": ")
		if !ok {
			continue
		}
		headers[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return headers
}

// firstItem is the first of a handoff's comma-separated list, empty when the
// handoff names none.
func firstItem(list string) string {
	first, _, _ := strings.Cut(list, ",")
	return strings.TrimSpace(first)
}

// commaItems reads a handoff's comma-separated list.
func commaItems(list string) []string {
	var items []string
	for _, item := range strings.Split(list, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}
