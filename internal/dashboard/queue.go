package dashboard

import (
	"context"
	"fmt"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

// Queue is the dashboard request queue seen as a relay forge store: the
// bridge cares about the request text and the answer, not the dashboard's
// bookkeeping.
type Queue struct {
	Store *Store
	// Root and ConfiguredURL find the forge's dashboard, which is what takes a
	// new request, so the wake it gives comes with it.
	Root          string
	ConfiguredURL string
}

// Requests returns the pending and answered requests in relay form.
func (q Queue) Requests() ([]relay.Request, error) {
	requests, readErr := q.Store.Requests()
	if readErr != nil {
		return nil, fmt.Errorf("read dashboard requests for %s: %w", q.Store.Root(), readErr)
	}
	relayed := make([]relay.Request, 0, len(requests))
	for _, request := range requests {
		relayed = append(relayed, relay.Request{
			ID:       request.ID,
			Source:   request.Source,
			Body:     request.Body,
			Response: request.Response,
		})
	}
	return relayed, nil
}

// CreateRequest gives the chat request to the forge's dashboard the way a
// client does: the dashboard queues it and wakes the lieutenant.
func (q Queue) CreateRequest(ctx context.Context, body string) (string, error) {
	url, err := URL(q.Root, q.ConfiguredURL)
	if err != nil {
		return "", err
	}
	if err := NewAPI(url).Chat(ctx, body); err != nil {
		return "", err
	}
	return "", nil
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-11-09T11:49:19+01:00","module_hash":"bf43a3efbc1b0445d3b382a0eaa5f6b443ae9bdfd548992b78b0eb7a11a734ff","functions":[{"id":"func/Queue.Requests","name":"Queue.Requests","line":22,"end_line":37,"hash":"b4b42e94f6297e2b14ad50a678c29c12d297a297264af1569f8b07a5050aa48c"},{"id":"func/Queue.CreateRequest","name":"Queue.CreateRequest","line":41,"end_line":50,"hash":"d096d34f1391990cbb5b3916b2e08d34221662bf60e36024e17b66f65932ab5e"}]}
// mutate4go-manifest-end
