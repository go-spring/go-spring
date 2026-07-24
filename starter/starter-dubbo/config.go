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
	"errors"
	"fmt"
	"time"

	"dubbo.apache.org/dubbo-go/v3"
	"dubbo.apache.org/dubbo-go/v3/client"
	"dubbo.apache.org/dubbo-go/v3/common/config"
	"dubbo.apache.org/dubbo-go/v3/graceful_shutdown"
	"dubbo.apache.org/dubbo-go/v3/metrics"
	"dubbo.apache.org/dubbo-go/v3/otel/trace"
	"dubbo.apache.org/dubbo-go/v3/protocol"
	"dubbo.apache.org/dubbo-go/v3/registry"
	"dubbo.apache.org/dubbo-go/v3/server"
	"go-spring.org/spring/gs"

	// Side-effect import: installs the dubbo-go -> go-spring log bridge (see
	// internal/logger). The bridge self-installs via init(), so no symbols are
	// referenced here - importing this package is what redirects dubbo-go's
	// own logs into the application's go-spring log pipeline.
	_ "go-spring.org/starter-dubbo/internal/logger"

	mapconfig "go-spring.org/starter-dubbo/internal/mapconfig"
)

func init() {
	// Activate mapconfig as dubbo-go's DynamicConfiguration so the dyncPoller
	// can push override rules into the configurator pipeline at runtime.
	config.GetEnvInstance().SetDynamicConfiguration(mapconfig.Singleton())

	gs.Provide(
		NewInstance,
		gs.IndexArg(0, gs.TagArg("${spring.dubbo}")),
	).Condition(gs.OnProperty("spring.dubbo.registries"))
}

// This file defines the canonical config model for the dubbo-go starter. Types
// mirror dubbo-go.json and are bound via go-spring value:"${...}" tags under
// the ${spring.dubbo} prefix. Field types match dubbo-go v3 Option APIs — not
// the raw JSON schema — so options() methods can pass them straight through.

// DubboConfig holds every top-level node that can appear under "dubbo" in
// dubbo-go.json. Bind it with gs.TagArg("${spring.dubbo}").
type DubboConfig struct {
	Application    DubboApplication         `value:"${application:=}"`
	Registries     map[string]DubboRegistry `value:"${registries:=}"`
	Protocols      map[string]DubboProtocol `value:"${protocols:=}"`
	MetadataReport DubboMetadataReport      `value:"${metadata-report:=}"`
	Provider       DubboProvider            `value:"${provider:=}"`
	Consumer       DubboConsumer            `value:"${consumer:=}"`
	Metrics        DubboMetric              `value:"${metrics:=}"`
	Tracing        DubboTracing             `value:"${tracing:=}"`
	Shutdown       DubboShutdown            `value:"${shutdown:=}"`
}

// --- application ---

// DubboApplication holds application metadata for the current process,
// whether it acts as a provider or a consumer.
type DubboApplication struct {
	Organization string `value:"${organization:=dubbo-go}"`
	Name         string `value:"${name:=dubbo.io}"`
	Module       string `value:"${module:=sample}"`
	Group        string `value:"${group:=}"`
	Version      string `value:"${version:=}"`
	Owner        string `value:"${owner:=dubbo-go}"`
	Environment  string `value:"${environment:=}"`
	// MetadataType is "local" or "remote"; default "local".
	MetadataType string `value:"${metadata-type:=local}"`
}

// --- registry ---

// DubboRegistry is a single registry-center entry. Map keys are free-form
// logical IDs validated against ^[_a-zA-Z][a-zA-Z\d_-]*$.
type DubboRegistry struct {
	// Protocol is one of: nacos, etcdv3, polaris, xds, zookeeper,
	// service-discovery-registry.
	Protocol  string `value:"${protocol:=}"`
	Timeout   string `value:"${timeout:=5s}"`
	Group     string `value:"${group:=}"`
	Namespace string `value:"${namespace:=}"`
	TTL       string `value:"${ttl:=10s}"`
	// Address format: {protocol}://address
	Address    string            `value:"${address:=}"`
	Username   string            `value:"${username:=}"`
	Password   string            `value:"${password:=}"`
	Simplified bool              `value:"${simplified:=false}"`
	Preferred  bool              `value:"${preferred:=false}"`
	Zone       string            `value:"${zone:=}"`
	Weight     int64             `value:"${weight:=100}"`
	Params     map[string]string `value:"${params:=}"`
}

