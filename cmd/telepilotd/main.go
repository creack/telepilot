// Package main implements a server for TelePilot service.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"path"
	"syscall"

	"go.creack.net/telepilot/pkg/apiserver"
	"go.creack.net/telepilot/pkg/tlsconfig"
)

func main() {
	keyDir := flag.String("certs", "./certs",
		"Certs directory. Expecting <certdir>/ca.pem, <certdir>/server.pem and <certdir>/server-key.pem.")
	flag.Parse()

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

	s, err := apiserver.NewServer(tlsConfig)
	if err != nil {
		slog.Error("Failed to create api server.", slog.Any("error", err))
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		// TODO: Consider making the addr a flag.
		if err := s.ListenAndServe("localhost:9090"); err != nil {
			slog.Error(err.Error())
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("Bye.")
	_ = s.Close() // Best effort.
	// TODO: Consider adding a timeout.
	<-doneCh
}
