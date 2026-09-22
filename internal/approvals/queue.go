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
