package telepilot_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestStartStop(t *testing.T) {
	t.Parallel()

	ts := newTestServer(t)

	ctx := context.Background()

	// Create a Job.
	jobID, err := ts.alice.StartJob(ctx, "true", nil)
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
	})
}

func TestInvalidStart(t *testing.T) {
	t.Parallel()

	ts := newTestServer(t)

	ctx := context.Background()

	// Create a Job.
	jobID, err := ts.alice.StartJob(ctx, "/", nil)
	if err == nil {
		t.Fatal("Starting invalid job should fail but didn't.")
	}
	assert(t, uuid.Nil, jobID, "invalid job id")
}
