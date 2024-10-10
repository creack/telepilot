package apiserver_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io"
	"os"
	"path"
	"syscall"
	"testing"
	"time"

	pb "go.creack.net/telepilot/api/v1"
	"go.creack.net/telepilot/pkg/apiserver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

type mockedService struct {
	w io.Writer
	grpc.ServerStreamingServer[pb.StreamLogsResponse]
	ready chan struct{}
}

func (f *mockedService) Context() context.Context { return context.Background() }

func (f *mockedService) Send(resp *pb.StreamLogsResponse) error {
	data := resp.GetData()
	if data == nil {
		close(f.ready)
		return nil
	}
	_, err := f.w.Write(data)
	return err //nolint:wrapcheck // No need to wrap here.
}

// func TestStreamLogsHistorical(t *testing.T) {
// 	t.Parallel()

// 	server := apiserver.NewServer()
// 	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
// 	defer cancel()

// 	ctx = peer.NewContext(ctx, &peer.Peer{
// 		AuthInfo: credentials.TLSInfo{
// 			State: tls.ConnectionState{
// 				PeerCertificates: []*x509.Certificate{
// 					{
// 						Subject: pkix.Name{
// 							CommonName: "alice",
// 						},
// 					},
// 				},
// 			},
// 		},
// 	})

// 	resp, err := server.StartJob(ctx, &pb.StartJobRequest{
// 		Command: "sh",
// 		Args:    []string{"-c", "echo hello"},
// 	})
// 	if err != nil {
// 		t.Fatalf("Failed to start job: %s.", err)
// 	}
// 	jobID := resp.GetJobId()

// 	w := bytes.NewBuffer(nil)
// 	if err := server.StreamLogs(
// 		&pb.StreamLogsRequest{JobId: jobID},
// 		&mockedService{w: w},
// 	); err != nil {
// 		t.Fatalf("Failed to stream logs: %s.", err)
// 	}

// 	w.Reset()
// 	if err := server.StreamLogs(
// 		&pb.StreamLogsRequest{JobId: jobID},
// 		&mockedService{w: w},
// 	); err != nil {
// 		t.Fatalf("Failed to stream logs: %s.", err)
// 	}

// 	if expect, got := "hello\n", w.String(); expect != got {
// 		t.Fatalf("Assert fail.\nExpect:\t%s\nGot:\t%s\n", expect, got)
// 	}
// }

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

func TestStreamLogsLive(t *testing.T) {
	t.Parallel()

	server := apiserver.NewServer()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ctx = peer.NewContext(ctx, &peer.Peer{
		AuthInfo: credentials.TLSInfo{
			State: tls.ConnectionState{
				PeerCertificates: []*x509.Certificate{
					{
						Subject: pkix.Name{
							CommonName: "alice",
						},
					},
				},
			},
		},
	})

	pipeName := mkTempPipe(t, "pipe")

	resp, err := server.StartJob(ctx, &pb.StartJobRequest{
		Command: "sh",
		Args:    []string{"-c", "cat " + pipeName + "; echo hello"},
	})
	if err != nil {
		t.Fatalf("Failed to start job: %s.", err)
	}
	jobID := resp.GetJobId()

	r, w := io.Pipe()
	mw := &mockedService{w: w, ready: make(chan struct{})}

	chStreamLogs := make(chan error, 1)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		select {
		case err := <-chStreamLogs:
			noError(t, err, "Stream Logs.")
		case <-ctx.Done():
			t.Fatal("timeout waiting on stream logs")
		}
	})
	go func() {
		defer close(chStreamLogs)
		chStreamLogs <- server.StreamLogs(&pb.StreamLogsRequest{JobId: jobID}, mw)
	}()

	select {
	case <-ctx.Done():
		t.Fatal("Wait on StreamLogs to be ready.")
	case <-mw.ready:
	}

	noError(t, os.WriteFile(pipeName, nil, 0), "unblock pipe")

	chRead := make(chan error, 1)
	buf := make([]byte, len("hello\n"))
	go func() {
		defer close(chRead)
		_, err := io.ReadFull(r, buf)
		chRead <- err
	}()
	select {
	case <-ctx.Done():
		t.Fatal("timeout reading data")
	case err := <-chRead:
		noError(t, err, "read data")
		assert(t, "hello\n", string(buf), "assert data")

	}
}
