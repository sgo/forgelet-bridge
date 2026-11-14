package relay

// DashboardSnapshot is the waiting work derived from one dashboard state
// response. The bridge reads it once for a tick and hands its fields to the
// approval and clarification consumers that used to read the endpoint
// independently.
type DashboardSnapshot struct {
	Approvals         []Approval
	ResolvedApprovals map[string]string
	Clarifications    []Clarification
}
