package apiserver

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "go.creack.net/telepilot/api/v1"
	"go.creack.net/telepilot/pkg/jobmanager"
)

// Common errors.
var (
	ErrInvalidClientCerts = errors.New("invalid client certifiate")
)

// Server is used to implement api.TelePilotService.
type Server struct {
	pb.UnimplementedTelePilotServiceServer

	grpcServer *grpc.Server

	jobmanager jobmanager.JobManager
}

func NewServer(tlsConfig *tls.Config) *Server {
	s := &Server{
		jobmanager: jobmanager.NewJobManager(),
	}
	grpcServer := grpc.NewServer(
		grpc.Creds(credentials.NewTLS(tlsConfig)),
		grpc.UnaryInterceptor(s.UnaryMiddleware),
		grpc.StreamInterceptor(s.StreamMiddleware),
	)
	pb.RegisterTelePilotServiceServer(grpcServer, s)
	s.grpcServer = grpcServer
	return s
}

func (s *Server) ListenAndServe(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	// NOTE: s.Serve takes ownership of lis. GracefulStop in s.Close() will invoke lis.Close().

	log.Printf("Server listening at %s.", lis.Addr())
	return s.Serve(lis)
}

func (s *Server) Serve(lis net.Listener) error {
	if err := s.grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("grpc serve: %w", err)
	}
	return nil
}

func (s *Server) Close() error {
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
	return nil
}
