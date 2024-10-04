// Package main implements a server for TelePilot service.
package main

import (
	"context"
	"log"
	"net"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"

	pb "go.creack.net/telepilot/api/v1"
)

// server is used to implement api.TelePilot.
type server struct {
	pb.UnimplementedTelePilotServiceServer
}

// HandlerMiddleware handles the authn from mtls for unary endpoints.
func HandlerMiddleware(
	ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
) (any, error) {
	// You can write your own code here to check client tls certificate.
	if p, ok := peer.FromContext(ctx); ok {
		if mtls, ok := p.AuthInfo.(credentials.TLSInfo); ok {
			for _, item := range mtls.State.PeerCertificates {
				log.Println("client certificate subject:", item.Subject)
			}
		}
	}
	return handler(ctx, req)
}

// StreamingMiddleware handles the authn from mtls for streaming endpoints.
func StreamingMiddleware(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	ctx := ss.Context()
	// You can write your own code here to check client tls certificate.
	if p, ok := peer.FromContext(ctx); ok {
		if mtls, ok := p.AuthInfo.(credentials.TLSInfo); ok {
			for _, item := range mtls.State.PeerCertificates {
				log.Println("client certificate subject:", item.Subject)
			}
		}
	}

	return handler(srv, ss)
}

func main() {
	s := grpc.NewServer(
		grpc.UnaryInterceptor(HandlerMiddleware),
		grpc.StreamInterceptor(StreamingMiddleware),
	)
	pb.RegisterTelePilotServiceServer(s, &server{})

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go func() {
		lis, err := net.Listen("tcp", "localhost:9090")
		if err != nil {
			log.Fatalf("Failed to listen: %s.", err)
		}

		log.Printf("Server listening at %s.", lis.Addr())
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %s.", err)
		}
	}()

	<-ctx.Done()
	log.Println("Bye.")
	s.GracefulStop()
}
