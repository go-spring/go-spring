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

package StarterDubbo

import (
	"context"
	"time"

	"dubbo.apache.org/dubbo-go/v3/protocol"
	"dubbo.apache.org/dubbo-go/v3/server"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Server side: gated on both a ServiceRegister and the *Instance bean — no
	// service to expose (or no registries) means no server, so client-only apps
	// are never forced to stand one up. It reads ${spring.dubbo.server} and derives
	// from the shared *Instance (global registries + observability).
	enableSimpleDubboServer := gs.OnProperty("spring.dubbo.server.enabled").
		HavingValue("true").MatchIfMissing()
	gs.Module(enableSimpleDubboServer, func(r gs.BeanProvider, p flatten.Storage) error {
		r.Provide(
			NewSimpleDubboServer,
			gs.IndexArg(0, gs.TagArg("${spring.dubbo.server}")),
		).Export(gs.As[gs.Server]()).Condition(
			gs.OnBean[ServiceRegister](),
			gs.OnBean[*Instance](),
		)
		return nil
	})
}

// ServiceRegister registers services on a Dubbo server.Server. This function
// type keeps SimpleDubboServer service-agnostic: it drives the lifecycle while
// each service supplies its own register bean.
type ServiceRegister func(svr *server.Server) error

// ProtocolCfg configures a single Dubbo protocol listener. The map key under
// ${spring.dubbo.server.protocols} is the dubbo-go protocol name (e.g. "tri",
// "dubbo", "jsonrpc", "rest") and is passed straight through, so any dubbo-go
// protocol works without changing this starter.
type ProtocolCfg struct {
	Ip     string            `value:"${ip:=}"`     // listen address; empty means dubbo-go's default (bind-all)
	Port   int               `value:"${port}"`     // required: a registered service needs a known port
	Params map[string]string `value:"${params:=}"` // extra protocol params, escape hatch
}

// ServerConfig defines Dubbo server configuration: the map-driven protocols, the
// role-specific registry selection, and the common provider-level knobs. Every
// field is optional and empty/zero values are skipped so dubbo-go keeps its own
// default.
//
// Enum-like fields accept the dubbo-go names:
//   - Cluster:       failover(default)|failfast|failsafe|failback|forking|available|broadcast|zoneAware
//   - LoadBalance:   random(default)|roundrobin|leastactive|consistenthashing|p2c
//   - Serialization: hessian2|protobuf|msgpack|json
type ServerConfig struct {
	// Provider-wide defaults applied to every exported service.
	Group           string        `value:"${group:=}"`
	Version         string        `value:"${version:=}"`
	Cluster         string        `value:"${cluster:=}"`
	LoadBalance     string        `value:"${load-balance:=}"`
	Serialization   string        `value:"${serialization:=}"`
	Retries         int           `value:"${retries:=-1}"` // negative means unset; 0 explicitly disables retries
	Filter          string        `value:"${filter:=}"`    // comma-separated filter chain; use "-name" to drop one from dubbo-go's default chain
	Token           string        `value:"${token:=}"`
	AccessLog       string        `value:"${access-log:=}"`
	Auth            string        `value:"${auth:=}"`
	Tag             string        `value:"${tag:=}"`
	Warmup          time.Duration `value:"${warmup:=}"` // e.g. "10m"
	NotRegister     bool          `value:"${not-register:=false}"`
	AdaptiveService bool          `value:"${adaptive-service:=false}"`

	// Filter tuning params, all service-level (apply to every service this
	// server exposes). The relevant filter must be in the chain (all but
	// param-sign are in dubbo-go's default chain). Empty/negative means unset,
	// so dubbo-go keeps its own default (tps/execute default to -1, unlimited).
	TpsLimiter                  string `value:"${tps-limiter:=}"`                    // tps limiter impl; empty uses dubbo-go default
	TpsLimitRate                int    `value:"${tps-limit-rate:=-1}"`               // allowed requests per interval; negative means unset (unlimited)
	TpsLimitStrategy            string `value:"${tps-limit-strategy:=}"`             // e.g. fixedWindow|slidingWindow|threadSafeFixedWindow
	TpsLimitRejectedHandler     string `value:"${tps-limit-rejected-handler:=}"`     // handler invoked when tps limit is exceeded
	ExecuteLimit                string `value:"${execute-limit:=}"`                  // max concurrent executions; empty/"-1" means unset (unlimited)
	ExecuteLimitRejectedHandler string `value:"${execute-limit-rejected-handler:=}"` // handler invoked when execute limit is exceeded
	ParamSign                   string `value:"${param-sign:=}"`                     // enable request param signing; pair with the "auth" filter

	// Params is an escape hatch for any other provider-level filter parameter
	// (e.g. long-tail filters) passed straight through to dubbo-go.
	Params map[string]string `value:"${params:=}"`

	// Protocols are map-driven: only configured entries are enabled, so one
	// server can expose several protocols at once.
	Protocols map[string]ProtocolCfg `value:"${protocols:=}"`

	// RegistryIDs selects which of the global ${spring.dubbo.registries} this
	// server publishes to. Empty means every global registry.
	RegistryIDs []string `value:"${registry-ids:=}"`
}

// SimpleDubboServer adapts a Dubbo-go server.Server to the Go-Spring server lifecycle.
type SimpleDubboServer struct {
	cfg  ServerConfig
	d    *Instance
	reg  ServiceRegister
	svr  *server.Server
	done chan struct{}
}

// NewSimpleDubboServer creates a SimpleDubboServer from ${spring.dubbo.server} configuration
// and the shared *Instance (global registries and observability).
func NewSimpleDubboServer(cfg ServerConfig, d *Instance, reg ServiceRegister) *SimpleDubboServer {
	return &SimpleDubboServer{cfg: cfg, d: d, reg: reg, done: make(chan struct{})}
}

