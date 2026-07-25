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
	"fmt"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"go-spring.org/log"
	"go-spring.org/spring/cloud/tlsconf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

var thriftTag = log.RegisterAppTag("thrift", "starter")

func init() {
	gs.Provide(
		NewSimpleThriftServer,
		gs.IndexArg(0, gs.TagArg("${spring.thrift.server}")),
	).Export(gs.As[gs.Server]()).
		Condition(gs.OnProperty("spring.thrift.server.addr"))
}

// Config defines Thrift server configuration.
//
// Protocol selects the on-the-wire message encoding and must match the
// client (binary/compact/json). Transport selects an optional transport
// wrapper: "none" keeps the raw socket (the historical default), while
// "framed" prepends a length prefix to each message — required by many
// cross-language clients. Both settings must be paired with a matching
// client; a mismatch corrupts the wire protocol.
//
// Observer toggles OTel tracing and metrics via a wrapped TProcessor.
// Both are on by default; importing starter-otel activates them.
type Config struct {
	Addr          string            `value:"${addr}"`
	ClientTimeout time.Duration     `value:"${clientTimeout:=0}"`
	Protocol      string            `value:"${protocol:=binary}"`
	Transport     string            `value:"${transport:=none}"`
	BufferSize    int               `value:"${bufferSize:=4096}"`
	TLS           tlsconf.TLSConfig `value:"${tls}"`
	Observer      ObserverConfig    `value:"${observer}"`
}

// ObserverConfig groups the built-in observability options the starter can
// apply to the thrift.TProcessor.
type ObserverConfig struct {
	Tracing TracingConfig `value:"${tracing}"`
	Metrics MetricsConfig `value:"${metrics}"`
}

// TracingConfig toggles wrapping the TProcessor with an OTel tracing wrapper.
// On by default.
type TracingConfig struct {
	Enabled bool `value:"${enabled:=true}"`
}

// MetricsConfig toggles wrapping the TProcessor with an OTel metrics wrapper.
// On by default.
type MetricsConfig struct {
	Enabled bool `value:"${enabled:=true}"`
}

// SimpleThriftServer adapts a thrift.TSimpleServer to the Go-Spring server lifecycle.
type SimpleThriftServer struct {
	cfg  Config
	proc thrift.TProcessor
	svr  *thrift.TSimpleServer
}

// NewSimpleThriftServer creates a SimpleThriftServer from ${spring.thrift.server} configuration.
func NewSimpleThriftServer(cfg Config, proc thrift.TProcessor) *SimpleThriftServer {
	log.Debugf(context.Background(), thriftTag, "thrift server created addr=%s protocol=%s transport=%s",
		cfg.Addr, cfg.Protocol, cfg.Transport)
	return &SimpleThriftServer{cfg: cfg, proc: proc}
}

// newTransport builds a server transport honoring the client timeout and,
// when enabled, TLS.
func (s *SimpleThriftServer) newTransport() (thrift.TServerTransport, error) {
	if s.cfg.TLS.Enabled {
		tlsCfg, err := s.cfg.TLS.Build()
		if err != nil {
			return nil, errutil.Explain(err, "thrift: build TLS")
		}
		return thrift.NewTSSLServerSocketTimeout(s.cfg.Addr, tlsCfg, s.cfg.ClientTimeout)
	}
	return thrift.NewTServerSocketTimeout(s.cfg.Addr, s.cfg.ClientTimeout)
}

// protocolFactory maps the configured protocol name to a thrift
// TProtocolFactory. The server and client must agree on the protocol.
func (s *SimpleThriftServer) protocolFactory() (thrift.TProtocolFactory, error) {
	switch s.cfg.Protocol {
	case "", "binary":
		return thrift.NewTBinaryProtocolFactoryConf(nil), nil
	case "compact":
		return thrift.NewTCompactProtocolFactoryConf(nil), nil
	case "json":
		return thrift.NewTJSONProtocolFactory(), nil
	default:
		return nil, fmt.Errorf("unknown thrift protocol %q (want binary/compact/json)", s.cfg.Protocol)
	}
}

// transportFactory maps the configured transport name to a thrift
// TTransportFactory. "none" keeps the raw socket (identity factory) to
// preserve backwards compatibility; the server and client must agree.
func (s *SimpleThriftServer) transportFactory() (thrift.TTransportFactory, error) {
	switch s.cfg.Transport {
	case "", "none":
		return thrift.NewTTransportFactory(), nil
	case "buffered":
		return thrift.NewTBufferedTransportFactory(s.cfg.BufferSize), nil
	case "framed":
		conf := &thrift.TConfiguration{MaxFrameSize: int32(s.cfg.BufferSize)}
		return thrift.NewTFramedTransportFactoryConf(thrift.NewTTransportFactory(), conf), nil
	default:
		return nil, fmt.Errorf("unknown thrift transport %q (want none/buffered/framed)", s.cfg.Transport)
	}
}

// Run starts the Thrift server after Go-Spring signals readiness.
func (s *SimpleThriftServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	transport, err := s.newTransport()
	if err != nil {
		return errutil.Explain(err, "failed to listen on %s", s.cfg.Addr)
	}
	protoFactory, err := s.protocolFactory()
	if err != nil {
		return err
	}
	transFactory, err := s.transportFactory()
	if err != nil {
		return err
	}
	proc := s.proc
	if s.cfg.Observer.Tracing.Enabled || s.cfg.Observer.Metrics.Enabled {
		proc = WrapProcessor(proc)
	}
	s.svr = thrift.NewTSimpleServer4(proc, transport, transFactory, protoFactory)
	<-sig.TriggerAndWait()
	log.Infof(ctx, thriftTag, "thrift server starting on %s", s.cfg.Addr)
	if err = s.svr.Serve(); err != nil {
		log.Errorf(ctx, thriftTag, "thrift server failed on %s: %v", s.cfg.Addr, err)
		return errutil.Explain(err, "failed to serve on %s", s.cfg.Addr)
	}
	return nil
}

// Stop gracefully stops the underlying Thrift server, interrupting the accept
// loop and waiting for in-flight requests to drain.
func (s *SimpleThriftServer) Stop() error {
	log.Infof(context.Background(), thriftTag, "thrift server shutting down on %s", s.cfg.Addr)
	return s.svr.Stop()
}
