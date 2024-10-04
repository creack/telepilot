package test

import (
	"context"
	"net"
	"testing"

	"go.creack.net/telepilot/pkg/apiclient"
	"go.creack.net/telepilot/pkg/apiserver"
	"go.creack.net/telepilot/pkg/tlsconfig"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func noError(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %s.", msg, err)
	}
}

func assert[T comparable](t *testing.T, expect, got T, msg string) {
	if expect != got {
		t.Fatalf("Assert fail: %s:\n Expect:\t%v\n Got:\t%v", msg, expect, got)
	}
}

func TestHappyPath(t *testing.T) {
	t.Parallel()

	aliceTLSConfig, err := tlsconfig.LoadTLSConfig("../certs/client-alice.pem", "../certs/client-alice-key.pem", "../certs/ca.pem", true)
	noError(t, err, "Load alice certs")

	bobTLSConfig, err := tlsconfig.LoadTLSConfig("../certs/client-bob.pem", "../certs/client-bob-key.pem", "../certs/ca.pem", true)
	noError(t, err, "Load bob certs")

	serverTLSConfig, err := tlsconfig.LoadTLSConfig("../certs/server.pem", "../certs/server-key.pem", "../certs/ca.pem", false)
	noError(t, err, "Load server certs")

	srv := apiserver.NewServer(serverTLSConfig)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	noError(t, err, "Listen")

	doneCh := make(chan struct{})
	go func() { defer close(doneCh); noError(t, srv.Serve(lis), "Serve") }()
	t.Cleanup(func() { noError(t, srv.Close(), "Closing server."); <-doneCh })

	aliceClient, err := apiclient.NewClient(aliceTLSConfig, lis.Addr().String())
	noError(t, err, "NewClient for Alice")
	t.Cleanup(func() { noError(t, aliceClient.Close(), "Closing Alice's client.") })

	bobClient, err := apiclient.NewClient(bobTLSConfig, lis.Addr().String())
	noError(t, err, "NewClient for Bob")
	t.Cleanup(func() { noError(t, bobClient.Close(), "Closing Bob's client.") })

	ctx := context.Background()

	jobID, err := aliceClient.StartJob(ctx, "foo", nil)
	noError(t, err, "Alice start job.")

	t.Run("happy alice", func(t *testing.T) {
		t.Parallel()
		st, ok := status.FromError(aliceClient.StopJob(ctx, jobID))
		assert(t, true, ok, "extract grpc status from error")

		// TODO: Change this to noError once implemented.
		assert(t, codes.Unimplemented, st.Code(), "invalid grpc status code")
	})

	t.Run("sad bob", func(t *testing.T) {
		t.Parallel()
		st, ok := status.FromError(bobClient.StopJob(ctx, jobID))
		assert(t, true, ok, "extract grpc status from error")

		// TODO: Change this to noError once implemented.
		assert(t, codes.PermissionDenied, st.Code(), "invalid grpc status code")
	})
}
