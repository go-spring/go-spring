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

package StarterThrift

import (
	"context"
	"crypto/tls"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	enableSimpleThriftServer := gs.OnProperty("spring.thrift.server.enabled").
		HavingValue("true").MatchIfMissing()
	gs.Module(enableSimpleThriftServer, func(r gs.BeanProvider, p flatten.Storage) error {

		// Register the Thrift server
		// when a processor is available.
		r.Provide(
			NewSimpleThriftServer,
			gs.IndexArg(0, gs.TagArg("${spring.thrift.server}")),
		).Export(gs.As[gs.Server]()).
			Condition(gs.OnBean[thrift.TProcessor]())
		return nil
	})
}

// TLSConfig enables a TLS server transport by pointing at a PEM certificate/key
// pair. When Enabled is false the server uses a plaintext socket transport.
type TLSConfig struct {
	Enabled  bool   `value:"${enabled:=false}"`
	CertFile string `value:"${certFile:=}"`
	KeyFile  string `value:"${keyFile:=}"`
}

// Config defines Thrift server configuration.
type Config struct {
	Addr          string        `value:"${addr:=:9292}"`
	ClientTimeout time.Duration `value:"${clientTimeout:=0}"`
	TLS           TLSConfig     `value:"${tls}"`
}

// SimpleThriftServer adapts a thrift.TSimpleServer to the Go-Spring server lifecycle.
type SimpleThriftServer struct {
	cfg  Config
	proc thrift.TProcessor
	svr  *thrift.TSimpleServer
}

// NewSimpleThriftServer creates a SimpleThriftServer from ${spring.thrift.server} configuration.
func NewSimpleThriftServer(cfg Config, proc thrift.TProcessor) *SimpleThriftServer {
	return &SimpleThriftServer{cfg: cfg, proc: proc}
}

// newTransport builds a server transport honoring the client timeout and,
// when enabled, TLS.
func (s *SimpleThriftServer) newTransport() (thrift.TServerTransport, error) {
	if s.cfg.TLS.Enabled {
		cert, err := tls.LoadX509KeyPair(s.cfg.TLS.CertFile, s.cfg.TLS.KeyFile)
		if err != nil {
			return nil, errutil.Explain(err, "failed to load TLS key pair")
		}
		cfg := &tls.Config{Certificates: []tls.Certificate{cert}}
		return thrift.NewTSSLServerSocketTimeout(s.cfg.Addr, cfg, s.cfg.ClientTimeout)
	}
	return thrift.NewTServerSocketTimeout(s.cfg.Addr, s.cfg.ClientTimeout)
}

// Run starts the Thrift server after Go-Spring signals readiness.
func (s *SimpleThriftServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	transport, err := s.newTransport()
	if err != nil {
		return errutil.Explain(err, "failed to listen on %s", s.cfg.Addr)
	}
	s.svr = thrift.NewTSimpleServer2(s.proc, transport)
	<-sig.TriggerAndWait()
	if err = s.svr.Serve(); err != nil {
		return errutil.Explain(err, "failed to serve on %s", s.cfg.Addr)
	}
	return nil
}

// Stop gracefully stops the underlying Thrift server, interrupting the accept
// loop and waiting for in-flight requests to drain.
func (s *SimpleThriftServer) Stop() error {
	return s.svr.Stop()
}
