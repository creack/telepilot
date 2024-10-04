// Package main implements a server for TelePilot service.
package main

import (
	"context"
	"flag"
	"log" // TODO: Consider using slog.
	"os/signal"
	"path"
	"syscall"

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

	s := apiserver.NewServer(tlsConfig)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		if err := s.ListenAndServe("localhost:9090"); err != nil {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	log.Println("Bye.")
	_ = s.Close() // Best effort.
	// TODO: Consider adding a timeout.
	<-doneCh
}