// --- protocol ---

// DubboProtocol is a single protocol-listener entry. Map keys are free-form
// logical IDs validated against ^[_a-zA-Z][a-zA-Z\d_-]*$.
type DubboProtocol struct {
	// Name is one of: dubbo, rest, grpc, filter, jsonrpc, tri, registry;
	// default "dubbo".
	Name   string         `value:"${name:=dubbo}"`
	Ip     string         `value:"${ip:=}"`
	Port   int            `value:"${port:=20000}"`
	Params map[string]any `value:"${params:=}"`
}

// --- metadata-report ---

// DubboMetadataReport is the metadata-report entry.
type DubboMetadataReport struct {
	Protocol  string `value:"${protocol:=}"`
	Address   string `value:"${address:=}"`
	Username  string `value:"${username:=}"`
	Password  string `value:"${password:=}"`
	Group     string `value:"${group:=}"`
	Namespace string `value:"${namespace:=}"`
	Timeout   string `value:"${timeout:=20s}"`
}

// --- provider ---

// DubboProvider is the provider-side configuration. Fields match
// dubbo-go v3 server.ServerOption — provider-wide defaults every exported
// service inherits unless overridden per-service.
type DubboProvider struct {
	Filter                 string                  `value:"${filter:=}"`
	RegistryIDs            []string                `value:"${registry-ids:=}"`
	Services               map[string]DubboService `value:"${services:=}"`
	ProtocolIDs            []string                `value:"${protocol-ids:=}"`
	Proxy                  string                  `value:"${proxy:=}"` // no v3 ServerOption; reserved
	TracingKey             string                  `value:"${tracing-key:=}"`
	AdaptiveService        bool                    `value:"${adaptive-service:=false}"`
	AdaptiveServiceVerbose bool                    `value:"${adaptive-service-verbose:=false}"`

	// Provider-wide defaults (server.ServerOption).
	Group                       string            `value:"${group:=}"`
	Version                     string            `value:"${version:=}"`
	Cluster                     string            `value:"${cluster:=}"`
	LoadBalance                 string            `value:"${loadbalance:=}"`
	Serialization               string            `value:"${serialization:=}"`
	Retries                     int               `value:"${retries:=0}"`
	Token                       string            `value:"${token:=}"`
	AccessLog                   string            `value:"${accesslog:=}"`
	Auth                        string            `value:"${auth:=}"`
	Tag                         string            `value:"${tag:=}"`
	Warmup                      string            `value:"${warmup:=}"` // duration string, e.g. "10m"
	NotRegister                 bool              `value:"${not-register:=false}"`
	ParamSign                   string            `value:"${param-sign:=}"`
	Params                      map[string]string `value:"${params:=}"`
	TpsLimiter                  string            `value:"${tps-limiter:=}"`
	TpsLimitRate                int               `value:"${tps-limit-rate:=0}"`
	TpsLimitStrategy            string            `value:"${tps-limit-strategy:=}"`
	TpsLimitRejectedHandler     string            `value:"${tps-limit-rejected-handler:=}"`
	ExecuteLimit                string            `value:"${execute-limit:=}"`
	ExecuteLimitRejectedHandler string            `value:"${execute-limit-rejected-handler:=}"`
}

