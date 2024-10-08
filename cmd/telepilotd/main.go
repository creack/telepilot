// Package main implements a server for TelePilot service.
package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"net"
	"os"
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
	init := flag.Bool("init", false, "internal flag to toggle init mode")
	flag.Parse()
	if *init {
		log.Printf("%q - %v\n", flag.Args()[0], flag.Args()[1:])
		return
	}

	tlsConfig, err := tlsconfig.LoadTLSConfig(
		path.Join(*keyDir, "server.pem"),
		path.Join(*keyDir, "server-key.pem"),
		path.Join(*keyDir, "ca.pem"),
		false,
	)
	if err != nil {
		slog.Error("Failed to load tls config.", slog.String("cert_dir", *keyDir), slog.Any("error", err))
		os.Exit(1)
	}

	s, err := apiserver.NewServer()
	if err != nil {
		slog.Error("Failed to create api server.", slog.Any("error", err))
		os.Exit(1)
	}

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
		// TODO: Consider making the addr a flag.
		lis, err := net.Listen("tcp", "localhost:9090")
		if err != nil {
			slog.Error("Listen error", slog.Any("error", err))
			os.Exit(1)
		}
		// NOTE: s.Serve takes ownership of lis. GracefulStop in s.Close() will invoke lis.Close().

		slog.With("addr", lis.Addr().String()).Info("Server listening.")

		if err := grpcServer.Serve(lis); err != nil {
			slog.Error("Serve error", slog.Any("error", err))
			os.Exit(1)
		}

		close(doneCh)
	}()

	<-ctx.Done()
	slog.Info("Bye.")
	grpcServer.GracefulStop()
	// TODO: Consider adding a timeout.
	<-doneCh
}
