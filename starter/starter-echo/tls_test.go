/*
 * Copyright 2025 The Go-Spring Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package StarterEcho

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"go-spring.org/cloud/tlsconf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/testing/assert"
)

// firedSignal is a gs.ReadySignal that fires immediately so Run does not block
// waiting for the container's readiness barrier.
type firedSignal struct{}

func (firedSignal) TriggerAndWait() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

var _ gs.ReadySignal = firedSignal{}

// TestEchoServer_MTLS verifies the server-side TLS wiring: ca-file enables
// mTLS (RequireAndVerifyClientCert), so a plain TLS client is rejected while a
// client presenting the trusted certificate completes the handshake and gets a
// 200. This pins the alignment with starter-grpc's BuildServer semantics.
func TestEchoServer_MTLS(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := writeTLSPair(t, dir)

	ln0, err := net.Listen("tcp", "127.0.0.1:0")
	assert.That(t, err).Nil()
	addr := ln0.Addr().String()
	_ = ln0.Close()

	cfg := Config{
		Address: addr,
		TLS: tlsconf.TLSConfig{
			Enabled:  true,
			CertFile: certPath,
			KeyFile:  keyPath,
			CAFile:   certPath, // the self-signed cert doubles as its own CA
		},
		Middleware: MiddlewareConfig{},
	}
	svr, err := NewSimpleEchoServer(func(e *echo.Echo) {
		e.GET("/ping", func(c echo.Context) error { return c.String(http.StatusOK, "pong") })
	}, nil, cfg)
	assert.That(t, err).Nil()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- svr.Run(ctx, firedSignal{}) }()
	stop := func() { _ = svr.Stop(context.Background()) }
	defer stop()

	caPEM, err := os.ReadFile(certPath)
	assert.That(t, err).Nil()
	pool := x509.NewCertPool()
	assert.That(t, pool.AppendCertsFromPEM(caPEM)).True()
	pair, err := tls.LoadX509KeyPair(certPath, keyPath)
	assert.That(t, err).Nil()

	// Wait for the listener to come up so a "connection refused" is never
	// mistaken for the handshake failure asserted below.
	for i := 0; i < 100; i++ {
		c, derr := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if derr == nil {
			_ = c.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Without a client certificate the mTLS handshake fails.
	noCert := &http.Client{
		Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, ServerName: "127.0.0.1"}},
		Timeout:   5 * time.Second,
	}
	_, err = noCert.Get("https://" + addr + "/ping")
	assert.That(t, err).NotNil()

	// With the trusted client certificate the request succeeds.
	withCert := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:      pool,
				ServerName:   "127.0.0.1",
				Certificates: []tls.Certificate{pair},
			},
		},
		Timeout: 5 * time.Second,
	}
	resp, err := withCert.Get("https://" + addr + "/ping")
	assert.That(t, err).Nil()
	defer resp.Body.Close()
	assert.That(t, resp.StatusCode).Equal(http.StatusOK)

	cancel()
	stop()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop")
	}
}

// writeTLSPair writes a self-signed RSA cert/key pair for TLS tests.
func writeTLSPair(t *testing.T, dir string) (certPath, keyPath string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.That(t, err).Nil()
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "127.0.0.1"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"127.0.0.1", "localhost"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	assert.That(t, err).Nil()

	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")
	assert.That(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600)).Nil()
	assert.That(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0o600)).Nil()
	return certPath, keyPath
}
