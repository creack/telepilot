// Package main implements a server for TelePilot service.
package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"

	"go.creack.net/telepilot/pkg/apiserver"
)

func main() {
	keyDir := flag.String("certs", "./certs",
		"Certs directory. Expecting <certdir>/ca.pem, <certdir>/server.pem and <certdir>/server-key.pem.")

	s := &apiserver.Server{}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go func() {
		if err := s.Serve(*keyDir); err != nil {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	log.Println("Bye.")
	_ = s.Close() // Best effort.
}
