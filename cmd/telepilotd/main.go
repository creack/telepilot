// Package main implements a server for TelePilot service.
package main

import (
	"context"
	"flag"
	"log" // TODO: Consider using slog.
	"net"
	"os/signal"
	"path"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "go.creack.net/telepilot/api/v1"
	"go.creack.net/telepilot/pkg/apiserver"
	"go.creack.net/telepilot/pkg/tlsconfig"
)

func main() {
	keyDir := flag.String("certs", "./certs",
		"Certs directory. Expecting <certdir>/ca.pem, <certdir>/server.pem and <certdir>/server-key.pem.")
	tlsConfig, err := tlsconfig.LoadTLSConfig(
		path.Join(*keyDir, "server.pem"),
		path.Join(*keyDir, "server-key.pem"),
		path.Join(*keyDir, "ca.pem"),
		false,
	)
	if err != nil {
		log.Fatalf("Failed to load tls config from %q: %s.", *keyDir, err)
	}

	s := apiserver.NewServer()

	grpcServer := grpc.NewServer(
		grpc.Creds(credentials.NewTLS(tlsConfig)),
		grpc.UnaryInterceptor(s.UnaryMiddleware),
		grpc.StreamInterceptor(s.StreamMiddleware),
	)
	pb.RegisterTelePilotServiceServer(grpcServer, s)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)

		// TODO: Consider making the addr a flag.
		lis, err := net.Listen("tcp", "localhost:9090")
		if err != nil {
			log.Fatalf("Listen error: %s.", err)
		}
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Serve error: %s", err)
		}
	}()

	<-ctx.Done()
	log.Println("Bye.")
	grpcServer.GracefulStop()
	// TODO: Consider adding a timeout.
	<-doneCh
}
