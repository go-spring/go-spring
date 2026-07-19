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
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/starter"
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

// Config defines Thrift server configuration.
//
// Protocol selects the on-the-wire message encoding and must match the
// client (binary/compact/json). Transport selects an optional transport
// wrapper: "none" keeps the raw socket (the historical default), while
// "framed" prepends a length prefix to each message — required by many
// cross-language clients. Both settings must be paired with a matching
// client; a mismatch corrupts the wire protocol.
type Config struct {
	Addr          string            `value:"${addr:=:9292}"`
	ClientTimeout time.Duration     `value:"${clientTimeout:=0}"`
	Protocol      string            `value:"${protocol:=binary}"`
	Transport     string            `value:"${transport:=none}"`
	BufferSize    int               `value:"${bufferSize:=4096}"`
	TLS           starter.TLSConfig `value:"${tls}"`
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
	s.svr = thrift.NewTSimpleServer4(s.proc, transport, transFactory, protoFactory)
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
