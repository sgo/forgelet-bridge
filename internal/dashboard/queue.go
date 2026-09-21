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
