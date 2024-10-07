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

// Client is our client business logic. It is the high level API Client.
type Client struct {
	conn   *grpc.ClientConn
	client pb.TelePilotServiceClient
}

// NewClient returns a client prepared to connect to the server.
func NewClient(tlsConfig *tls.Config, addr string) (*Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if err != nil {
		return nil, fmt.Errorf("grpc new client: %w", err)
	}
	return &Client{conn: conn, client: pb.NewTelePilotServiceClient(conn)}, nil
}

// Close the connection if connected.
func (c *Client) Close() error {
	return c.conn.Close() //nolint:wrapcheck // No wrap needed here.
}

func (c *Client) StartJob(ctx context.Context, cmd string, args []string) (string, error) {
	resp, err := c.client.StartJob(ctx, &pb.StartJobRequest{Command: cmd, Args: args})
	if err != nil {
		return "", fmt.Errorf("call start job: %w", err)
	}
	return resp.GetJobId(), nil
}

func (c *Client) StopJob(ctx context.Context, jobID string) error {
	_, err := c.client.StopJob(ctx, &pb.StopJobRequest{JobId: jobID})
	return err //nolint:wrapcheck // Only error path, no need for wrap here.
}

func (c *Client) GetJobStatus(ctx context.Context, jobID string) (string, error) {
	resp, err := c.client.GetJobStatus(ctx, &pb.GetJobStatusRequest{JobId: jobID})
	if err != nil {
		return "", err //nolint:wrapcheck // Only error path, no need for wrap here.
	}
	status := resp.GetStatus()
	if status == pb.JobStatus_JOB_STATUS_EXITED || status == pb.JobStatus_JOB_STATUS_STOPPED {
		return fmt.Sprintf("%s (%d)", status, resp.GetExitCode()), nil
	}
	return status.String(), nil
}

func (c *Client) StreamLogs(ctx context.Context, jobID string, w io.Writer) error {
	stream, err := c.client.StreamLogs(ctx, &pb.StreamLogsRequest{JobId: jobID})
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
