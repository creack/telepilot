package apiserver

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"go.creack.net/telepilot/pkg/jobmanager"
)

// Common method to extract the user's CN from context.
func getUserFromContext(ctx context.Context) (string, error) {
	var user string

	if p, ok := peer.FromContext(ctx); ok {
		if mtls, ok := p.AuthInfo.(credentials.TLSInfo); ok {
			// NOTE: We control user management and their certificate, we only expect one.
			if len(mtls.State.PeerCertificates) > 1 {
				return "", fmt.Errorf("too many peers in cert: %w", ErrInvalidClientCerts)
			}
			for _, item := range mtls.State.PeerCertificates {
				user = item.Subject.CommonName
				break
			}
		}
	}
	if user == "" {
		return "", fmt.Errorf("CN not found: %w", ErrInvalidClientCerts)
	}
	return user, nil
}

// middleware to enforce authorization policies.
func (s *Server) authMiddleware(user, fullMethod string, req any) error {
	var j *jobmanager.Job
	if getter, ok := req.(interface{ GetJobId() string }); ok {
		jobID, err := uuid.Parse(getter.GetJobId())
		if err != nil {
			return fmt.Errorf("invalid uuid %q: %w", getter.GetJobId(), err)
		}
		job, err := s.jobmanager.LookupJob(jobID)
		if err != nil {
			// NOTE: The only possibel error at the moment is 'not found'. Return PermissionDenied
			// to avoid 'leaking' job info to unauthorized users.
			return status.Error(codes.PermissionDenied, "forbidden") //nolint:wrapcheck // Expected direct return.
		}
		j = job
	}
	if !enforcePolicies(user, j, policies[fullMethod]...) {
		return status.Error(codes.PermissionDenied, "forbidden") //nolint:wrapcheck // Expected direct return.
	}
	return nil
}

// UnaryMiddleware handles authn/authz from mtls for unary endpoints.
func (s *Server) UnaryMiddleware(
	ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
) (any, error) {
	// Authentication.
	user, err := getUserFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("getUserFromContext: %w", err)
	}
	// Authorization.
	if err := s.authMiddleware(user, info.FullMethod, req); err != nil {
		return nil, err
	}
	return handler(ctx, req)
}

// serverStreamWrapper wraps the RecvMsg to enforce authorization upon each streamed request.
type serverStreamWrapper struct {
	grpc.ServerStream

	s          *Server
	user       string
	fullMethod string
}

func (w *serverStreamWrapper) RecvMsg(m any) error {
	if err := w.ServerStream.RecvMsg(m); err != nil {
		return err //nolint:wrapcheck // Expected direct return.
	}
	if err := w.s.authMiddleware(w.user, w.fullMethod, m); err != nil {
		return err
	}
	return nil
}

// StreamMiddleware handles the authn/authz from mtls for streaming endpoints.
func (s *Server) StreamMiddleware(
	server any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler,
) error {
	ctx := ss.Context()
	// Authentication, only once.
	user, err := getUserFromContext(ctx)
	if err != nil {
		return fmt.Errorf("getUserFromContext: %w", err)
	}
	// Authorization, on each message.
	return handler(srv, &serverStreamWrapper{ServerStream: ss, s: s, user: user, fullMethod: info.FullMethod})
}
