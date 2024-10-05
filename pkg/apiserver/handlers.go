package apiserver

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"
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
	//nolint:contextcheck // False positive. We don't want to use the request context to start the job in the background.
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

func (s *Server) StreamLogs(req *pb.StreamLogsRequest, ss grpc.ServerStreamingServer[pb.StreamLogsResponse]) error {
	ctx := ss.Context()
	jobID, _ := uuid.Parse(req.GetJobId()) // Already validated in middleware.

	r, err := s.jobmanager.StreamLogs(ctx, jobID)
	if err != nil {
		return fmt.Errorf("job manager stream logs: %w", err)
	}

	buf := make([]byte, 32*1024) //nolint:mnd // Default value from io.Copy, reasonable.
loop:
	n, err := r.Read(buf)
	if err != nil {
		if errors.Is(err, io.ErrClosedPipe) {
			return nil
		}
		return fmt.Errorf("consume logs: %w", err)
	}
	// TODO: Investigate this, doesn't smell right, probably need to make a copy of the bufer.
	if err := ss.Send(&pb.StreamLogsResponse{Data: buf[:n]}); err != nil {
		return fmt.Errorf("send log entry: %w", err)
	}
	goto loop
}
