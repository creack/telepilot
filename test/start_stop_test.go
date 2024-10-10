package telepilot_test

import (
	"io"
	"os"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "go.creack.net/telepilot/api/v1"
)

// Make sure that we can call start and stop without errors.
func TestStartStop(t *testing.T) {
	t.Parallel()

	ts, ctx := newTestServer(t)

	// Create a Job.
	jobID, err := ts.alice.StartJob(ctx, os.Args[0], nil)
	noError(t, err, "Alice start job.")

	// Stop the Job from the same user.
	t.Run("happy alice", func(t *testing.T) {
		t.Parallel()

		noError(t, ts.alice.StopJob(ctx, jobID), "Stop Job.")

		// Trying to stop again should have no effect and no error.
		noError(t, ts.alice.StopJob(ctx, jobID), "Stop Job.")
	})

	// Attempt to stop the job from a different user.
	t.Run("sad bob", func(t *testing.T) {
		t.Parallel()

		st, ok := status.FromError(ts.bob.StopJob(ctx, jobID))
		assert(t, true, ok, "extract grpc status from error")
		assert(t, codes.PermissionDenied, st.Code(), "invalid grpc status code")

		// Attempt to stop a non-existing job.
		st, ok = status.FromError(ts.bob.StopJob(ctx, uuid.New().String()))
		assert(t, true, ok, "extract grpc status from error")
		assert(t, codes.PermissionDenied, st.Code(), "invalid grpc status code")
	})
}

func TestInvalidStart(t *testing.T) {
	t.Parallel()

	ts, ctx := newTestServer(t)

	// Create a Job.
	jobID, err := ts.alice.StartJob(ctx, "/", nil)
	noError(t, err, "Invalid job should still initially work, creating the job and id.")

	// Make sure to wait for the job to end.
	noError(t, ts.alice.StreamLogs(ctx, jobID, io.Discard), "Wait for job to end.")

	// Assert that the job failed.
	st, err := ts.alice.GetJobStatus(ctx, jobID)
	noError(t, err, "Get job status.")
	assert(t, pb.JobStatus_JOB_STATUS_EXITED.String()+" (1)", st, "invalid failed job status")
}
