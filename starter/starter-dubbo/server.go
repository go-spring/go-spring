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
	"runtime"
	"time"

	"dubbo.apache.org/dubbo-go/v3/config"
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

// ServerConfig defines Dubbo server configuration: the provider-level defaults,
// the role-specific registry selection, and the filter tuning knobs. Protocols
// are no longer server-scoped — they live on the global ${spring.dubbo.protocols}
// node and are inherited from the shared Instance; this config only carries the
// fallback decision (none configured → a single Triple:20000 listener). Every
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

	// RegistryIDs selects which of the global ${spring.dubbo.registries} this
	// server publishes to. Empty means every global registry.
	RegistryIDs []string `value:"${registry-ids:=}"`

	// Services carries per-service overrides under ${spring.dubbo.server.services.<name>}
	// (keyed by the name passed to RegisterService). Empty fields fall back to the
	// provider-wide defaults above - dubbo-go merges provider-wide (SetProvider)
	// with these per-service ServiceOptions, service taking priority. Each entry
	// may also carry per-method tuning under .methods.<method>. Mirrors
	// dubbo-go.json's provider.services node (this starter's "server" = "provider").
	Services map[string]ServiceCfg `value:"${services:=}"`
}

// ServiceCfg holds per-service overrides under ${spring.dubbo.server.services.<name>},
// the per-service counterpart to ServerConfig's provider-wide defaults. Every
// field is optional; empty/zero keeps the provider-wide default (dubbo-go merges
// them, service-level wins). Methods carries per-method tuning. Mirrors
// dubbo-go.json's services.<name> (only fields with a v3 server.ServiceOption are
// surfaced; interface/max_message_size/tracing-key and service-level
// tps-limit-interval have no v3 service-level Option).
//
// Enum-like fields accept the dubbo-go names:
//   - Cluster:       failover(default)|failfast|failsafe|failback|forking|available|broadcast|zoneAware
//   - LoadBalance:   random(default)|roundrobin|leastactive|consistenthashing|p2c
//   - Serialization: hessian2|protobuf|msgpack|json
type ServiceCfg struct {
	Group                       string               `value:"${group:=}"`
	Version                     string               `value:"${version:=}"`
	Cluster                     string               `value:"${cluster:=}"`
	LoadBalance                 string               `value:"${load-balance:=}"`
	Serialization               string               `value:"${serialization:=}"`
	Retries                     int                  `value:"${retries:=-1}"` // negative means unset; 0 explicitly disables
	Filter                      string               `value:"${filter:=}"`
	Token                       string               `value:"${token:=}"`
	AccessLog                   string               `value:"${access-log:=}"`
	Auth                        string               `value:"${auth:=}"`
	Tag                         string               `value:"${tag:=}"`
	Warmup                      time.Duration        `value:"${warmup:=}"`
	NotRegister                 bool                 `value:"${not-register:=false}"`
	ProtocolIDs                 []string             `value:"${protocol-ids:=}"`
	RegistryIDs                 []string             `value:"${registry-ids:=}"`
	TpsLimiter                  string               `value:"${tps-limiter:=}"`
	TpsLimitRate                int                  `value:"${tps-limit-rate:=-1}"`
	TpsLimitStrategy            string               `value:"${tps-limit-strategy:=}"`
	TpsLimitRejectedHandler     string               `value:"${tps-limit-rejected-handler:=}"`
	ExecuteLimit                string               `value:"${execute-limit:=}"` // v3 service-level takes a string
	ExecuteLimitRejectedHandler string               `value:"${execute-limit-rejected-handler:=}"`
	ParamSign                   string               `value:"${param-sign:=}"`
	Params                      map[string]string    `value:"${params:=}"`
	Methods                     map[string]MethodCfg `value:"${methods:=}"`
}

