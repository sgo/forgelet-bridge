package fixtures

import (
	"io"
	"net/http"
	"testing"

	"github.com/unclebob/forgelet-bridge/internal/dashboard"
)

func TestFakeDashboardCountsStateRequestsAndBytes(t *testing.T) {
	fake := StartFakeDashboard()
	defer fake.Close()
	fake.SetState(dashboard.State{Approvals: []dashboard.ApprovalRequest{{ID: "one"}}})

	for range 2 {
		response, err := http.Get(fake.URL() + "/api/state")
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		if len(body) == 0 {
			t.Fatal("fake dashboard returned no state")
		}
	}
	counts := fake.Counts()
	if counts.Requests != 2 || counts.Failed != 0 || counts.Bytes == 0 {
		t.Errorf("counts = %+v, want two successful requests with bytes", counts)
	}
}

func TestFakeDashboardCountsFailedStateReads(t *testing.T) {
	fake := StartFakeDashboard()
	defer fake.Close()
	fake.SetFailure(true)

	response, err := http.Get(fake.URL() + "/api/state")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want service unavailable", response.StatusCode)
	}
	counts := fake.Counts()
	if counts.Requests != 1 || counts.Failed != 1 || counts.Bytes != 0 {
		t.Errorf("counts = %+v, want one failed read and no body", counts)
	}
}
