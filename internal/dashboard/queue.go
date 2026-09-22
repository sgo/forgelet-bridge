package dashboard

import (
	"context"

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
	requests, err := q.Store.Requests()
	if err != nil {
		return nil, err
	}
	relayed := make([]relay.Request, 0, len(requests))
	for _, request := range requests {
		relayed = append(relayed, relay.Request{
			ID:       request.ID,
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
// {"version":1,"tested_at":"2026-09-22T21:28:19+02:00","module_hash":"200db89bc7045ecc03fc4cc8187e2d0b66e264de75c10004b58e546f05265ab4","functions":[{"id":"func/Queue.Requests","name":"Queue.Requests","line":21,"end_line":35,"hash":"b91905a09bc1c384883a8271cc1bfb18a12ef831c100bf4dab704be7a5cba86f"},{"id":"func/Queue.CreateRequest","name":"Queue.CreateRequest","line":39,"end_line":48,"hash":"d096d34f1391990cbb5b3916b2e08d34221662bf60e36024e17b66f65932ab5e"}]}
// mutate4go-manifest-end
