package telepilot_test

import (
	"context"
	"crypto/x509"
	"errors"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	pb "go.creack.net/telepilot/api/v1"
	"go.creack.net/telepilot/pkg/apiclient"
	"go.creack.net/telepilot/pkg/apiserver"
)

func TestUnauthenticatedUser(t *testing.T) {
	t.Parallel()

	// Load certs for the server and a couple of clients.
	serverTLSConfig := loadTLSConfig(t, "server")
	// TODO: Use certs from an unknown CA, to make the test is more realistic.
	aliceTLSConfig := loadTLSConfig(t, "client-alice")
	bobTLSConfig := loadTLSConfig(t, "client-bob")

	// Make the VerifyPeerCertificate always fail.
	serverTLSConfig.VerifyPeerCertificate = func([][]byte, [][]*x509.Certificate) error {
		return errors.New("fail") //nolint:err113 // No need for custom error here.
	}

	// Create a server.
	s, err := apiserver.NewServer()
	noError(t, err, "New API Server.")
	grpcServer := grpc.NewServer(grpc.Creds(credentials.NewTLS(serverTLSConfig)),
		grpc.UnaryInterceptor(s.UnaryMiddleware), grpc.StreamInterceptor(s.StreamMiddleware))
	pb.RegisterTelePilotServiceServer(grpcServer, s)

	// Listen on a random port.
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	noError(t, err, "Listen")
	t.Cleanup(func() { _ = lis.Close() }) // No strictly needed, but just to make sure.

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
