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

// This file is a faithful Go mirror of dubbo-go.json using go-spring config
// binding syntax. Bind DubboConfig via gs.TagArg("${spring.dubbo}") — every
// nested struct uses value:"${...}" tags that resolve relative to that prefix.
//
// It is intentionally independent of the starter's opinionated config types
// (InstanceConfig, ServerConfig, ClientConfig, etc.) and exposes every field
// the upstream schema defines, including those with no v3 Option yet.

// DubboConfig holds every top-level node that can appear under "dubbo" in
// dubbo-go.json. Bind it with gs.TagArg("${spring.dubbo}").
type DubboConfig struct {
	Profiles       DubboProfiles            `value:"${profiles:=}"`
	Application    DubboApplication         `value:"${application:=}"`
	Registries     map[string]DubboRegistry `value:"${registries:=}"`
	Protocols      map[string]DubboProtocol `value:"${protocols:=}"`
	ConfigCenter   DubboConfigCenter        `value:"${config-center:=}"`
	MetadataReport DubboMetadataReport      `value:"${metadata-report:=}"`
	Provider       DubboProvider            `value:"${provider:=}"`
	Consumer       DubboConsumer            `value:"${consumer:=}"`
	Metrics        map[string]DubboMetric   `value:"${metrics:=}"`
	Tracing        map[string]DubboTracing  `value:"${tracing:=}"`
	Shutdown       DubboShutdown            `value:"${shutdown:=}"`
}

// --- profiles ---

// DubboProfiles controls loading and merging of configuration files based on
// the active profile suffix.
type DubboProfiles struct {
	Active string `value:"${active:=}"` // the file suffix to be loaded
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
	Address    string `value:"${address:=}"`
	Username   string `value:"${username:=}"`
	Password   string `value:"${password:=}"`
	Simplified bool   `value:"${simplified:=false}"`
	// Preferred: always use this registry first when set to true.
	Preferred bool `value:"${preferred:=false}"`
	// Zone: region for traffic isolation.
	Zone string `value:"${zone:=}"`
	// Weight: traffic distribution among registries; default 100.
	Weight int `value:"${weight:=100}"`
	// RegistryType: "service" for application-level discovery.
	RegistryType string         `value:"${registry-type:=}"`
	Params       map[string]any `value:"${params:=}"`
}

// --- protocol ---

// DubboProtocol is a single protocol-listener entry. Map keys are free-form
// logical IDs validated against ^[_a-zA-Z][a-zA-Z\d_-]*$.
type DubboProtocol struct {
	// Name is one of: dubbo, rest, grpc, filter, jsonrpc, tri, registry;
	// default "dubbo".
	Name   string         `value:"${name:=dubbo}"`
	Ip     string         `value:"${ip:=}"`
	Port   float64        `value:"${port:=20000}"` // schema type is "number"
	Params map[string]any `value:"${params:=}"`
}

// --- config-center ---

// DubboConfigCenter is the remote configuration-center entry.
type DubboConfigCenter struct {
	// Protocol is one of: nacos, apollo, file, zookeeper.
	Protocol  string         `value:"${protocol:=}"`
	Address   string         `value:"${address:=}"` // format {protocol}://address
	DataID    string         `value:"${data-id:=}"` // data id for nacos
	AppID     string         `value:"${app-id:=}"`  // app id for apollo
	Cluster   string         `value:"${cluster:=}"`
	Username  string         `value:"${username:=}"`
	Password  string         `value:"${password:=}"`
	Group     string         `value:"${group:=}"`
	Namespace string         `value:"${namespace:=}"`
	Params    map[string]any `value:"${params:=}"`
	Timeout   string         `value:"${timeout:=5s}"`
	// FileExtension is the suffix of config dataId, also the file extension
	// of config content: json, toml, yaml, yml, properties.
	FileExtension string `value:"${file-extension:=}"`
}

// --- metadata-report ---