// options translates a ServiceCfg into dubbo-go server.ServiceOptions: the
// per-service overrides plus one server.WithMethod per configured method.
// Empty/zero fields are skipped so the provider-wide default applies.
func (s ServiceCfg) options() []server.ServiceOption {
	var opts []server.ServiceOption
	if s.Group != "" {
		opts = append(opts, server.WithGroup(s.Group))
	}
	if s.Version != "" {
		opts = append(opts, server.WithVersion(s.Version))
	}
	if s.Cluster != "" {
		opts = append(opts, server.WithCluster(s.Cluster))
	}
	if s.LoadBalance != "" {
		opts = append(opts, server.WithLoadBalance(s.LoadBalance))
	}
	if s.Serialization != "" {
		opts = append(opts, server.WithSerialization(s.Serialization))
	}
	if s.Retries >= 0 {
		opts = append(opts, server.WithRetries(s.Retries))
	}
	if s.Filter != "" {
		opts = append(opts, server.WithFilter(s.Filter))
	}
	if s.Token != "" {
		opts = append(opts, server.WithToken(s.Token))
	}
	if s.AccessLog != "" {
		opts = append(opts, server.WithAccesslog(s.AccessLog))
	}
	if s.Auth != "" {
		opts = append(opts, server.WithAuth(s.Auth))
	}
	if s.Tag != "" {
		opts = append(opts, server.WithTag(s.Tag))
	}
	if s.Warmup > 0 {
		opts = append(opts, server.WithWarmUp(s.Warmup))
	}
	if s.NotRegister {
		opts = append(opts, server.WithNotRegister())
	}
	if len(s.ProtocolIDs) > 0 {
		opts = append(opts, server.WithProtocolIDs(s.ProtocolIDs))
	}
	if len(s.RegistryIDs) > 0 {
		opts = append(opts, server.WithRegistryIDs(s.RegistryIDs))
	}
	if s.TpsLimiter != "" {
		opts = append(opts, server.WithTpsLimiter(s.TpsLimiter))
	}
	if s.TpsLimitRate >= 0 {
		opts = append(opts, server.WithTpsLimitRate(s.TpsLimitRate))
	}
	if s.TpsLimitStrategy != "" {
		opts = append(opts, server.WithTpsLimitStrategy(s.TpsLimitStrategy))
	}
	if s.TpsLimitRejectedHandler != "" {
		opts = append(opts, server.WithTpsLimitRejectedHandler(s.TpsLimitRejectedHandler))
	}
	if s.ExecuteLimit != "" {
		opts = append(opts, server.WithExecuteLimit(s.ExecuteLimit))
	}
	if s.ExecuteLimitRejectedHandler != "" {
		opts = append(opts, server.WithExecuteLimitRejectedHandler(s.ExecuteLimitRejectedHandler))
	}
	if s.ParamSign != "" {
		opts = append(opts, server.WithParamSign(s.ParamSign))
	}
	for k, v := range s.Params {
		opts = append(opts, server.WithParam(k, v))
	}
	// Per-method tuning: one WithMethod per configured method, each carrying its
	// own MethodOption set. The map key is the dubbo-go method name; it defaults
	// MethodCfg.Name when Name is not explicitly set, so the config attaches to
	// the right method.
	for name, m := range s.Methods {
		if m.Name == "" {
			m.Name = name
		}
		if mopts := m.options(); len(mopts) > 0 {
			opts = append(opts, server.WithMethod(mopts...))
		}
	}
	return opts
}

