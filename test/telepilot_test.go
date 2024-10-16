package telepilot_test

import (
	"context"
	"crypto/tls"
	"flag"
	"io"
	"log/slog"
	"net"
	"os"
	"path"
	"syscall"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "go.creack.net/telepilot/api/v1"
	"go.creack.net/telepilot/pkg/apiclient"
	"go.creack.net/telepilot/pkg/apiserver"
	"go.creack.net/telepilot/pkg/cgroups"
	"go.creack.net/telepilot/pkg/initd"
	"go.creack.net/telepilot/pkg/tlsconfig"
)

func TestMain(m *testing.M) {
	isInit := flag.Bool("init", false, "internal flag to toggle init mode")
	flag.Parse()
	if *isInit {
		if err := initd.Init(flag.Args()); err != nil {
			slog.Error("Init error.", "error", err, "args", flag.Args())
			os.Exit(1)
		}
		return
	}

	if err := cgroups.InitialSetup(); err != nil {
		slog.Error("Failed to init cgroups.", "error", err)
		os.Exit(1)
	}

	ret := m.Run()
	os.Exit(ret)
}

// Helper to assert success.
func noError(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %s.", msg, err)
	}
}

// Generic assert.
func assert[T comparable](t *testing.T, expect, got T, msg string) {
	t.Helper()
	if expect != got {
		t.Fatalf("Assert fail: %s:\n Expect:\t'%v'\n Got:\t'%v'", msg, expect, got)
	}
}

func assertChanOnce(ctx context.Context, t *testing.T, expect string, ch <-chan []byte, msg string) {
	t.Helper()

	select {
	case <-ctx.Done():
		t.Fatal("Timeout waiting for chan.")
	case got := <-ch:
		assert(t, expect, string(got), msg)
	}
}

// Helper to load TLS Config from the certts dir.
// Requires the certs to be present, otherwise, skip the test.
// Run `make mtls` (or `make test`) to generate them.
func loadTLSConfig(t *testing.T, name string) *tls.Config {
	t.Helper()
	const certDir = "../certs"

	// Make sure we have the expected key. Just check for one, assume that the ca and cert are present if the key is there.
	if _, err := os.Stat(path.Join(certDir, name+".pem")); err != nil {
		t.Skip("Missing cert, skipping. Make sure to run `make mtls` first or invoke tests with `make test`.")
	}
	cfg, err := tlsconfig.LoadTLSConfig(
		path.Join(certDir, name+".pem"),
		path.Join(certDir, name+"-key.pem"),
		path.Join(certDir, "ca.pem"),
		name != "server",
	)
	noError(t, err, "Load certs for "+name)
	return cfg
}

// testServer wraps a running server listening on local host
// and a coupe of clients pointing to it.
type testServer struct {
	grpcServer *grpc.Server
	alice, bob *apiclient.Client
}

// newTestServer handles the common setup to create a test server and clients.
func newTestServer(t *testing.T) (*testServer, context.Context) {
	t.Helper()

	// Create a context with a large enough timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	// Load certs for the server and a couple of clients.
	serverTLSConfig := loadTLSConfig(t, "server")
	aliceTLSConfig := loadTLSConfig(t, "client-alice")
	bobTLSConfig := loadTLSConfig(t, "client-bob")

	// Create a server.
	s := apiserver.NewServer()
	grpcServer := grpc.NewServer(
		grpc.Creds(credentials.NewTLS(serverTLSConfig)),
		grpc.UnaryInterceptor(s.UnaryMiddleware),
		grpc.StreamInterceptor(s.StreamMiddleware),
	)
	pb.RegisterTelePilotServiceServer(grpcServer, s)

	// Listen on a random port.
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	noError(t, err, "Listen")
	t.Cleanup(func() { _ = lis.Close() }) // No strictly needed, but just to make sure. Called by the server Close().

	// Start the server.
	doneCh := make(chan struct{})
	go func() { defer close(doneCh); noError(t, grpcServer.Serve(lis), "Serve") }()
	t.Cleanup(func() { grpcServer.GracefulStop(); <-doneCh }) // Wait for the server to be fully closed.

	// Create clients.
	aliceClient, err := apiclient.NewClient(aliceTLSConfig, lis.Addr().String())
	noError(t, err, "NewClient for Alice")
	t.Cleanup(func() { noError(t, aliceClient.Close(), "Closing Alice's client.") })

	bobClient, err := apiclient.NewClient(bobTLSConfig, lis.Addr().String())
	noError(t, err, "NewClient for Bob")
	t.Cleanup(func() { noError(t, bobClient.Close(), "Closing Bob's client.") })

	return &testServer{
		grpcServer: grpcServer,
		alice:      aliceClient,
		bob:        bobClient,
	}, ctx
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

// consumeFromPipe creates a pipe and a channel, then start a goroutine to consume the pipe and forwards to the channel.
// Reads `len(sizes)` times and makes the channel buffered with `len(sizes)` size.
// Waits for the goroutine to be complete on cleanup, i.e. will block if one of the `len(sizes)` reads blocks.
func consumeFromPipe(ctx context.Context, t *testing.T, sizes ...int) (<-chan []byte, io.Writer) {
	t.Helper()

	// Create the pipe.
	r, w := io.Pipe()
	t.Cleanup(func() { _, _ = r.Close(), w.Close() }) // Make sure to cleanup.

	// Sanity check, make sure than when all done, the goroutine is gone.
	done := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-done:
		case <-ctx.Done():
			t.Fatal("Timeout waiting for read loop to end.")
		}
	})

	// Data channel, will send what we read via this.
	ch := make(chan []byte, len(sizes))
	go func() {
		defer close(done) // Signal when we are done.

		// Main read loop.
		for _, s := range sizes {
			buf := make([]byte, s)        // Making inside the loop to avoid override.
			n, err := io.ReadFull(r, buf) // TODO: Consider closing `w` on timeout to fail faster.
			noError(t, err, "read pipe")
			ch <- buf[:n]
		}
		close(ch)
	}()

	return ch, w
}
