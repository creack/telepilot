package apiclient

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "go.creack.net/telepilot/api/v1"
)

// NewClient returns a client prepared to connect to the server.
func NewClient(tlsConfig *tls.Config, addr string) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if err != nil {
		return nil, fmt.Errorf("grpc new client: %w", err)
	}
	return conn, nil
}

func GetJobStatus(ctx context.Context, client pb.TelePilotServiceClient, jobID string) (string, error) {
	resp, err := client.GetJobStatus(ctx, &pb.GetJobStatusRequest{JobId: jobID})
	if err != nil {
		return "", err //nolint:wrapcheck // Only error path, no need for wrap here.
	}
	status := resp.GetStatus()
	if status == pb.JobStatus_JOB_STATUS_EXITED || status == pb.JobStatus_JOB_STATUS_STOPPED {
		return fmt.Sprintf("%s (%d)", status, resp.GetExitCode()), nil
	}
	return status.String(), nil
}

func StreamLogs(ctx context.Context, client pb.TelePilotServiceClient, jobID string, w io.Writer) error {
	stream, err := client.StreamLogs(ctx, &pb.StreamLogsRequest{JobId: jobID})
	if err != nil {
		return fmt.Errorf("call streamlogs: %w", err)
	}
	for {
		msg, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("recv log entry: %w", err)
		}
		_, _ = fmt.Fprint(w, string(msg.GetData())) // Best effort.
	}
}