// MethodCfg holds per-method tuning under
// ${spring.dubbo.server.services.<name>.methods.<method>}, overriding the
// service/provider defaults for that one method. The map key is the dubbo-go
// method name; Name is optional (defaults to the key). All fields optional;
// empty/zero keeps the service/provider default. Mirrors dubbo-go.json's
// method node; every field has a v3 config.MethodOption.
type MethodCfg struct {
	Name                        string        `value:"${name:=}"`
	Retries                     int           `value:"${retries:=-1}"`
	LoadBalance                 string        `value:"${load-balance:=}"`
	Weight                      int64         `value:"${weight:=-1}"`
	TpsLimitInterval            int           `value:"${tps-limit-interval:=-1}"`
	TpsLimitRate                int           `value:"${tps-limit-rate:=-1}"`
	TpsLimitStrategy            string        `value:"${tps-limit-strategy:=}"`
	ExecuteLimit                int           `value:"${execute-limit:=-1}"` // v3 method-level takes an int (unlike service-level string)
	ExecuteLimitRejectedHandler string        `value:"${execute-limit-rejected-handler:=}"`
	Sticky                      bool          `value:"${sticky:=false}"`
	Timeout                     time.Duration `value:"${timeout:=}"`
}

// options translates a MethodCfg into dubbo-go config.MethodOptions (passed to a
// service via server.WithMethod). Empty/zero fields are skipped.
func (m MethodCfg) options() []config.MethodOption {
	var opts []config.MethodOption
	if m.Name != "" {
		opts = append(opts, config.WithName(m.Name))
	}
	if m.Retries >= 0 {
		opts = append(opts, config.WithRetries(m.Retries))
	}
	if m.LoadBalance != "" {
		opts = append(opts, config.WithLoadBalance(m.LoadBalance))
	}
	if m.Weight >= 0 {
		opts = append(opts, config.WithWeight(m.Weight))
	}
	if m.TpsLimitInterval >= 0 {
		opts = append(opts, config.WithTpsLimitInterval(m.TpsLimitInterval))
	}
	if m.TpsLimitRate >= 0 {
		opts = append(opts, config.WithTpsLimitRate(m.TpsLimitRate))
	}
	if m.TpsLimitStrategy != "" {
		opts = append(opts, config.WithTpsLimitStrategy(m.TpsLimitStrategy))
	}
	if m.ExecuteLimit >= 0 {
		opts = append(opts, config.WithExecuteLimit(m.ExecuteLimit))
	}
	if m.ExecuteLimitRejectedHandler != "" {
		opts = append(opts, config.WithExecuteLimitRejectedHandler(m.ExecuteLimitRejectedHandler))
	}
	if m.Sticky {
		opts = append(opts, config.WithSticky())
	}
	if m.Timeout > 0 {
		opts = append(opts, config.WithRequestTimeout(m.Timeout))
	}
	return opts
}

// SimpleDubboServer adapts a Dubbo-go server.Server to the Go-Spring server lifecycle.
type SimpleDubboServer struct {
	cfg ServerConfig
	d   *Instance
	// Regs collects every ServiceRegister bean - one per RegisterService call,
	// or a single legacy gs.Provide(func() ServiceRegister{...}). gs autowires the
	// slice (autowire:"?" keeps it optional; the OnBean[ServiceRegister] gate is
	// what actually decides whether a server stands up). Each is invoked once in
	// Run, so multi-service apps just call RegisterService per stub.
	Regs []ServiceRegister `autowire:"?"`
	svr  *server.Server
	done chan struct{}
}

// NewSimpleDubboServer creates a SimpleDubboServer from ${spring.dubbo.server}
// configuration and the shared *Instance (global registries and observability).
// The ServiceRegister beans are autowired into Regs after construction.
func NewSimpleDubboServer(cfg ServerConfig, d *Instance) *SimpleDubboServer {
	return &SimpleDubboServer{cfg: cfg, d: d, done: make(chan struct{})}
}

