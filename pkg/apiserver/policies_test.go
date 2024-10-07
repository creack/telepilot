package apiserver //nolint:testpackage // Expected to test the internal package to validate policy count.

import (
	"testing"

	pb "go.creack.net/telepilot/api/v1"
)

// Make sure we always have as many polcies as handlers.
func TestPolicyCount(t *testing.T) {
	t.Parallel()

	// Create a set of the known methods/streams.
	tmp := map[string]struct{}{}
	for _, ep := range pb.TelePilotService_ServiceDesc.Methods {
		tmp["/"+pb.TelePilotService_ServiceDesc.ServiceName+"/"+ep.MethodName] = struct{}{}
	}
	for _, ep := range pb.TelePilotService_ServiceDesc.Streams {
		tmp["/"+pb.TelePilotService_ServiceDesc.ServiceName+"/"+ep.StreamName] = struct{}{}
	}

	// Go over our policies and remove what we have from the set.
	for ep := range policies {
		if _, ok := tmp[ep]; !ok {
			t.Errorf("Left over unused policy for endpoint %q.", ep)
		}
		delete(tmp, ep)
	}

	// Make sure we don't have any left-over.
	if len(tmp) != 0 {
		t.Fatalf("Missing policies for TelePilotService methods/streams: %v.", tmp)
	}
}
