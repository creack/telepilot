package apiserver

import (
	pb "go.creack.net/telepilot/api/v1"
	"go.creack.net/telepilot/pkg/jobmanager"
)

//nolint:gochecknoglobals // Expected global.
var policies = map[string][]policyFct{
	pb.TelePilotService_StartJob_FullMethodName:     {policyAllowed},
	pb.TelePilotService_StopJob_FullMethodName:      {policySameOwner},
	pb.TelePilotService_GetJobStatus_FullMethodName: {policySameOwner},
	pb.TelePilotService_StreamLogs_FullMethodName:   {policySameOwner},
}

// Make sure we always have as many polcies as handlers.
//
//nolint:gochecknoinits // Expected init.
func init() {
	if len(policies) != len(pb.TelePilotService_ServiceDesc.Methods)+len(pb.TelePilotService_ServiceDesc.Streams) {
		panic("missing policies for TelePilotService methods/streams")
	}
}

func enforcePolicies(user string, job *jobmanager.Job, policies ...policyFct) bool {
	if len(policies) == 0 {
		return false
	}
	for _, p := range policies {
		if !p(user, job) {
			return false
		}
	}
	return true
}

type policyFct func(user string, job *jobmanager.Job) bool

// allow everything.
func policyAllowed(string, *jobmanager.Job) bool { return true }

// only allow if the user is set and matches the job's one.
func policySameOwner(user string, job *jobmanager.Job) bool {
	return user != "" && job != nil && user == job.Owner
}
