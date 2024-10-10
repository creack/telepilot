package telepilot_test

import (
	"context"
	"io"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.creack.net/telepilot/pkg/apiclient"
)

func TestAuthorization(t *testing.T) {
	t.Parallel()

	ts, ctx := newTestServer(t)

	// Create a Job.
	aliceJobID1, err := ts.alice.StartJob(ctx, "true", nil)
	noError(t, err, "Alice start job 1.")
	aliceJobID2, err := ts.alice.StartJob(ctx, "false", nil)
	noError(t, err, "Alice start job 2.")
	bobJobID, err := ts.bob.StartJob(ctx, "true", nil)
	noError(t, err, "Bob start job.")

	t.Run("happy alice", func(t *testing.T) {
		t.Parallel()

		// Try all the endpoints, make sure we have no errors.
		_, err := ts.alice.GetJobStatus(ctx, aliceJobID1)
		noError(t, err, "Get Job Status 1.")
		noError(t, ts.alice.StreamLogs(ctx, aliceJobID1, io.Discard), "Stream Logs 1.")
		noError(t, ts.alice.StopJob(ctx, aliceJobID1), "Stop Job 1.")

		// Trying again to make sure it wasn't a one-off thing.
		_, err = ts.alice.GetJobStatus(ctx, aliceJobID1)
		noError(t, err, "Get Job Status 1.")
		noError(t, ts.alice.StreamLogs(ctx, aliceJobID1, io.Discard), "Stream Logs 1.")
		noError(t, ts.alice.StopJob(ctx, aliceJobID1), "Stop Job 1.")

		// Try all the endpoints on a different job.
		_, err = ts.alice.GetJobStatus(ctx, aliceJobID2)
		noError(t, err, "Get Job Status 2.")
		noError(t, ts.alice.StreamLogs(ctx, aliceJobID2, io.Discard), "Stream Logs 2.")
		noError(t, ts.alice.StopJob(ctx, aliceJobID2), "Stop Job.")
	})

	t.Run("happy bob", func(t *testing.T) {
		t.Parallel()

		// Try all the endpoints, make sure we have no errors.
		_, err := ts.bob.GetJobStatus(ctx, bobJobID)
		noError(t, err, "Get Job Status 1.")
		noError(t, ts.bob.StreamLogs(ctx, bobJobID, io.Discard), "Stream Logs 1.")
		noError(t, ts.bob.StopJob(ctx, bobJobID), "Stop Job 1.")
	})

	t.Run("sad alice", func(t *testing.T) {
		t.Parallel()
		sadAuthorization(t, ts.alice, bobJobID)
	})
	t.Run("sad bob 1", func(t *testing.T) {
		t.Parallel()
		sadAuthorization(t, ts.bob, aliceJobID1)
	})
	t.Run("sad bob 2", func(t *testing.T) {
		t.Parallel()
		sadAuthorization(t, ts.bob, aliceJobID2)
	})
}

// Sad path test from the given client to the given job id.
func sadAuthorization(t *testing.T, client *apiclient.Client, jobID string) {
	t.Helper()

	ctx := context.Background()

	// Try all the endpoints.
	st, ok := status.FromError(client.StopJob(ctx, jobID))
	assert(t, true, ok, "extract grpc status from stop job error")
	assert(t, codes.PermissionDenied, st.Code(), "invalid grpc status code for stop job")

	st, ok = status.FromError(client.StreamLogs(ctx, jobID, io.Discard))
	assert(t, true, ok, "extract grpc status from stream logs error")
	assert(t, codes.PermissionDenied, st.Code(), "invalid grpc status code for stream logs")

	_, err := client.GetJobStatus(ctx, jobID)
	st, ok = status.FromError(err)
	assert(t, true, ok, "extract grpc status from get job status error")
	assert(t, codes.PermissionDenied, st.Code(), "invalid grpc status code for get job status")
}
