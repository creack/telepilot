package telepilot_test

import (
	"context"
	"fmt"
	"io"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "go.creack.net/telepilot/api/v1"
)

func TestRunningJobStatus(t *testing.T) {
	t.Parallel()

	ts := newTestServer(t)

	ctx := context.Background()

	// Create a long running job.
	jobID, err := ts.alice.StartJob(ctx, "sh", []string{"-c", "while true; do sleep 1; done"})
	noError(t, err, "Alice start job.")

	t.Run("happy alice", func(t *testing.T) {
		t.Parallel()

		{
			jobStatus, err := ts.alice.GetJobStatus(ctx, jobID)
			noError(t, err, "Get Job Status.")
			assert(t, pb.JobStatus_JOB_STATUS_RUNNING.String(), jobStatus, "invalid job status")
		}

		noError(t, ts.alice.StopJob(ctx, jobID), "Stop Job.")

		{
			jobStatus, err := ts.alice.GetJobStatus(ctx, jobID)
			noError(t, err, "Get Job Status.")
			assert(t, fmt.Sprintf("%s (-1)", pb.JobStatus_JOB_STATUS_STOPPED), jobStatus, "invalid job status")
		}
	})

	// Attempt to get the status from a different user.
	t.Run("sad dave", func(t *testing.T) {
		t.Parallel()

		_, err := ts.bob.GetJobStatus(ctx, jobID)
		st, ok := status.FromError(err)
		assert(t, true, ok, "extract grpc status from error")
		assert(t, codes.PermissionDenied, st.Code(), "invalid grpc status code")
	})
}

func TestExitedJobStatus(t *testing.T) {
	t.Parallel()

	ts := newTestServer(t)

	ctx := context.Background()

	const exitCode = 12

	// Create a long running job.
	jobID, err := ts.alice.StartJob(ctx, "sh", []string{"-c", fmt.Sprintf("exit %d", exitCode)})
	noError(t, err, "Alice start job.")

	// Wait for the job to end. (as we didn't implement Wait, use StreamLogs and discard output).
	noError(t, ts.alice.StreamLogs(ctx, jobID, io.Discard), "Waiting for job to end.")

	// Assert that the status is Exited.
	{
		jobStatus, err := ts.alice.GetJobStatus(ctx, jobID)
		noError(t, err, "Get Job Status.")
		assert(t, fmt.Sprintf("%s (%d)", pb.JobStatus_JOB_STATUS_EXITED, exitCode), jobStatus, "invalid job status")
	}

	// Stop the job even though it is exited.
	noError(t, ts.alice.StopJob(ctx, jobID), "Stop Job.")

	// Assert that the status didn't change.
	{
		jobStatus, err := ts.alice.GetJobStatus(ctx, jobID)
		noError(t, err, "Get Job Status.")
		assert(t, fmt.Sprintf("%s (%d)", pb.JobStatus_JOB_STATUS_EXITED, exitCode), jobStatus, "invalid job status")
	}
}
