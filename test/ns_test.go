package telepilot_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	pb "go.creack.net/telepilot/api/v1"
)

func TestPIDNamespace(t *testing.T) {
	t.Parallel()

	ts, ctx := newTestServer(t)

	t.Run("ps", func(t *testing.T) {
		t.Parallel()

		jobID, err := ts.alice.StartJob(ctx, "ps", []string{"-e"})
		noError(t, err, "Start job.")
		t.Cleanup(func() { noError(t, ts.alice.StopJob(ctx, jobID), "Cleanup stop job.") })

		w := &strings.Builder{}
		noError(t, ts.alice.StreamLogs(ctx, jobID, w), "Stream logs.")
		// We expect only 2 lines: the headers and the lone process in the namesapce.
		if l := len(strings.Split(strings.TrimSpace(w.String()), "\n")); l != 2 {
			t.Fatalf("Unexpected line count for `ps -e`: %d.\n%s\n", l, w)
		}
	})
	t.Run("getpid", func(t *testing.T) {
		t.Parallel()

		jobID, err := ts.alice.StartJob(ctx, "sh", []string{"-c", "echo $$"})
		noError(t, err, "Start job.")
		t.Cleanup(func() { noError(t, ts.alice.StopJob(ctx, jobID), "Cleanup stop job.") })

		w := &strings.Builder{}
		noError(t, ts.alice.StreamLogs(ctx, jobID, w), "Stream logs.")
		// We expect to be pid 1.
		assert(t, "1", strings.TrimSpace(w.String()), "invalid pid")
	})
}

func TestNetworkNamespace(t *testing.T) {
	t.Parallel()

	ts, ctx := newTestServer(t)

	t.Run("iface", func(t *testing.T) {
		t.Parallel()

		jobID, err := ts.alice.StartJob(ctx, "ip", []string{"address", "show"})
		noError(t, err, "Start job.")
		t.Cleanup(func() { noError(t, ts.alice.StopJob(ctx, jobID), "Cleanup stop job.") })

		w := &strings.Builder{}
		noError(t, ts.alice.StreamLogs(ctx, jobID, w), "Stream logs.")
		st, err := ts.alice.GetJobStatus(ctx, jobID)
		noError(t, err, "Get job status.")
		if st != pb.JobStatus_JOB_STATUS_EXITED.String()+" (0)" {
			t.Skip("ip exited with error, likely missing")
		}
		// We expect only 1 entry of 2 lines: the loopback.
		if l := len(strings.Split(strings.TrimSpace(w.String()), "\n")); l != 2 {
			t.Fatalf("Unexpected iface count: %d\n", l)
		}
		if !strings.Contains(w.String(), "1: lo:") {
			t.Fatal("Invalid network interface output. No loopback.")
		}
	})
	t.Run("ping", func(t *testing.T) {
		t.Parallel()

		// Attempt to ping Cloudflare.
		// NOTE: Depending on the system, ping may exit different codes. Wrap it in a shell to control
		// the exit code.
		jobID, err := ts.alice.StartJob(ctx, "sh", []string{"-c", "ping 1.1.1.1 -c 1 || exit 1"})
		noError(t, err, "Start job.")
		t.Cleanup(func() { noError(t, ts.alice.StopJob(ctx, jobID), "Cleanup stop job.") })

		w := &strings.Builder{}
		noError(t, ts.alice.StreamLogs(ctx, jobID, w), "Stream logs.")

		st, err := ts.alice.GetJobStatus(ctx, jobID)
		noError(t, err, "Get job status.")
		assert(t, pb.JobStatus_JOB_STATUS_EXITED.String()+" (1)", st, "invalid status")
	})
}

func TestMountNamespace(t *testing.T) {
	t.Parallel()

	ts, ctx := newTestServer(t)

	// NOTE: Once we implement a pivot-root, this won't work and we will need
	// to have a mount-point relative to the new root.
	mountPoint := t.TempDir()

	// Add a mountpoint in the job, dump the mount table, wait on a pipe and then cleanup.
	jobID1, err := ts.alice.StartJob(ctx, "sh", []string{
		"-c",
		"mount -t tmpfs tmpfs " + mountPoint + " && mount && sleep 5",
	})
	noError(t, err, "Start job.")

	r, w := io.Pipe()
	ch := make(chan error, 1)
	go func() { ch <- ts.alice.StreamLogs(ctx, jobID1, w) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		select {
		case <-ctx.Done():
			t.Error("Timeout waiting for StreamLogs to end.")
		case err := <-ch:
			noError(t, err, "Stream Logs")
		}
	})
	t.Cleanup(func() { noError(t, ts.alice.StopJob(ctx, jobID1), "Cleanup stop job.") })

	buf := make([]byte, 32*1024) // Default from io.Copy. Reasonable. Even for large mount table.
	for {
		n, err := r.Read(buf)
		if errors.Is(err, io.EOF) {
			t.Fatal("Mountpoint missing from job output.")
		}
		noError(t, err, "Read job logs.")
		if strings.Contains(string(buf[:n]), "tmpfs on "+mountPoint) {
			break
		}
	}

	// While still hanging on the pipe, start a new job and check the mount table.
	{
		jobID2, err := ts.alice.StartJob(ctx, "mount", nil)
		noError(t, err, "Start job.")
		t.Cleanup(func() { noError(t, ts.alice.StopJob(ctx, jobID2), "Cleanup stop job.") })

		w := &strings.Builder{}
		noError(t, ts.alice.StreamLogs(ctx, jobID2, w), "Stream logs.")
		if strings.Contains(w.String(), mountPoint) {
			t.Fatal("Mountpoint from different job present in mount table.")
		}
	}

	// Assert that we were indeed still running.
	st, err := ts.alice.GetJobStatus(ctx, jobID1)
	noError(t, err, "Get job status.")
	assert(t, pb.JobStatus_JOB_STATUS_RUNNING.String(), st, "invalid status")
}