// buildOptions translates ServerConfig into dubbo-go server options. Protocols
// are the global ${spring.dubbo.protocols} from the Instance (injected into the
// server by ins.NewServer, so they need no server-side Option here); only when
// that set is empty does this server fall back to a Triple:20000 listener.
// Registries are selected by RegistryIDs (empty means all).
func (c *ServerConfig) buildOptions(protocols map[string]ProtocolCfg, global map[string]RegistryCfg) ([]server.ServerOption, error) {
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

	// Protocol listeners come from the global ${spring.dubbo.protocols} block on
	// the Instance: dubbo-go's ins.NewServer already injects them via
	// SetServerProtocols, so when any are configured there is nothing to add
	// here (re-adding would duplicate the listeners). Only when none are
	// configured globally does this server stand up its own Triple:20000
	// fallback, so a server with no explicit protocols still serves.
	if len(protocols) == 0 {
		opts = append(opts, server.WithServerProtocol(
			protocol.WithID("tri"),
			protocol.WithProtocol("tri"),
			protocol.WithPort(20000),
		))
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
	opts, err := s.cfg.buildOptions(s.d.Protocols(), s.d.Registries())
	if err != nil {
		return err
	}
	// NewServer layers the shared instance config (metadata, metrics, tracing,
	// graceful shutdown) under these options, which take priority.
	svr, err := s.d.NewServer(opts...)
	if err != nil {
		return errutil.Explain(err, "failed to create dubbo server")
	}
	if err = s.regAll(svr); err != nil {
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

// regAll invokes every collected ServiceRegister bean against the assembled
// server. Each bean registers one service with its own per-service/per-method
// options already baked in (see RegisterService).
func (s *SimpleDubboServer) regAll(svr *server.Server) error {
	for _, reg := range s.Regs {
		if err := reg(svr); err != nil {
			return err
		}
	}
	return nil
}

// RegisterService registers a Dubbo service as a ServiceRegister bean, so
// SimpleDubboServer invokes it (alongside any others) when the server starts.
// Pass the code-generated register function (e.g. greet.RegisterGreetServiceHandler,
// shape func(*server.Server, <handler>, ...server.ServiceOption) error) and the
// handler value:
//
//	StarterDubbo.RegisterService("greet", greet.RegisterGreetServiceHandler,
//	    greet.GreetServiceHandler(&GreetProvider{}))
//
// T is inferred from register's handler parameter (the generated handler
// interface, e.g. greet.GreetServiceHandler). hdlr is typed T, so the app
// converts the concrete handler to that interface at the call site - a Go
// generics limitation: a concrete value does not match a type parameter
// inferred as an interface, so the explicit conversion is what lets the
// signature stay type-safe (no runtime assertion).
//
// The starter only provides this helper; the app calls it explicitly per service
// (like RegisterReference on the client side). It binds the global
// ${spring.dubbo.server} config (carrying the Services map) into the register
// bean, then at registration time extracts ${spring.dubbo.server.services.<name>}
// and turns it into dubbo-go server.ServiceOption via ServiceCfg.options (per-
// service overrides + one server.WithMethod per configured method), passing
// those opts into register. Empty fields keep the provider-wide defaults; if
// <name> is not configured at all, no ServiceOption is passed and the service
// runs on provider-wide defaults.
//
// name is the key under ${spring.dubbo.server.services} to bind. Multiple
// services = multiple RegisterService calls; SimpleDubboServer collects and
// invokes them all.
//
// Must be called before gs.Run() (e.g. in main or an init), like gs.Provide.
func RegisterService[T any](name string, register func(*server.Server, T, ...server.ServiceOption) error, hdlr T) {
	b := gs.Provide(func(cfg ServerConfig) ServiceRegister {
		return func(svr *server.Server) error {
			var opts []server.ServiceOption
			if svc, ok := cfg.Services[name]; ok {
				opts = svc.options()
			}
			return register(svr, hdlr, opts...)
		}
	}, gs.IndexArg(0, gs.TagArg("${spring.dubbo.server}")))
	// gs.Provide records its own call site (inside this helper) as the bean's
	// source location. Override it with the app's call site so the bean points
	// where RegisterService was invoked - transparent vs. calling gs.Provide
	// inline, which matters now that this is a shared library helper.
	if _, file, line, ok := runtime.Caller(1); ok {
		b.SetFileLine(file, line)
	}
}
