package approvals

import (
	"fmt"

	"github.com/unclebob/forgelet-bridge/internal/relay"
)

// Queue is the approvals side of a forge seen the way the bridge's relay needs
// it: the approvals waiting for the operator, and the decisions it makes.
type Queue struct {
	Store *Store
}

// Pending lists every approval the forge's open projects are waiting for.
func (q Queue) Pending() ([]relay.Approval, error) {
	pending, err := q.Store.Pending()
	if err != nil {
		return nil, err
	}
	relayed := make([]relay.Approval, 0, len(pending))
	for _, approval := range pending {
		relayed = append(relayed, relay.Approval{
			Key:       Key(approval.Project, approval.ID),
			Project:   approval.Project,
			ID:        approval.ID,
			Card:      approval.Card,
			Gate:      approval.Gate,
			Artifacts: approval.Artifacts,
		})
	}
	return relayed, nil
}

// Approve approves an approval the way the desktop dashboard does.
func (q Queue) Approve(project, id string) error {
	return q.Store.Approve(project, id)
}

// SendBack records the operator's feedback and hands the card back.
func (q Queue) SendBack(project, id, feedback string) error {
	return q.Store.SendBack(project, id, feedback)
}

// Key is how an approval is named across the bridge: project and handoff id,
// so two projects cannot be confused.
func Key(project, id string) string {
	return fmt.Sprintf("%s/%s", project, id)
}

// mutate4go-manifest-begin
// {"version":1,"tested_at":"2026-09-22T15:41:43+02:00","module_hash":"4bc28fcc6f2a93b1b83105348bc1df8e1723d563486e69ce3c8f14f40b8db21f","functions":[{"id":"func/Queue.Pending","name":"Queue.Pending","line":16,"end_line":33,"hash":"cc810dd66357863f0461e709dceb4513ccb5e634db5999975625a361a5811178"},{"id":"func/Queue.Approve","name":"Queue.Approve","line":36,"end_line":38,"hash":"ca5e5e31e26a0427f30802fcd48028b6eaebc91d25b32ab4f6184b28c98b5334"},{"id":"func/Queue.SendBack","name":"Queue.SendBack","line":41,"end_line":43,"hash":"db53516dda5b6d3edf24d5c98c336effe549007643215cacbd7af8b5806db683"},{"id":"func/Key","name":"Key","line":47,"end_line":49,"hash":"68e43f3e4ead2972e95508918766fe1a2d42375378089b9839f72dfe6360aa27"}]}
// mutate4go-manifest-end
