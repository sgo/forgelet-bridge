package dashboard

import "testing"

// TestGateNamesTheHandoverRolesOnlyWhenBothAreReported checks the gate the
// operator reads: the forge's roles when it reports both, otherwise what the
// forge said about the gate.
func TestGateNamesTheHandoverRolesOnlyWhenBothAreReported(t *testing.T) {
	cases := map[string]struct {
		request ApprovalRequest
		want    string
	}{
		"both roles": {ApprovalRequest{From: "coder", To: "refactorer"}, "coder → refactorer"},
		"only from":  {ApprovalRequest{From: "coder", Gate: "spec → refactorer"}, "spec → refactorer"},
		"only to":    {ApprovalRequest{To: "refactorer", Gate: "spec → refactorer"}, "spec → refactorer"},
		"neither":    {ApprovalRequest{Gate: "spec → refactorer"}, "spec → refactorer"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := tc.request.gate(); got != tc.want {
				t.Errorf("gate() = %q, want %q", got, tc.want)
			}
		})
	}
}
