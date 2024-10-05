package telepilot_test

import (
	"crypto/tls"
	"net"
	"os"
	"path"
	"testing"

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

type testServer struct {
	srv        *apiserver.Server
	alice, bob *apiclient.Client
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()

	// Load certs for the server and a couple of clients.
	serverTLSConfig := loadTLSConfig(t, "server")
	aliceTLSConfig := loadTLSConfig(t, "client-alice")
	bobTLSConfig := loadTLSConfig(t, "client-bob")

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

	return &testServer{
		srv:   srv,
		alice: aliceClient,
		bob:   bobClient,
	}
}
