package dashboard

import "github.com/unclebob/forgelet-bridge/internal/relay"

// Queue is the dashboard request queue seen as a relay forge store: the
// bridge cares about the request text and the answer, not the dashboard's
// bookkeeping.
type Queue struct {
	Store *Store
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

// CreateRequest queues a chat request for the lieutenant.
func (q Queue) CreateRequest(body string) (string, error) {
	return q.Store.CreateRequest(body)
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-21T23:18:09+02:00","module_hash":"61ca5bdfe172e71906a52d8b6fa491e2585e74bc9c9b1f255d28d05eb7a38a46","functions":[{"id":"func/Queue.Requests","name":"Queue.Requests","line":13,"end_line":27,"hash":"b91905a09bc1c384883a8271cc1bfb18a12ef831c100bf4dab704be7a5cba86f"},{"id":"func/Queue.CreateRequest","name":"Queue.CreateRequest","line":30,"end_line":32,"hash":"4567248275920b2e51926aa38488d3e80b240d2be7351cff9c0e29645726925a"}]}
// mutate4go-manifest-end
