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

func NewServer() *Server {
	return &Server{
		jobmanager: jobmanager.NewJobManager(),
	}
}
