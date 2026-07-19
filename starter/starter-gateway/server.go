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

package StarterGateway

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"os"

	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

// TLSConfig enables transport-level TLS on the gateway's listen port. When
// Enabled is false the gateway serves plaintext. Setting CAFile turns on mutual
// TLS: clients must present a certificate signed by the given CA.
type TLSConfig struct {
	Enabled  bool   `value:"${enabled:=false}"`
	CertFile string `value:"${certFile:=}"`
	KeyFile  string `value:"${keyFile:=}"`
	CAFile   string `value:"${caFile:=}"` // client CA bundle; presence enables mTLS
}

// ServerConfig configures the gateway's own listen port. It is deliberately
// separate from the business web server so both can run in one process on
// distinct ports.
type ServerConfig struct {
	Addr string    `value:"${addr:=:9440}"`
	TLS  TLSConfig `value:"${tls}"`
}

// GatewayServer adapts the gateway to the Go-Spring server lifecycle. It listens
// early (so a port clash fails at startup) but only begins serving after the app
// signals readiness, and stops via graceful Shutdown so in-flight requests drain.
type GatewayServer struct {
	Cfg ServerConfig `value:"${spring.gateway.server}"`
	tbl *RouteTable
	svr *http.Server
}

func newGatewayServer(tbl *RouteTable) *GatewayServer {
	return &GatewayServer{tbl: tbl}
}

// ServeHTTP matches the request against the route table and delegates to the
// matched route's compiled handler chain, or replies 404 when nothing matches.
func (s *GatewayServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	route := s.tbl.Match(r)
	if route == nil {
		http.Error(w, "404 Not Found", http.StatusNotFound)
		return
	}
	route.handler.ServeHTTP(w, r)
}

// tlsConfig builds the *tls.Config from the bound TLS settings, failing fast if
// certificate files are missing or unreadable.
func (s *GatewayServer) tlsConfig() (*tls.Config, error) {
	t := s.Cfg.TLS
	if !t.Enabled {
		return nil, nil
	}
	cert, err := tls.LoadX509KeyPair(t.CertFile, t.KeyFile)
	if err != nil {
		return nil, errutil.Explain(err, "gateway: failed to load TLS key pair")
	}
	cfg := &tls.Config{Certificates: []tls.Certificate{cert}}
	if t.CAFile != "" {
		pem, err := os.ReadFile(t.CAFile)
		if err != nil {
			return nil, errutil.Explain(err, "gateway: failed to read client CA")
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, errutil.Explain(nil, "gateway: no certificates found in %s", t.CAFile)
		}
		cfg.ClientCAs = pool
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return cfg, nil
}

// Run listens immediately, then serves after readiness is signaled, aligning the
// gateway with the framework's graceful-drain orchestration.
func (s *GatewayServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	// Compile the route table now that FilterWrapper beans have been injected, so
	// a bad initial config (unknown filter, missing wrapper bean) fails startup.
	if err := s.tbl.warmup(); err != nil {
		return err
	}

	tlsCfg, err := s.tlsConfig()
	if err != nil {
		return err
	}
	s.svr = &http.Server{Handler: s}

	listener, err := net.Listen("tcp", s.Cfg.Addr)
	if err != nil {
		return errutil.Explain(err, "gateway: failed to listen on %s", s.Cfg.Addr)
	}
	if tlsCfg != nil {
		listener = tls.NewListener(listener, tlsCfg)
	}

	<-sig.TriggerAndWait()
	log.Infof(ctx, log.TagAppDef, "gateway: serving on %s", s.Cfg.Addr)
	if err = s.svr.Serve(listener); err != nil && err != http.ErrServerClosed {
		return errutil.Explain(err, "gateway: failed to serve on %s", s.Cfg.Addr)
	}
	return nil
}

// Stop gracefully shuts the server down, letting in-flight requests finish.
func (s *GatewayServer) Stop() error {
	return s.svr.Shutdown(context.Background())
}
