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
	"net"
	"net/http"

	"go-spring.org/cloud/security"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

// ServerConfig configures the gateway's own listen port. It is deliberately
// separate from the business web server so both can run in one process on
// distinct ports.
//
// The nested TLS block reuses the shared security.TLSConfig for its cert/key/CA
// fields; for the gateway CAFile means "PEM bundle of client CAs" (presence
// enables mTLS), which is the server-side counterpart of the shared struct's
// generic "verify the peer" role.
type ServerConfig struct {
	Addr string             `value:"${addr}"`
	TLS  security.TLSConfig `value:"${tls}"`
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
	log.Debugf(context.Background(), log.TagAppDef, "gateway server created")
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

// tlsConfig builds the server-side *tls.Config from the bound TLS settings via
// security.BuildServer, failing fast if certificate files are missing or
// unreadable. A configured CAFile means "bundle of client CAs" and turns on
// mTLS (RequireAndVerifyClientCert).
func (s *GatewayServer) tlsConfig() (*tls.Config, error) {
	return s.Cfg.TLS.BuildServer()
}

// Run listens immediately, then serves after readiness is signaled, aligning the
// gateway with the framework's graceful-drain orchestration.
func (s *GatewayServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	// The listening address rides on the context: every line this server logs
	// about its own lifecycle carries it without repeating it.
	ctx = log.WithFields(ctx, log.String("addr", s.Cfg.Addr))

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
	log.Info(ctx, log.TagAppDef, log.Msg("gateway: serving"))
	if err = s.svr.Serve(listener); err != nil && !errutil.IsServerClosed(err) {
		log.Error(ctx, log.TagAppDef,
			log.Err(err),
			log.Msg("gateway: failed to serve"))
		return errutil.Explain(err, "gateway: failed to serve on %s", s.Cfg.Addr)
	}
	return nil
}

// Stop gracefully shuts the server down, letting in-flight requests
// finish; ctx is propagated into http.Server.Shutdown so the drain rides the
// shutdown context.
func (s *GatewayServer) Stop(ctx context.Context) error {
	log.Info(ctx, log.TagAppDef,
		log.String("addr", s.Cfg.Addr),
		log.Msg("gateway: shutting down"))
	return s.svr.Shutdown(ctx)
}