// DubboService is a per-service entry under provider.services. Map keys are
// free-form logical IDs validated against ^[_a-zA-Z][a-zA-Z\d_-]*$.
// Fields match dubbo-go v3 server.ServiceOption.
type DubboService struct {
	Filter      string   `value:"${filter:=}"`
	ProtocolIDs []string `value:"${protocol-ids:=}"`
	Interface   string   `value:"${interface:=}"`
	RegistryIDs []string `value:"${registry-ids:=}"`
	// Cluster: default "failover".
	Cluster string `value:"${cluster:=failover}"`
	// LoadBalance is one of: random, roundrobin, consistenthashing,
	// leastactive, xdsringhash, p2c; default "random".
	LoadBalance string `value:"${loadbalance:=random}"`
	Retries     int    `value:"${retries:=2}"`
	Group       string `value:"${group:=}"`
	Version     string `value:"${version:=}"`
	// Serialization is one of: protobuf, hessian2, msgpack, jsonMapStruct.
	Serialization string                 `value:"${serialization:=}"`
	Methods       map[string]DubboMethod `value:"${methods:=}"`
	Warmup        string                 `value:"${warmup:=}"` // duration string, e.g. "10m"
	Params        map[string]string      `value:"${params:=}"`
	Token         string                 `value:"${token:=}"`
	AccessLog     string                 `value:"${accesslog:=}"`
	TpsLimiter    string                 `value:"${tps.limiter:=}"`
	TpsLimitRate  int                    `value:"${tps.limit.rate:=0}"`
	// TpsLimitStrategy is one of: threadSafeFixedWindow, slidingWindow,
	// fixedWindow, default.
	TpsLimitStrategy            string `value:"${tps.limit.strategy:=}"`
	TpsLimitRejectedHandler     string `value:"${tps.limit.rejected.handler:=}"`
	ExecuteLimit                string `value:"${execute.limit:=}"`
	ExecuteLimitRejectedHandler string `value:"${execute.limit.rejected.handler:=}"`
	Auth                        string `value:"${auth:=}"`
	ParamSign                   string `value:"${param-sign:=}"`
	Tag                         string `value:"${tag:=}"`
	// MaxMessageSize: no v3 ServiceOption; reserved.
	MaxMessageSize int    `value:"${max_message_size:=4}"`
	TracingKey     string `value:"${tracing-key:=}"`
	NotRegister    bool   `value:"${not-register:=false}"`
}

// --- consumer ---

// DubboConsumer is the consumer-side configuration. Fields match
// dubbo-go v3 client.ClientOption — process-wide defaults every reference
// inherits unless overridden per-reference.
type DubboConsumer struct {
	Filter                         string                    `value:"${filter:=}"`
	RegistryIDs                    []string                  `value:"${registry-ids:=}"`
	Protocol                       string                    `value:"${protocol:=}"`        // dubbo|tri|triple|jsonrpc
	RequestTimeout                 string                    `value:"${request-timeout:=}"` // duration string, e.g. "3s"
	Check                          bool                      `value:"${check:=true}"`
	References                     map[string]DubboReference `value:"${references:=}"`
	Proxy                          string                    `value:"${proxy:=}"` // no v3 ClientOption; reserved
	TracingKey                     string                    `value:"${tracing-key:=}"`
	MaxWaitTimeForServiceDiscovery string                    `value:"${max-wait-time-for-service-discovery:=}"` // no v3 ClientOption; reserved

	// Consumer-level defaults (client.ClientOption).
	Cluster       string `value:"${cluster:=}"`
	LoadBalance   string `value:"${loadbalance:=}"`
	Retries       int    `value:"${retries:=0}"`
	Group         string `value:"${group:=}"`
	Version       string `value:"${version:=}"`
	Serialization string `value:"${serialization:=}"`
	Sticky        bool   `value:"${sticky:=false}"`
	ForceTag      bool   `value:"${force.tag:=false}"`
}

// DubboReference is a per-reference entry under consumer.references. Map keys
// are free-form logical IDs validated against ^[_a-zA-Z][a-zA-Z\d_-]*$.
// Fields match dubbo-go v3 client.ReferenceOption.
type DubboReference struct {
	Interface   string   `value:"${interface:=}"`
	Check       bool     `value:"${check:=false}"`
	URL         string   `value:"${url:=}"`
	Filter      string   `value:"${filter:=}"`
	Protocol    string   `value:"${protocol:=}"`
	RegistryIDs []string `value:"${registry-ids:=}"`
	// Cluster: default "failover".
	Cluster string `value:"${cluster:=failover}"`
	// LoadBalance is one of: random, roundrobin, consistenthashing,
	// leastactive, xdsringhash, p2c; default "random".
	LoadBalance   string                 `value:"${loadbalance:=random}"`
	Retries       int                    `value:"${retries:=0}"`
	Group         string                 `value:"${group:=}"`
	Version       string                 `value:"${version:=}"`
	Serialization string                 `value:"${serialization:=}"`
	Methods       map[string]DubboMethod `value:"${methods:=}"`
	Async         bool                   `value:"${async:=false}"`
	Params        map[string]string      `value:"${params:=}"`
	Generic       bool                   `value:"${generic:=false}"`
	Sticky        bool                   `value:"${sticky:=false}"`
	Timeout       string                 `value:"${timeout:=}"` // duration string, e.g. "3s"
	ForceTag      bool                   `value:"${force.tag:=false}"`
	TracingKey    string                 `value:"${tracing-key:=}"`
}

