package apiserver

import (
	"errors"
	"fmt"
	"log"
	"net"
	"path"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "go.creack.net/telepilot/api/v1"
	"go.creack.net/telepilot/pkg/jobmanager"
	"go.creack.net/telepilot/pkg/tlsconfig"
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

func (s *Server) Serve(certDir string) error {
	s.jobmanager = jobmanager.NewJobManager()
	tlsConfig, err := tlsconfig.LoadTLSConfig(
		path.Join(certDir, "server.pem"),
		path.Join(certDir, "server-key.pem"),
		path.Join(certDir, "ca.pem"),
		false,
	)
	if err != nil {
		return fmt.Errorf("load tls config from %q: %w", certDir, err)
	}

	grpcServer := grpc.NewServer(
		grpc.Creds(credentials.NewTLS(tlsConfig)),
		grpc.UnaryInterceptor(s.UnaryMiddleware),
		grpc.StreamInterceptor(s.StreamMiddleware),
	)
	pb.RegisterTelePilotServiceServer(grpcServer, s)
	s.grpcServer = grpcServer

	lis, err := net.Listen("tcp", "localhost:9090")
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	log.Printf("Server listening at %s.", lis.Addr())
	if err := grpcServer.Serve(lis); err != nil {
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
