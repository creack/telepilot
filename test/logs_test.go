package telepilot_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	pb "go.creack.net/telepilot/api/v1"
)

func TestStreamLogsSimple(t *testing.T) {
	t.Parallel()

	ts, ctx := newTestServer(t)

	t.Run("no output", func(t *testing.T) {
		t.Parallel()

		// Create a Job without output.
		jobID, err := ts.bob.StartJob(ctx, "true", nil)
		noError(t, err, "Bob start job.")

		// Stream logs and assert.
		w := bytes.NewBuffer(nil)
		noError(t, ts.bob.StreamLogs(ctx, jobID, w), "Stream Logs.")
		assert(t, "", w.String(), "invalid output")

		// One more time should yield same resutlt.
		w.Reset()
		noError(t, ts.bob.StreamLogs(ctx, jobID, w), "Stream Logs.")
		assert(t, "", w.String(), "invalid output")
	})

	t.Run("finite output", func(t *testing.T) {
		t.Parallel()

		// Create a Job with a few lines.
		jobID, err := ts.bob.StartJob(ctx, "sh", []string{"-c", "echo hello; echo world"})
		noError(t, err, "Bob start job.")

		// Wait for the process to end.
		noError(t, ts.bob.StreamLogs(ctx, jobID, io.Discard), "Stream Logs.")

		// Assert that the process is indeed exited.
		status, err := ts.bob.GetJobStatus(ctx, jobID)
		noError(t, err, "Get Job Status.")
		assert(t, pb.JobStatus_JOB_STATUS_EXITED.String()+" (0)", status, "unexpected status after stream logs")

		// Stream logs and assert.
		w := bytes.NewBuffer(nil)
		noError(t, ts.bob.StreamLogs(ctx, jobID, w), "Stream Logs.")
		assert(t, "hello\nworld\n", w.String(), "invalid output")
	})
}

// Test streaming logs of a long-running process.
func TestStreamLogsOngoingOutput(t *testing.T) {
	t.Parallel()

	ts, ctx := newTestServer(t)

	// Waitgroup to make sure all routines are done at the end.
	var wg sync.WaitGroup
	t.Cleanup(wg.Wait)

	// Pipe to sync the job with the test.
	pipeName := mkTempPipe(t, "pipe")

	// Create a Job with a few lines and keep the job alive.
	jobID, err := ts.bob.StartJob(
		ctx,
		"sh", []string{"-c", "echo hello; echo world; cat " + pipeName},
	)
	noError(t, err, "Bob start job.")
	t.Cleanup(func() { noError(t, ts.bob.StopJob(context.Background(), jobID), "Cleanup stop job.") })

	// Create a pipe to consume logs.
	logEntry, w := consumeFromPipe(ctx, t, 12)

	// Start streaming the logs.
	wg.Add(1)
	go func() {
		defer wg.Done()
		noError(t, ts.bob.StreamLogs(ctx, jobID, w), "Stream Logs.")
	}()

	// Wait for the read data and assert it.
	assertChanOnce(ctx, t, "hello\nworld\n", logEntry, "unexpected output")

	// Assert that the job is still running.
	jobStatus, err := ts.bob.GetJobStatus(ctx, jobID)
	noError(t, err, "Get Job Status.")
	assert(t, pb.JobStatus_JOB_STATUS_RUNNING.String(), jobStatus, "invalid job status")
}

func TestStreamLogsMultiClient(t *testing.T) {
	t.Parallel()

	ts, ctx := newTestServer(t)

	// Waitgroup to make sure all routines are done at the end.
	var wg sync.WaitGroup
	t.Cleanup(func() {
		ch := make(chan struct{})
		go func() {
			defer close(ch)
			wg.Wait()
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		select {
		case <-ch:
		case <-ctx.Done():
			t.Fatal("Timeout waiting for cleanup.")
		}
	})

	// Pipe to sync the job with the test.
	pipeName := mkTempPipe(t, "pipe")

	// Create a Job printing a line, wait on a pipe, then print another line.
	jobID, err := ts.bob.StartJob(
		ctx,
		"sh", []string{"-c", "echo hello; cat " + pipeName + " > /dev/null; echo world"},
	)
	noError(t, err, "Bob start job.")
	t.Cleanup(func() { noError(t, ts.bob.StopJob(context.Background(), jobID), "Cleanup stop job.") })

	logEntry1, w1 := consumeFromPipe(ctx, t, 6, 6)
	logEntry2, w2 := consumeFromPipe(ctx, t, 6, 6)

	// Start the two client Stream logs. Equates to running `telepilot -user bob logs <job_id>` twice in parallel.
	wg.Add(2)
	go func() { defer wg.Done(); noError(t, ts.bob.StreamLogs(ctx, jobID, w1), "Stream Logs 1.") }()
	go func() { defer wg.Done(); noError(t, ts.bob.StreamLogs(ctx, jobID, w2), "Stream Logs 2.") }()

	// Wait for the read data and assert it. Expect only the first part of the logs.
	assertChanOnce(ctx, t, "hello\n", logEntry1, "unexpected first output 1")
	assertChanOnce(ctx, t, "hello\n", logEntry2, "unexpected first output 2")

	// Unblock the job.
	ch := make(chan error, 1)
	go func() { ch <- os.WriteFile(pipeName, nil, 0) }()
	select {
	case <-ctx.Done():
		t.Fatal("Timeout waiting on writing to pipe.")
	case err := <-ch:
		noError(t, err, "Unblock pipe.")
	}

	// Wait for the read data and assert it. Expect only the second part without
	// the beginning as we are still running the same stream logs command.
	// If we ran the command again, we would expect the full output, we have a different test for that.
	assertChanOnce(ctx, t, "world\n", logEntry1, "unexpected second output 1")
	assertChanOnce(ctx, t, "world\n", logEntry2, "unexpected second output 2")
}