// --- method (shared by services and references) ---

// DubboMethod is a per-method tuning entry. Map keys are free-form method names
// validated against ^[_a-zA-Z][a-zA-Z\d_-]*$.
// Fields match dubbo-go v3 config.MethodOption.
type DubboMethod struct {
	Name    string `value:"${name:=}"`
	Retries int    `value:"${retries:=2}"`
	// LoadBalance is one of: random, roundrobin, consistenthashing,
	// leastactive, xdsringhash, p2c; default "random".
	LoadBalance      string `value:"${loadbalance:=random}"`
	Weight           int64  `value:"${weight:=100}"`
	TpsLimitInterval int    `value:"${tps.limit.interval:=}"`
	TpsLimitRate     int    `value:"${tps.limit.rate:=}"`
	// TpsLimitStrategy is one of: threadSafeFixedWindow, slidingWindow,
	// fixedWindow, default.
	TpsLimitStrategy            string `value:"${tps.limit.strategy:=}"`
	ExecuteLimit                int    `value:"${execute.limit:=}"`
	ExecuteLimitRejectedHandler string `value:"${execute.limit.rejected.handler:=}"`
	Sticky                      bool   `value:"${sticky:=false}"`
	Timeout                     string `value:"${timeout:=}"` // duration string, e.g. "3s"
}

// --- metrics ---

// DubboMetric is a single metrics entry. Only Prometheus is supported by v3.
type DubboMetric struct {
	Enable             bool   `value:"${enable:=true}"`
	Port               int    `value:"${port:=9090}"`
	Path               string `value:"${path:=/metrics}"`
	PushGatewayAddress string `value:"${push-gateway-address:=}"`
	// Mode/Namespace: no v3 metrics.Option; reserved.
	Mode      string `value:"${mode:=}"`
	Namespace string `value:"${namespace:=}"`
}

// --- tracing ---

// DubboTracing is the OTel tracing config. Fields match dubbo-go v3 trace.Option.
type DubboTracing struct {
	Enable     bool    `value:"${enable:=true}"`
	Exporter   string  `value:"${exporter:=stdout}"`
	Endpoint   string  `value:"${endpoint:=}"`
	Propagator string  `value:"${propagator:=w3c}"`
	Mode       string  `value:"${mode:=}"` // always|never|ratio; empty keeps dubbo-go default
	Ratio      float64 `value:"${ratio:=1.0}"`
	Insecure   bool    `value:"${insecure:=false}"`
	// Legacy jaeger fields: no v3 trace.Option; reserved.
	Name        string `value:"${name:=}"`
	ServiceName string `value:"${serviceName:=}"`
	Address     string `value:"${address:=}"`
	UseAgent    bool   `value:"${use-agent:=false}"`
}

// --- shutdown ---

// DubboShutdown configures graceful shutdown behavior.
// Fields match dubbo-go v3 graceful_shutdown.Option.
type DubboShutdown struct {
	Timeout                string `value:"${timeout:=}"`                   // duration string, e.g. "60s"
	StepTimeout            string `value:"${step-timeout:=}"`              // duration string, e.g. "3s"
	ConsumerUpdateWaitTime string `value:"${consumer-update-wait-time:=}"` // duration string, e.g. "3s"
	RejectHandler          string `value:"${reject-handler:=}"`            // v3 has no named-handler Option yet; non-empty turns on rejection
	InternalSignal         bool   `value:"${internal-signal:=true}"`
	// OfflineRequestWindowTimeout: duration string; v3 graceful_shutdown.Option.
	OfflineRequestWindowTimeout string `value:"${offline-request-window-timeout:=}"`
}

// Instance is the shared top-level facade over the process-wide *dubbo.Instance
// and the complete DubboConfig. It hands out server.Server / client.Client that
// inherit the observability config; both roles depend on it, so its existence
// enables the rest of the starter.
type Instance struct {
	ins *dubbo.Instance
	Cfg DubboConfig
}

// Registries returns the global registries from Cfg.
func (d *Instance) Registries() map[string]DubboRegistry {
	return d.Cfg.Registries
}

// Protocols returns the global protocol listeners from Cfg.
func (d *Instance) Protocols() map[string]DubboProtocol {
	return d.Cfg.Protocols
}

