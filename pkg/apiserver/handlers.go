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
	// Contextcheck // False positive. We don't want to use the request context to start the job in the background.
	jobID, err := s.jobmanager.StartJob(user, req.GetCommand(), req.GetArgs())
	if err != nil {
		return nil, fmt.Errorf("job manager start job: : %w", err)
	}
	return &pb.StartJobResponse{JobId: jobID.String()}, nil
}

func (s *Server) StopJob(_ context.Context, req *pb.StopJobRequest) (*pb.StopJobResponse, error) {
	jobID, _ := uuid.Parse(req.GetJobId()) // Already validated in middleware.

	if err := s.jobmanager.StopJob(jobID); err != nil {
		return nil, fmt.Errorf("job manager stop job: %w", err)
	}

	return &pb.StopJobResponse{}, nil
}

func (s *Server) GetJobStatus(_ context.Context, req *pb.GetJobStatusRequest) (*pb.GetJobStatusResponse, error) {
	jobID, _ := uuid.Parse(req.GetJobId()) // Already validated in middleware.
	job, err := s.jobmanager.LookupJob(jobID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "lookup job: %s", err)
	}
	resp := &pb.GetJobStatusResponse{Status: job.Status()}
	if resp.GetStatus() != pb.JobStatus_JOB_STATUS_RUNNING {
		//nolint:gosec // False positive about int/int32 conversion, but in POSIX, exit codes are actually uint8.
		exitCode := int32(job.ExitCode())
		resp.ExitCode = &exitCode
	}
	return resp, nil
}

func (s *Server) StreamLogs(req *pb.StreamLogsRequest, ss grpc.ServerStreamingServer[pb.StreamLogsResponse]) error {
	ctx := ss.Context()
	jobID, _ := uuid.Parse(req.GetJobId()) // Already validated in middleware.

	r, err := s.jobmanager.StreamLogs(ctx, jobID)
	if err != nil {
		return fmt.Errorf("job manager stream logs: %w", err)
	}

	buf := make([]byte, 32*1024) //nolint:mnd // Default value from io.Copy, reasonable.
	for {
		n, err := r.Read(buf)
		if n > 0 {
			if err := ss.Send(&pb.StreamLogsResponse{Data: buf[:n]}); err != nil {
				return fmt.Errorf("send log entry: %w", err)
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) {
				return nil
			}
			return fmt.Errorf("consume logs: %w", err)
		}
	}
}