// buildOptions translates ServerConfig into dubbo-go server options. With no
// protocol configured, a Triple listener on port 20000 is used. Registries are
// selected by RegistryIDs (empty means all).
func (c *ServerConfig) buildOptions(global map[string]RegistryCfg) ([]server.ServerOption, error) {
	var opts []server.ServerOption

	// Provider-wide defaults.
	if c.Group != "" {
		opts = append(opts, server.WithServerGroup(c.Group))
	}
	if c.Version != "" {
		opts = append(opts, server.WithServerVersion(c.Version))
	}
	if c.Cluster != "" {
		opts = append(opts, server.WithServerCluster(c.Cluster))
	}
	if c.LoadBalance != "" {
		opts = append(opts, server.WithServerLoadBalance(c.LoadBalance))
	}
	if c.Serialization != "" {
		opts = append(opts, server.WithServerSerialization(c.Serialization))
	}
	if c.Retries >= 0 {
		opts = append(opts, server.WithServerRetries(c.Retries))
	}
	if c.Filter != "" {
		opts = append(opts, server.WithServerFilter(c.Filter))
	}
	if c.Token != "" {
		opts = append(opts, server.WithServerToken(c.Token))
	}
	if c.AccessLog != "" {
		opts = append(opts, server.WithServerAccesslog(c.AccessLog))
	}
	if c.Auth != "" {
		opts = append(opts, server.WithServerAuth(c.Auth))
	}
	if c.TpsLimiter != "" {
		opts = append(opts, server.WithServerTpsLimiter(c.TpsLimiter))
	}
	if c.TpsLimitRate >= 0 {
		opts = append(opts, server.WithServerTpsLimitRate(c.TpsLimitRate))
	}
	if c.TpsLimitStrategy != "" {
		opts = append(opts, server.WithServerTpsLimitStrategy(c.TpsLimitStrategy))
	}
	if c.TpsLimitRejectedHandler != "" {
		opts = append(opts, server.WithServerTpsLimitRejectedHandler(c.TpsLimitRejectedHandler))
	}
	if c.ExecuteLimit != "" {
		opts = append(opts, server.WithServerExecuteLimit(c.ExecuteLimit))
	}
	if c.ExecuteLimitRejectedHandler != "" {
		opts = append(opts, server.WithServerExecuteLimitRejectedHandler(c.ExecuteLimitRejectedHandler))
	}
	if c.ParamSign != "" {
		opts = append(opts, server.WithServerParamSign(c.ParamSign))
	}
	for k, v := range c.Params {
		opts = append(opts, server.WithServerParam(k, v))
	}
	if c.Tag != "" {
		opts = append(opts, server.WithServerTag(c.Tag))
	}
	if c.Warmup > 0 {
		opts = append(opts, server.WithServerWarmUp(c.Warmup))
	}
	if c.NotRegister {
		opts = append(opts, server.WithServerNotRegister())
	}
	if c.AdaptiveService {
		opts = append(opts, server.WithServerAdaptiveService())
	}

	// Protocol listeners.
	if len(c.Protocols) == 0 {
		// Default: a single Triple listener on port 20000.
		opts = append(opts, server.WithServerProtocol(
			protocol.WithID("tri"),
			protocol.WithProtocol("tri"),
			protocol.WithPort(20000),
		))
	} else {
		for name, pc := range c.Protocols {
			// The map key is the dubbo-go protocol name; WithID keeps each
			// entry distinct in the server's protocol map.
			pOpts := []protocol.ServerOption{
				protocol.WithID(name),
				protocol.WithProtocol(name),
				protocol.WithPort(pc.Port),
			}
			if pc.Ip != "" {
				pOpts = append(pOpts, protocol.WithIp(pc.Ip))
			}
			if len(pc.Params) > 0 {
				pOpts = append(pOpts, protocol.WithParams(pc.Params))
			}
			opts = append(opts, server.WithServerProtocol(pOpts...))
		}
	}

	// Registry publish targets, selected from the global block by RegistryIDs.
	registries, err := selectRegistries(global, c.RegistryIDs)
	if err != nil {
		return nil, err
	}
	for name, r := range registries {
		opts = append(opts, server.WithServerRegistry(r.options(name)...))
	}

	return opts, nil
}

// Run assembles the Dubbo server from ServerConfig and starts serving once
// Go-Spring signals readiness. Dubbo's Serve blocks forever, so it runs in a
// goroutine while Run parks on the done channel; Stop closes done to hand
// control back to Go-Spring's shutdown sequence.
func (s *SimpleDubboServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	opts, err := s.cfg.buildOptions(s.d.Registries())
	if err != nil {
		return err
	}
	// NewServer layers the shared instance config (metadata, metrics, tracing,
	// graceful shutdown) under these options, which take priority.
	svr, err := s.d.NewServer(opts...)
	if err != nil {
		return errutil.Explain(err, "failed to create dubbo server")
	}
	if err = s.reg(svr); err != nil {
		return errutil.Explain(err, "failed to register dubbo service")
	}
	s.svr = svr

	<-sig.TriggerAndWait()

	errCh := make(chan error, 1)
	go func() {
		// Serve exports the service (binding the listeners) and then blocks.
		errCh <- svr.Serve()
	}()

	select {
	case err = <-errCh:
		return errutil.Explain(err, "failed to serve dubbo server")
	case <-s.done:
		return nil
	}
}

// Stop signals Run to return so Go-Spring can complete its shutdown sequence.
func (s *SimpleDubboServer) Stop() error {
	close(s.done)
	return nil
}
