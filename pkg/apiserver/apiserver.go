package apiserver

import (
	"errors"

	pb "go.creack.net/telepilot/api/v1"
	"go.creack.net/telepilot/pkg/jobmanager"
)

// Common errors.
var (
	ErrInvalidClientCerts = errors.New("invalid client certifiate")
)

// Server is used to implement api.TelePilotService.
type Server struct {
	pb.UnimplementedTelePilotServiceServer

	jobmanager *jobmanager.JobManager
}

// Create the server.
// NOTE: As this creates a new job manager, it expected
// the cgroup to be initialized via cgroups.InitalSetup()
// before being ready to use.
func NewServer() *Server {
	return &Server{
		jobmanager: jobmanager.NewJobManager(),
	}
}
