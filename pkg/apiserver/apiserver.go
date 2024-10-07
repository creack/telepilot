package apiserver

import (
	"errors"
	"fmt"

	pb "go.creack.net/telepilot/api/v1"
	"go.creack.net/telepilot/pkg/cgroups"
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

func NewServer() (*Server, error) {
	if err := cgroups.InitialSetup(); err != nil {
		return nil, fmt.Errorf("cgroups initial setup: %w", err)
	}

	return &Server{
		jobmanager: jobmanager.NewJobManager(),
	}, nil
}
