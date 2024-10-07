package telepilot_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"os"
	"path"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.creack.net/telepilot/pkg/apiclient"
	"go.creack.net/telepilot/pkg/apiserver"
	"go.creack.net/telepilot/pkg/tlsconfig"
)

func noError(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %s.", msg, err)
	}
}

func assert[T comparable](t *testing.T, expect, got T, msg string) {
	t.Helper()
	if expect != got {
		t.Fatalf("Assert fail: %s:\n Expect:\t%v\n Got:\t%v", msg, expect, got)
	}
}

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

func TestStartStop(t *testing.T) {
	t.Parallel()

	// Load certs for the server and a couple of clients.
	serverTLSConfig := loadTLSConfig(t, "server")
	aliceTLSConfig := loadTLSConfig(t, "client-alice")
	bobTLSConfig := loadTLSConfig(t, "client-bob")

	// Create a server.
	srv := apiserver.NewServer(serverTLSConfig)

	// Listen on a random port.
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	noError(t, err, "Listen")
	t.Cleanup(func() { _ = lis.Close() }) // No strictly needed, but just to make sure. Called by the server Close().

	// Start the server.
	doneCh := make(chan struct{})
	go func() { defer close(doneCh); noError(t, srv.Serve(lis), "Serve") }()
	t.Cleanup(func() { noError(t, srv.Close(), "Closing server."); <-doneCh }) // Wait for the server to be fully closed.

	// Create clients.
	aliceClient, err := apiclient.NewClient(aliceTLSConfig, lis.Addr().String())
	noError(t, err, "NewClient for Alice")
	t.Cleanup(func() { noError(t, aliceClient.Close(), "Closing Alice's client.") })

	bobClient, err := apiclient.NewClient(bobTLSConfig, lis.Addr().String())
	noError(t, err, "NewClient for Bob")
	t.Cleanup(func() { noError(t, bobClient.Close(), "Closing Bob's client.") })

	ctx := context.Background()

	// Create a Job.
	jobID, err := aliceClient.StartJob(ctx, "foo", nil)
	noError(t, err, "Alice start job.")

	// Stop the Job from the same user.
	t.Run("happy alice", func(t *testing.T) {
		t.Parallel()
		st, ok := status.FromError(aliceClient.StopJob(ctx, jobID))
		assert(t, true, ok, "extract grpc status from error")

		// TODO: Change this to noError once implemented.
		assert(t, codes.Unimplemented, st.Code(), "invalid grpc status code")
	})

	// Attempt to stop the job from a different user.
	t.Run("sad bob", func(t *testing.T) {
		t.Parallel()

		st, ok := status.FromError(bobClient.StopJob(ctx, jobID))
		assert(t, true, ok, "extract grpc status from error")

		assert(t, codes.PermissionDenied, st.Code(), "invalid grpc status code")

		// Attempt to stop a non-exiting job.
		st, ok = status.FromError(bobClient.StopJob(ctx, uuid.New().String()))
		assert(t, true, ok, "extract grpc status from error")

		assert(t, codes.PermissionDenied, st.Code(), "invalid grpc status code")
	})
}

func TestUnauthenticatedUser(t *testing.T) {
	t.Parallel()

	// Load certs for the server and a couple of clients.
	serverTLSConfig := loadTLSConfig(t, "server")
	aliceTLSConfig := loadTLSConfig(t, "client-alice")
	bobTLSConfig := loadTLSConfig(t, "client-bob")

	// Make the VerifyPeerCertificate always fail.
	serverTLSConfig.VerifyPeerCertificate = func([][]byte, [][]*x509.Certificate) error {
		return errors.New("fail") //nolint:err113 // No need for custom error here.
	}

	// Create a server.
	srv := apiserver.NewServer(serverTLSConfig)

	// Listen on a random port.
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	noError(t, err, "Listen")
	t.Cleanup(func() { _ = lis.Close() }) // No strictly needed, but just to make sure.

	// Start the server.
	doneCh := make(chan struct{})
	go func() { defer close(doneCh); noError(t, srv.Serve(lis), "Serve") }()
	t.Cleanup(func() { noError(t, srv.Close(), "Closing server."); <-doneCh }) // Wait for the server to be fully closed.

	// Create clients.
	aliceClient, err := apiclient.NewClient(aliceTLSConfig, lis.Addr().String())
	noError(t, err, "NewClient for Alice")
	t.Cleanup(func() { noError(t, aliceClient.Close(), "Closing Alice's client.") })

	bobClient, err := apiclient.NewClient(bobTLSConfig, lis.Addr().String())
	noError(t, err, "NewClient for Bob")
	t.Cleanup(func() { noError(t, bobClient.Close(), "Closing Bob's client.") })

	ctx := context.Background()

	// Attempt to Start a Job.
	t.Run("sad alice", func(t *testing.T) {
		t.Parallel()
		_, err := aliceClient.StartJob(ctx, "true", nil)
		st, ok := status.FromError(err)
		assert(t, true, ok, "extract grpc status from error")

		assert(t, codes.Unavailable, st.Code(), "invalid grpc status code")
	})

	// Attempt to Start a Job.
	t.Run("sad bob", func(t *testing.T) {
		t.Parallel()
		_, err := bobClient.StartJob(ctx, "true", nil)
		st, ok := status.FromError(err)
		assert(t, true, ok, "extract grpc status from error")

		assert(t, codes.Unavailable, st.Code(), "invalid grpc status code")
	})
}
