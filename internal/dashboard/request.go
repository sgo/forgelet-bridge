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
	b.WriteString("id: " + r.ID + "\n")
	b.WriteString("status: " + r.Status + "\n")
	if strings.TrimSpace(r.Role) != "" {
		b.WriteString("role: " + r.Role + "\n")
	}
	b.WriteString("created_at: " + r.CreatedAt + "\n")
	if r.UpdatedAt != "" {
		b.WriteString("updated_at: " + r.UpdatedAt + "\n")
	}
	if r.Response != "" {
		b.WriteString("response: " + strings.ReplaceAll(r.Response, "\n", `\n`) + "\n")
	}
	body := r.Body
	if body == "" {
		body = bodyFallback
	}
	b.WriteString("\n" + body)
	if !strings.HasSuffix(body, "\n") {
		b.WriteString("\n")
	}
	return b.String()
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
// {"version":1,"tested_at":"2026-09-21T23:27:45+02:00","module_hash":"22cd0e7ee8f13f36560680553cee6f8052fe16109b83cbdfdfdfa5472628477d","functions":[{"id":"func/Request.Done","name":"Request.Done","line":29,"end_line":31,"hash":"fa620f41e6c44d32b56b13bf3b99d677cad50139217e6f1448ebde95fc1f568d"},{"id":"func/render","name":"render","line":34,"end_line":57,"hash":"76f2a31e5d7f111f582c38401047b25877e955da6f2e5a83497afb2d58100d45"},{"id":"func/Parse","name":"Parse","line":62,"end_line":90,"hash":"b9359bc58b3ff298d19f4bf0e3394d55f8ecb84f8962c9c74991313b02a3a25f"},{"id":"func/timestamp","name":"timestamp","line":94,"end_line":96,"hash":"03104c7009f0f97f4b7718b177d1ea08aa8227da590e3e927df50a199c716c72"},{"id":"func/compactTimestamp","name":"compactTimestamp","line":99,"end_line":101,"hash":"5134edc621d6cc8ac7ba6910ca8cf1128976d71202d14423091144a1dbfc68bb"}]}
// mutate4go-manifest-end
