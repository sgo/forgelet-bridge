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

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-22T21:26:19+02:00","module_hash":"deb7958f7955b3b90a0820bb64cf123a385cd4deb37679641d2ea80a230324b0","functions":[{"id":"func/readHandoff","name":"readHandoff","line":19,"end_line":30,"hash":"56cc7ec083d2ea932b6d18ce6fea0ebb8475b1b8fcaa090daac5398e2c1fe761"},{"id":"func/handoffPath","name":"handoffPath","line":33,"end_line":35,"hash":"6aa48943bf76b5cd242d95b7ec27cde905128c1eedba2629d9b7a09684182ae7"},{"id":"func/handoffHeaders","name":"handoffHeaders","line":38,"end_line":49,"hash":"c06d52ca4d62eab3b1c0a3879e337c3261f76b3ab704b80281defc60f7cd6997"},{"id":"func/firstItem","name":"firstItem","line":53,"end_line":56,"hash":"74cd3584d86870618d21d50d0937c21eb7ee474ab0243e9ef9d63f5a7f3ef97d"},{"id":"func/commaItems","name":"commaItems","line":59,"end_line":67,"hash":"ff6d7ac9dfc1d6e5c47cefad5a10eb3303242c6c07db18fe0579ca9b8c149d51"}]}
// mutate4go-manifest-end
