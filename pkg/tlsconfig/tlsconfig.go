// Package tlsconfig provides helper to load a robust TLS config.
package tlsconfig

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
)

// LoadTLSConfig loads the certs from file.
//
// NOTE: We currently use the same CA for clients/server, for production, should use distinct ones.
func LoadTLSConfig(certFile, keyFile, caFile string, isClient bool) (*tls.Config, error) {
	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate: %w", err)
	}

	ca, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("faild to read CA certificate: %w", err)
	}

	capool := x509.NewCertPool()
	if !capool.AppendCertsFromPEM(ca) {
		return nil, errors.New("unable to append the CA certificate to CA pool") //nolint: err113 // Acceptable.
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP521,
			tls.CurveP384,
			tls.CurveP256,
		},
		// NOTE: CipherSuites are ignored in TLS1.3.
		Certificates: []tls.Certificate{certificate},
	}

	if isClient {
		tlsConfig.RootCAs = capool
	} else {
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		tlsConfig.ClientCAs = capool
	}

	return tlsConfig, nil
}
