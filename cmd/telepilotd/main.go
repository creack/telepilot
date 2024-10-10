// Package main implements a server for TelePilot service.
package main

import (
	"context"
	"flag"
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
	"go.creack.net/telepilot/pkg/cgroups"
	"go.creack.net/telepilot/pkg/initd"
	"go.creack.net/telepilot/pkg/tlsconfig"
)

func main() {
	keyDir := flag.String("certs", "./certs",
		"Certs directory. Expecting <certdir>/ca.pem, <certdir>/server.pem and <certdir>/server-key.pem.")
	isInit := flag.Bool("init", false, "internal flag to toggle init mode")
	flag.Parse()

	if *isInit {
		if err := initd.Init(flag.Args()); err != nil {
			slog.Error("Init error.", "error", err, "args", flag.Args())
			os.Exit(1)
		}
		return
	}

	server(*keyDir)
}

func server(keyDir string) {
	tlsConfig, err := tlsconfig.LoadTLSConfig(
		path.Join(keyDir, "server.pem"),
		path.Join(keyDir, "server-key.pem"),
		path.Join(keyDir, "ca.pem"),
		false,
	)
	if err != nil {
		slog.Error("Failed to load tls config.", "cert_dir", keyDir, "error", err)
		os.Exit(1)
	}

	if err := cgroups.InitialSetup(); err != nil {
		slog.Error("Failed to init cgroups.", "error", err)
		os.Exit(1)
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
		// TODO: Consider making the addr a flag.
		lis, err := net.Listen("tcp", "localhost:9090")
		if err != nil {
			slog.Error("Listen error", "error", err)
			os.Exit(1)
		}
		// NOTE: s.Serve takes ownership of lis. GracefulStop in s.Close() will invoke lis.Close().

		slog.Info("Server listening.", "addr", lis.Addr().String())

		if err := grpcServer.Serve(lis); err != nil {
			slog.Error("Serve error", "error", err)
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
