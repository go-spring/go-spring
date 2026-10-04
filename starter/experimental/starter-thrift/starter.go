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
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"go-spring.org/cloud/governance"
	"go-spring.org/cloud/resilience"
	"go-spring.org/cloud/security"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

func init() {
	gs.Provide(
		NewSimpleThriftServer,
		gs.IndexArg(0, gs.TagArg("${spring.thrift.server}")),
		// The governance center is the family's sole injection point: it hands
		// out the resilience/fault/loadbalance authorities.
	).Export(gs.As[gs.Server]()).
		Condition(gs.And(
			gs.OnProperty("spring.thrift.server.enabled").HavingValue("true").MatchIfMissing(),
			gs.OnProperty("spring.thrift.server.addr"),
		))
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
	// Enabled gates the server (enabled, default true) — the starter convention:
	// the switch opts OUT, the addr key below opts IN.
	Enabled bool `value:"${enabled:=true}"`

	Addr          string             `value:"${addr}"`
	ClientTimeout time.Duration      `value:"${clientTimeout:=0}"`
	Protocol      string             `value:"${protocol:=binary}"`
	Transport     string             `value:"${transport:=none}"`
	BufferSize    int                `value:"${bufferSize:=4096}"`
	TLS           security.TLSConfig `value:"${tls}"`
	Observer      ObserverConfig     `value:"${observer}"`
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

	// center is the governance center bean gs injects (nil in a standalone
	// call); its resilience authority backs the inbound middleware Run installs.
	center *governance.Center
}

// NewSimpleThriftServer creates a SimpleThriftServer from ${spring.thrift.server}
// configuration. center is the injected governance center the inbound
// middleware is armed from.
func NewSimpleThriftServer(cfg Config, proc thrift.TProcessor, center *governance.Center) *SimpleThriftServer {
	log.Debug(context.Background(), log.TagAppDef, func() []log.Field {
		return []log.Field{
			log.String("addr", cfg.Addr),
			log.String("protocol", cfg.Protocol),
			log.String("transport", cfg.Transport),
			log.Msg("create thrift server success"),
		}
	})
	return &SimpleThriftServer{cfg: cfg, proc: proc, center: center}
}

// newTransport builds a server transport honoring the client timeout and,
// when enabled, TLS.
func (s *SimpleThriftServer) newTransport() (thrift.TServerTransport, error) {
	if s.cfg.TLS.Enabled {
		tlsCfg, err := s.cfg.TLS.BuildClient()
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
	case "header":
		// THeaderProtocol self-frames and carries per-message headers, which is
		// what makes W3C trace-context propagation possible (see middleware.go).
		// Pair it with transport "none" (it manages framing itself).
		return thrift.NewTHeaderProtocolFactoryConf(nil), nil
	default:
		return nil, errutil.Explain(nil, "unknown thrift protocol %q (want binary/compact/json/header)", s.cfg.Protocol)
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
		return nil, errutil.Explain(nil, "unknown thrift transport %q (want none/buffered/framed)", s.cfg.Transport)
	}
}

// Run starts the Thrift server after Go-Spring signals readiness.
func (s *SimpleThriftServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	// The listening address rides on the context: every line this server logs
	// about its own lifecycle carries it without repeating it.
	ctx = log.WithFields(ctx, log.String("addr", s.cfg.Addr))

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
	// All three middlewares ride the library's own per-method hook, so ORDER is
	// what makes the promises hold: Observe is listed first (outermost) so a
	// call Admit rejects is still traced and counted, and AccessLog sits inside
	// Observe (its line then carries the span's trace_id) but outside Admit (so
	// a rejection is logged too). A rejection short-circuits inside Admit and
	// returns back through the two outer middlewares.
	//
	// Observe is the only one behind a switch — it rides the OTel globals.
	// AccessLog is always installed (the RPC family requires every member to
	// have one), and so is inbound: each call passes the service's
	// rate-limit / bulkhead / breaker policy before reaching the service; with
	// governance off the executor is a transparent pass-through, so installing
	// it costs a call frame and changes nothing else.
	var mws []thrift.ProcessorMiddleware
	if s.cfg.Observer.Tracing.Enabled || s.cfg.Observer.Metrics.Enabled {
		mws = append(mws, Observe())
	}
	mws = append(mws, AccessLog())
	mws = append(mws, Admit(resilience.ServiceLabel("thrift", s.cfg.Addr), "thrift", s.center.Resilience()))
	proc := thrift.WrapProcessor(s.proc, mws...)
	s.svr = thrift.NewTSimpleServer4(proc, transport, transFactory, protoFactory)
	<-sig.TriggerAndWait()
	log.Info(ctx, log.TagAppDef, log.Msg("thrift server starting"))
	if err = s.svr.Serve(); err != nil {
		log.Error(ctx, log.TagAppDef, err, log.Msg("thrift server failed"))
		return errutil.Explain(err, "failed to serve on %s", s.cfg.Addr)
	}
	return nil
}

// Stop stops the underlying Thrift server. Thrift's Stop does NOT wait for
// in-flight connections — there is no graceful drain (declared boundary; use
// mature frameworks for production-grade thrift shutdown) — and takes no
// context, so ctx only tags the shutdown log.
func (s *SimpleThriftServer) Stop(ctx context.Context) error {
	log.Info(ctx, log.TagAppDef,
		log.String("addr", s.cfg.Addr),
		log.Msg("thrift server shutting down"))
	return s.svr.Stop()
}
