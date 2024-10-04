package apiserver //nolint:testpackage // Expected to test the internal package to validate policy count.

import (
	"testing"

	pb "go.creack.net/telepilot/api/v1"
)

// Make sure we always have as many polcies as handlers.
func TestPolicyCount(t *testing.T) {
	t.Parallel()
	if len(policies) != len(pb.TelePilotService_ServiceDesc.Methods)+len(pb.TelePilotService_ServiceDesc.Streams) {
		t.Fatal("missing policies for TelePilotService methods/streams")
	}
}