// Consumer returns the consumer config from Cfg.
func (d *Instance) Consumer() DubboConsumer {
	return d.Cfg.Consumer
}

// Provider returns the provider config from Cfg.
func (d *Instance) Provider() DubboProvider {
	return d.Cfg.Provider
}

// NewServer builds a *server.Server from the shared instance, inheriting its
// metrics and tracing. Caller options take priority.
func (d *Instance) NewServer(opts ...server.ServerOption) (*server.Server, error) {
	return d.ins.NewServer(opts...)
}

// NewClient builds a *client.Client from the shared instance, inheriting its
// metrics and tracing. Caller options take priority.
func (d *Instance) NewClient(opts ...client.ClientOption) (*client.Client, error) {
	return d.ins.NewClient(opts...)
}

// NewInstance builds the shared *dubbo.Instance from cfg and captures the global
// registries and protocols.
func NewInstance(cfg DubboConfig) (*Instance, error) {
	app := cfg.Application
	if app.Name == "" {
		return nil, errors.New("${spring.dubbo.application.name} is required")
	}
	if len(cfg.Registries) == 0 {
		return nil, errors.New("${spring.dubbo.registries} must define at least one registry")
	}

	opts := []dubbo.InstanceOption{dubbo.WithName(app.Name)}
	if app.Organization != "" {
		opts = append(opts, dubbo.WithOrganization(app.Organization))
	}
	if app.Module != "" {
		opts = append(opts, dubbo.WithModule(app.Module))
	}
	if app.Version != "" {
		opts = append(opts, dubbo.WithVersion(app.Version))
	}
	if app.Owner != "" {
		opts = append(opts, dubbo.WithOwner(app.Owner))
	}
	if app.Environment != "" {
		opts = append(opts, dubbo.WithEnvironment(app.Environment))
	}
	if app.Group != "" {
		opts = append(opts, dubbo.WithGroup(app.Group))
	}
	if app.MetadataType == "remote" {
		opts = append(opts, dubbo.WithRemoteMetadata())
	}

	// Protocols are process-wide listeners mounted on the Instance; a server
	// built from it inherits them via SetServerProtocols. Empty Name falls
	// back to the map key (the protocol ID).
	for id, pc := range cfg.Protocols {
		opts = append(opts, dubbo.WithProtocol(pc.options(id)...))
	}

	// Metrics.
	if m := cfg.Metrics; m.Enable {
		mopts := []metrics.Option{
			metrics.WithEnabled(),
			metrics.WithPrometheus(),
			metrics.WithPrometheusExporterEnabled(),
		}
		if m.Port > 0 {
			mopts = append(mopts, metrics.WithPort(m.Port))
		}
		if m.Path != "" {
			mopts = append(mopts, metrics.WithPath(m.Path))
		}
		if m.PushGatewayAddress != "" {
			mopts = append(mopts,
				metrics.WithPrometheusPushgatewayEnabled(),
				metrics.WithPrometheusGatewayUrl(m.PushGatewayAddress),
			)
		}
		opts = append(opts, dubbo.WithMetrics(mopts...))
	}

	// Tracing (OTel).
	if t := cfg.Tracing; t.Enable {
		topts := []trace.Option{trace.WithEnabled()}
		if t.Exporter != "" {
			topts = append(topts, trace.WithExporter(t.Exporter))
		}
		if t.Endpoint != "" {
			topts = append(topts, trace.WithEndpoint(t.Endpoint))
		}
		if t.Propagator != "" {
			topts = append(topts, trace.WithPropagator(t.Propagator))
		}
		if t.Mode != "" {
			topts = append(topts, trace.WithMode(t.Mode))
		}
		if t.Insecure {
			topts = append(topts, trace.WithInsecure())
		}
		topts = append(topts, trace.WithRatio(t.Ratio))
		opts = append(opts, dubbo.WithTracing(topts...))
	}

	// Shutdown.
	if sd := cfg.Shutdown; sd.any() {
		sopts := []graceful_shutdown.Option{}
		if sd.Timeout != "" {
			if d, err := time.ParseDuration(sd.Timeout); err == nil && d > 0 {
				sopts = append(sopts, graceful_shutdown.WithTimeout(d))
			}
		}
		if sd.StepTimeout != "" {
			if d, err := time.ParseDuration(sd.StepTimeout); err == nil && d > 0 {
				sopts = append(sopts, graceful_shutdown.WithStepTimeout(d))
			}
		}
		if sd.ConsumerUpdateWaitTime != "" {
			if d, err := time.ParseDuration(sd.ConsumerUpdateWaitTime); err == nil && d > 0 {
				sopts = append(sopts, graceful_shutdown.WithConsumerUpdateWaitTime(d))
			}
		}
		if sd.OfflineRequestWindowTimeout != "" {
			if d, err := time.ParseDuration(sd.OfflineRequestWindowTimeout); err == nil && d > 0 {
				sopts = append(sopts, graceful_shutdown.WithOfflineRequestWindowTimeout(d))
			}
		}
		if sd.RejectHandler != "" {
			sopts = append(sopts, graceful_shutdown.WithRejectRequest())
		}
		if !sd.InternalSignal {
			sopts = append(sopts, graceful_shutdown.WithoutInternalSignal())
		}
		if len(sopts) > 0 {
			opts = append(opts, dubbo.WithShutdown(sopts...))
		}
	}

	ins, err := dubbo.NewInstance(opts...)
	if err != nil {
		return nil, err
	}
	return &Instance{ins: ins, Cfg: cfg}, nil
}

