package apiserver

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "go.creack.net/telepilot/api/v1"
)

func (s *Server) StartJob(ctx context.Context, req *pb.StartJobRequest) (*pb.StartJobResponse, error) {
	user, err := getUserFromContext(ctx)
	if err != nil {
		// NOTE: Not supposed to happen as already checked, but check anyway.
		return nil, fmt.Errorf("getUserFromContext: %w", err)
	}
	jobID, err := s.jobmanager.StartJob(user, req.GetCommand(), req.GetArgs())
	if err != nil {
		return nil, fmt.Errorf("jobmanager start job: : %w", err)
	}
	return &pb.StartJobResponse{JobId: jobID.String()}, nil
}
