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

func (s *Server) StopJob(_ context.Context, _ *pb.StopJobRequest) (*pb.StopJobResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method StopJob not implemented")
}

func (s *Server) GetJobStatus(_ context.Context, _ *pb.GetJobStatusRequest) (*pb.GetJobStatusResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetJobStatus not implemented")
}

func (s *Server) StreamLogs(_ *pb.StreamLogsRequest, _ grpc.ServerStreamingServer[pb.StreamLogsResponse]) error {
	return status.Errorf(codes.Unimplemented, "method StreamLogs not implemented")
}
