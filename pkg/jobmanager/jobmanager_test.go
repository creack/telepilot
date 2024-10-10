package jobmanager_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path"
	"syscall"
	"testing"

	pb "go.creack.net/telepilot/api/v1"
	"go.creack.net/telepilot/pkg/jobmanager"
)

// Helper to assert success.
func noError(t *testing.T, err error, msg string) {
	t.Helper()
	// Ignore EOF/Closed Pipe errors as it is commonly used to notify closure.
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("%s: %s.", msg, err)
	}
}

// Generic assert.
func assert[T comparable](t *testing.T, expect, got T, msg string) {
	t.Helper()
	if expect != got {
		t.Fatalf("Assert fail: %s:\n Expect:\t%v\n Got:\t%v", msg, expect, got)
	}
}

// mkTempPipe creates a unix pipe in a temp directory. Used to synchronize job with test.
// Returns the path of the pipe file.
// NOTE: Once we implement support for input, may be better to use that.
func mkTempPipe(t *testing.T, name string) string {
	t.Helper()

	pipeName := path.Join(t.TempDir(), name)
	// NOTE: POSIX compliant.
	noError(t, syscall.Mknod(pipeName, syscall.S_IFIFO|0o644, 0), "Create tmp pipe.")
	t.Cleanup(func() { noError(t, os.Remove(pipeName), "Cleanup tmp pipe.") })
	return pipeName
}

func TestStreamLogsHistorical(t *testing.T) {
	t.Parallel()

	jm := jobmanager.NewJobManager()

	jobID, err := jm.StartJob("alice", "sh", []string{"-c", "echo hello"})
	if err != nil {
		t.Fatalf("Start job %s.", err)
	}
	t.Cleanup(func() { _ = jm.StopJob(jobID) }) // Best effort.

	ctx := context.Background()

	{
		r, err := jm.StreamLogs(ctx, jobID)
		if err != nil {
			t.Fatalf("StreamLogs: %s.", err)
		}
		if _, err := io.ReadAll(r); err != nil {
			t.Fatalf("Read logs: %s.", err)
		}
		j, err := jm.LookupJob(jobID)
		if err != nil {
			t.Fatalf("Lookup Job: %s.", err)
		}
		if st := j.Status(); st != pb.JobStatus_JOB_STATUS_EXITED {
			t.Fatalf("Unexpected status after initial streamlogs: %s.", st)
		}
	}

	r, err := jm.StreamLogs(ctx, jobID)
	if err != nil {
		t.Fatalf("StreamLogs: %s.", err)
	}
	buf, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("Read logs: %s.", err)
	}
	if expect, got := "hello\n", string(buf); expect != got {
		t.Fatalf("Assert fail.\nEpect:\t%s\nGot:\t%s\n", expect, got)
	}
}

func TestStreamLogsLive(t *testing.T) {
	t.Parallel()

	jm := jobmanager.NewJobManager()

	// Pipe to sync the job with the test.
	pipeName := mkTempPipe(t, "pipe")

	jobID, err := jm.StartJob("alice", "sh", []string{"-c", "cat " + pipeName + "; echo hello"})
	if err != nil {
		t.Fatalf("Start job %s.", err)
	}
	t.Cleanup(func() { _ = jm.StopJob(jobID) }) // Best effort.

	ctx := context.Background()

	r, err := jm.StreamLogs(ctx, jobID)
	if err != nil {
		t.Fatalf("StreamLogs: %s.", err)
	}
	noError(t, os.WriteFile(pipeName, nil, 0), "Unblock initial pipe.")
	buf, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("Read logs: %s.", err)
	}
	if expect, got := "hello\n", string(buf); expect != got {
		t.Fatalf("Assert fail.\nEpect:\t%s\nGot:\t%s\n", expect, got)
	}
}

// type mockedService struct {
// 	w io.Writer
// 	grpc.ServerStreamingServer[pb.StreamLogsResponse]
// }

// func (f *mockedService) Context() context.Context { return context.Background() }

// func (f *mockedService) Send(resp *pb.StreamLogsResponse) error {
// 	data := resp.GetData()
// 	fmt.Printf("%s %s %s --- %p\n", time.Now().String(), "SENDING", string(data), f.w)
// 	defer println(time.Now().String(), "POST SENDING", string(data))
// 	_, err := f.w.Write(data)
// 	return err
// }

// func fooTestMe(r io.Reader, ss grpc.ServerStreamingServer[pb.StreamLogsResponse]) error {
// 	buf := make([]byte, 32*1024) //nolint:mnd // Default value from io.Copy, reasonable.
// 	for {
// 		n, err := r.Read(buf)
// 		if n > 0 {
// 			if err := ss.Send(&pb.StreamLogsResponse{Data: []byte(string(buf[:n]))}); err != nil {
// 				println("FAIL1!!!?")
// 				return fmt.Errorf("send log entry: %w", err)
// 			}
// 		}
// 		if err != nil {
// 			println("FAIL2!!!?", err.Error())
// 			// If the process dies while setting up StreamLogs, we can get a ErrClosedPipe.
// 			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) {
// 				return nil
// 			}
// 			return fmt.Errorf("consume logs: %w", err)
// 		}
// 	}
// }

// func TestStreamLogsLive2(t *testing.T) {
// 	t.Parallel()

// 	jm := jobmanager.NewJobManager()

// 	// Pipe to sync the job with the test.
// 	pipeName := mkTempPipe(t, "pipe")

// 	jobID, err := jm.StartJob("alice", "sh", []string{"-c", "cat " + pipeName + "; echo hello"})
// 	if err != nil {
// 		t.Fatalf("Start job %s.", err)
// 	}
// 	t.Cleanup(func() { _ = jm.StopJob(jobID) }) // Best effort.

// 	ctx := context.Background()

// 	r, err := jm.StreamLogs(ctx, jobID)
// 	if err != nil {
// 		t.Fatalf("StreamLogs: %s.", err)
// 	}
// 	w := bytes.NewBuffer(nil)
// 	go fooTestMe(r, &mockedService{w: w})
// 	os.WriteFile(pipeName, nil, 0)
// 	// buf, err := io.ReadAll(r)
// 	// if err != nil {
// 	// 	t.Fatalf("Read logs: %s.", err)
// 	// }
// 	if expect, got := "hello\n", string(w.String()); expect != got {
// 		t.Fatalf("Assert fail.\nEpect:\t%s\nGot:\t%s\n", expect, got)
// 	}
// }