// DubboMetadataReport is the metadata-report entry.
type DubboMetadataReport struct {
	Protocol  string `value:"${protocol:=}"`
	Address   string `value:"${address:=}"` // format {protocol}://address
	Username  string `value:"${username:=}"`
	Password  string `value:"${password:=}"`
	Group     string `value:"${group:=}"`
	Namespace string `value:"${namespace:=}"`
	Timeout   string `value:"${timeout:=20s}"`
}

// --- provider ---

// DubboProvider is the provider-side configuration.
type DubboProvider struct {
	Filter                 string                  `value:"${filter:=}"`
	Register               bool                    `value:"${register:=false}"`
	RegistryIDs            []string                `value:"${registry-ids:=}"`
	TracingKey             string                  `value:"${tracing-key:=}"`
	Proxy                  string                  `value:"${proxy:=default}"`
	AdaptiveService        bool                    `value:"${adaptive-service:=false}"`
	AdaptiveServiceVerbose bool                    `value:"${adaptive-service-verbose:=false}"`
	Services               map[string]DubboService `value:"${services:=}"`
}

// DubboService is a per-service entry under provider.services. Map keys are
// free-form logical IDs validated against ^[_a-zA-Z][a-zA-Z\d_-]*$.
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
	// Retries: retry count; default 2; schema type is "number".
	Retries float64 `value:"${retries:=2}"`
	Group   string  `value:"${group:=}"`
	Version string  `value:"${version:=}"`
	// Serialization is one of: protobuf, hessian2, msgpack, jsonMapStruct.
	Serialization string                 `value:"${serialization:=}"`
	Methods       map[string]DubboMethod `value:"${methods:=}"`
	// Warmup: service register warm-up time in ms; default 600; schema type "number".
	Warmup float64        `value:"${warmup:=600}"`
	Params map[string]any `value:"${params:=}"`
	// Token: set "true" or "default" to use uuid.
	Token     string `value:"${token:=}"`
	AccessLog string `value:"${accesslog:=}"`
	// TPSLimiter is one of: method-service, default.
	TPSLimiter       string `value:"${tps.limiter:=}"`
	TPSLimitInterval int    `value:"${tps.limit.interval:=}"`
	// TPSLimitStrategy is one of: threadSafeFixedWindow, slidingWindow,
	// fixedWindow, default.
	TPSLimitStrategy            string  `value:"${tps.limit.strategy:=}"`
	TPSLimitRate                int     `value:"${tps.limit.rate:=}"`
	TPSLimitRejectedHandler     string  `value:"${tps.limit.rejected.handler:=log}"`
	ExecuteLimit                float64 `value:"${execute.limit:=}"` // schema type "number"
	ExecuteLimitRejectedHandler string  `value:"${execute.limit.rejected.handler:=log}"`
	Auth                        bool    `value:"${auth:=false}"`
	ParamSign                   bool    `value:"${param.sign:=false}"`
	Tag                         string  `value:"${tag:=}"`
	// MaxMessageSize: default 4 (MB).
	MaxMessageSize int    `value:"${max_message_size:=4}"`
	TracingKey     string `value:"${tracing-key:=}"`
}

// --- consumer ---

// DubboConsumer is the consumer-side configuration.
type DubboConsumer struct {
	Filter                         string                    `value:"${filter:=}"`
	RegistryIDs                    []string                  `value:"${registry-ids:=}"`
	RequestTimeout                 string                    `value:"${request-timeout:=3s}"`
	Proxy                          string                    `value:"${proxy:=default}"`
	Check                          bool                      `value:"${check:=false}"`
	AdaptiveService                bool                      `value:"${adaptive-service:=false}"`
	TracingKey                     string                    `value:"${tracing-key:=}"`
	MaxWaitTimeForServiceDiscovery string                    `value:"${max-wait-time-for-service-discovery:=3s}"`
	References                     map[string]DubboReference `value:"${references:=}"`
}

