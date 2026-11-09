package dashboard

import (
	"strings"
	"time"
)

// Statuses a dashboard request can carry.
const (
	StatusPending = "pending"
	StatusDone    = "done"
)

// bodyFallback is the body a request file carries when the request has none.
const bodyFallback = ""

// Request is one chat request in a forge's dashboard.
type Request struct {
	ID        string
	Status    string
	Role      string
	Source    string
	Body      string
	Response  string
	CreatedAt string
	UpdatedAt string
}

// Done reports whether the lieutenant has answered this request.
func (r Request) Done() bool {
	return r.Status == StatusDone
}

// render writes a request in the dashboard's on-disk format.
func render(r Request) string {
	var b strings.Builder
	header(&b, "id", r.ID)
	header(&b, "status", r.Status)
	optionalHeader(&b, "role", r.Role)
	optionalHeader(&b, "source", r.Source)
	header(&b, "created_at", r.CreatedAt)
	optionalHeader(&b, "updated_at", r.UpdatedAt)
	optionalHeader(&b, "response", strings.ReplaceAll(r.Response, "\n", `\n`))
	body(&b, r.Body)
	return b.String()
}

// header writes one request header line.
func header(b *strings.Builder, name, value string) {
	b.WriteString(name + ": " + value + "\n")
}

// optionalHeader writes a request header only when it carries a value.
func optionalHeader(b *strings.Builder, name, value string) {
	if strings.TrimSpace(value) != "" {
		header(b, name, value)
	}
}

// body writes the blank separator and the request body.
func body(b *strings.Builder, text string) {
	if text == "" {
		text = bodyFallback
	}
	b.WriteString("\n" + text)
	if !strings.HasSuffix(text, "\n") {
		b.WriteString("\n")
	}
}

// Parse reads a request file: header lines, a blank line, then the body. It is
// the way every reader of a dashboard request file, in this process or the
// dashboard's own tools, gets at the request's fields.
func Parse(text string) Request {
	header, body, found := strings.Cut(text, "\n\n")
	if !found {
		header, body = text, ""
	}

	request := Request{}
	// The same fields render writes, so the two stay in step.
	fields := map[string]*string{
		"id":         &request.ID,
		"status":     &request.Status,
		"role":       &request.Role,
		"source":     &request.Source,
		"created_at": &request.CreatedAt,
		"updated_at": &request.UpdatedAt,
		"response":   &request.Response,
	}
	for _, line := range strings.Split(header, "\n") {
		key, value, ok := strings.Cut(line, ": ")
		if !ok {
			continue
		}
		if field, known := fields[key]; known {
			*field = strings.TrimSpace(value)
		}
	}
	request.Response = strings.ReplaceAll(request.Response, `\n`, "\n")
	request.Body = strings.TrimRight(body, "\n")
	return request
}

// timestamp is the moment a request was created or answered, as a request file
// writes it.
func timestamp(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.999999999Z")
}

// compactTimestamp is the same moment in the shape a request id carries it.
func compactTimestamp(t time.Time) string {
	return t.UTC().Format("20060102T150405.000000000Z")
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-11-09T11:50:14+01:00","module_hash":"efefa4fef1e74f50b9638deb83391802f38b3ee98faf992e578c6ead22b8f1f6","functions":[{"id":"func/Request.Done","name":"Request.Done","line":30,"end_line":32,"hash":"fa620f41e6c44d32b56b13bf3b99d677cad50139217e6f1448ebde95fc1f568d"},{"id":"func/render","name":"render","line":35,"end_line":46,"hash":"da8c18d11bee4e5e4e6e8be1c6098438eca17f4a966cc5a82269f61376ed438b"},{"id":"func/header","name":"header","line":49,"end_line":51,"hash":"2b49c17433bdcdcd510950cdaba7e2ead7054e01be3347129f9af32fb42bd143"},{"id":"func/optionalHeader","name":"optionalHeader","line":54,"end_line":58,"hash":"8a622ff738c0b5be7ec53e1f5df9a974dfade069c3c461fe46bbd2da5009b2ee"},{"id":"func/body","name":"body","line":61,"end_line":69,"hash":"b14fca220392a8a51efdbba11024b8077eb17413716e9df53d8f80822c756f34"},{"id":"func/Parse","name":"Parse","line":74,"end_line":103,"hash":"cad3f20ec596e3fb5ea3f1be3a7d28250dccfe63fc3a37ca80bc1e49f20c54cb"},{"id":"func/timestamp","name":"timestamp","line":107,"end_line":109,"hash":"03104c7009f0f97f4b7718b177d1ea08aa8227da590e3e927df50a199c716c72"},{"id":"func/compactTimestamp","name":"compactTimestamp","line":112,"end_line":114,"hash":"5134edc621d6cc8ac7ba6910ca8cf1128976d71202d14423091144a1dbfc68bb"}]}
// mutate4go-manifest-end