// options translates DubboProtocol into dubbo-go protocol.ServerOptions.
func (pc DubboProtocol) options(id string) []protocol.ServerOption {
	name := pc.Name
	if name == "" {
		name = id
	}
	pOpts := []protocol.ServerOption{
		protocol.WithID(id),
		protocol.WithProtocol(name),
		protocol.WithPort(pc.Port),
	}
	if pc.Ip != "" {
		pOpts = append(pOpts, protocol.WithIp(pc.Ip))
	}
	if len(pc.Params) > 0 {
		pOpts = append(pOpts, protocol.WithParams(pc.Params))
	}
	return pOpts
}

// any reports whether any shutdown field was set.
func (sd DubboShutdown) any() bool {
	return sd.Timeout != "" || sd.StepTimeout != "" ||
		sd.ConsumerUpdateWaitTime != "" || sd.OfflineRequestWindowTimeout != "" ||
		sd.RejectHandler != "" || !sd.InternalSignal
}

// options translates DubboRegistry into dubbo-go registry.Options.
func (rc DubboRegistry) options(id string) []registry.Option {
	regType := rc.Protocol
	if regType == "" {
		regType = id
	}
	opts := []registry.Option{
		registry.WithID(id),
		registry.WithRegistry(regType),
		registry.WithAddress(rc.Address),
	}
	if rc.Namespace != "" {
		opts = append(opts, registry.WithNamespace(rc.Namespace))
	}
	if rc.Group != "" {
		opts = append(opts, registry.WithGroup(rc.Group))
	}
	if rc.Username != "" {
		opts = append(opts, registry.WithUsername(rc.Username))
	}
	if rc.Password != "" {
		opts = append(opts, registry.WithPassword(rc.Password))
	}
	if d, err := time.ParseDuration(rc.Timeout); err == nil && d > 0 {
		opts = append(opts, registry.WithTimeout(d))
	}
	if d, err := time.ParseDuration(rc.TTL); err == nil && d > 0 {
		opts = append(opts, registry.WithTTL(d))
	}
	if rc.Weight >= 0 {
		opts = append(opts, registry.WithWeight(rc.Weight))
	}
	if rc.Zone != "" {
		opts = append(opts, registry.WithZone(rc.Zone))
	}
	if rc.Simplified {
		opts = append(opts, registry.WithSimplified())
	}
	if rc.Preferred {
		opts = append(opts, registry.WithPreferred())
	}
	if len(rc.Params) > 0 {
		opts = append(opts, registry.WithParams(rc.Params))
	}
	return opts
}

// selectRegistries resolves the registries a role uses from the global
// ${spring.dubbo.registries} block. An empty ids list selects every registry;
// otherwise only the listed IDs are returned, and an unknown ID fails fast.
func selectRegistries(registries map[string]DubboRegistry, ids []string) (map[string]DubboRegistry, error) {
	if len(ids) == 0 {
		return registries, nil
	}
	selected := make(map[string]DubboRegistry, len(ids))
	for _, id := range ids {
		rc, ok := registries[id]
		if !ok {
			return nil, fmt.Errorf("dubbo: registry id %q is not defined under ${spring.dubbo.registries}", id)
		}
		selected[id] = rc
	}
	return selected, nil
}