// DubboReference is a per-reference entry under consumer.references. Map keys
// are free-form logical IDs validated against ^[_a-zA-Z][a-zA-Z\d_-]*$.
// Note: unlike DubboService, reference has no additionalProperties:false in
// the schema, so it is open to extra fields.
type DubboReference struct {
	Interface   string   `value:"${interface:=}"`
	Check       bool     `value:"${check:=false}"`
	URL         string   `value:"${url:=}"`
	Filter      string   `value:"${filter:=}"`
	Protocol    string   `value:"${protocol:=tri}"`
	RegistryIDs []string `value:"${registry-ids:=}"`
	// Cluster: default "failover".
	Cluster string `value:"${cluster:=failover}"`
	// LoadBalance is one of: random, roundrobin, consistenthashing,
	// leastactive, xdsringhash, p2c; default "random".
	LoadBalance string `value:"${loadbalance:=random}"`
	Group       string `value:"${group:=}"`
	Version     string `value:"${version:=}"`
	// Serialization is one of: protobuf, hessian2, msgpack, jsonMapStruct.
	Serialization string                 `value:"${serialization:=}"`
	Methods       map[string]DubboMethod `value:"${methods:=}"`
	Async         bool                   `value:"${async:=false}"`
	Params        map[string]any         `value:"${params:=}"`
	Generic       bool                   `value:"${generic:=false}"`
	Sticky        bool                   `value:"${sticky:=false}"`
	Timeout       string                 `value:"${timeout:=}"`
	ForceTag      bool                   `value:"${force.tag:=false}"`
	TracingKey    string                 `value:"${tracing-key:=}"`
}

// --- method (shared by services and references) ---

// DubboMethod is a per-method tuning entry. Map keys are free-form method names
// validated against ^[_a-zA-Z][a-zA-Z\d_-]*$.
type DubboMethod struct {
	Name    string  `value:"${name:=}"`
	Retries float64 `value:"${retries:=2}"` // retry count; schema type "number"
	// LoadBalance is one of: random, roundrobin, consistenthashing,
	// leastactive, xdsringhash, p2c; default "random".
	LoadBalance string `value:"${loadbalance:=random}"`
	// Weight: default 100.
	Weight           int `value:"${weight:=100}"`
	TPSLimitInterval int `value:"${tps.limit.interval:=}"`
	TPSLimitRate     int `value:"${tps.limit.rate:=}"`
	// TPSLimitStrategy is one of: threadSafeFixedWindow, slidingWindow,
	// fixedWindow, default.
	TPSLimitStrategy            string  `value:"${tps.limit.strategy:=}"`
	ExecuteLimit                float64 `value:"${execute.limit:=}"` // schema type "number"
	ExecuteLimitRejectedHandler string  `value:"${execute.limit.rejected.handler:=log}"`
	Sticky                      bool    `value:"${sticky:=false}"`
	Timeout                     string  `value:"${timeout:=}"`
}

// --- metrics ---

// DubboMetric is a single metrics entry. Map keys are free-form logical IDs
// validated against ^[_a-zA-Z][a-zA-Z\d_-]*$.
type DubboMetric struct {
	Mode               string `value:"${mode:=pull}"`
	Namespace          string `value:"${namespace:=dubbo}"`
	Enable             bool   `value:"${enable:=true}"`
	Port               int    `value:"${port:=9090}"`
	Path               string `value:"${path:=/metrics}"`
	PushGatewayAddress string `value:"${push-gateway-address:=}"`
}

// --- tracing ---

// DubboTracing is a single tracing entry. Map keys are free-form logical IDs
// validated against ^[_a-zA-Z][a-zA-Z\d_-]*$.
type DubboTracing struct {
	Name        string `value:"${name:=jaeger}"`
	ServiceName string `value:"${serviceName:=}"`
	Address     string `value:"${address:=}"`
	UseAgent    bool   `value:"${use-agent:=false}"`
}

// --- shutdown ---

// DubboShutdown configures graceful shutdown behavior.
type DubboShutdown struct {
	Timeout                string `value:"${timeout:=60s}"`
	StepTimeout            string `value:"${step-timeout:=3s}"`
	ConsumerUpdateWaitTime string `value:"${consumer-update-wait-time:=3s}"`
	RejectHandler          string `value:"${reject-handler:=}"`
	InternalSignal         bool   `value:"${internal-signal:=true}"`
}
