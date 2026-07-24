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

// This file is a faithful Go mirror of dubbo-go.json. Every definition,
// every property, every nested map matches the upstream schema exactly.
// It is intentionally independent of the starter's opinionated config types.

// DubboRoot is the top-level container. The dubbo-go.json schema wraps
// everything under a single "dubbo" key.
type DubboRoot struct {
	Dubbo DubboConfig `json:"dubbo"`
}

// DubboConfig holds every top-level node that can appear under "dubbo".
type DubboConfig struct {
	Profiles       DubboProfiles            `json:"profiles,omitempty"`
	Application    DubboApplication         `json:"application,omitempty"`
	Registries     map[string]DubboRegistry `json:"registries,omitempty"`
	Protocols      map[string]DubboProtocol `json:"protocols,omitempty"`
	ConfigCenter   DubboConfigCenter        `json:"config-center,omitempty"`
	MetadataReport DubboMetadataReport      `json:"metadata-report,omitempty"`
	Provider       DubboProvider            `json:"provider,omitempty"`
	Consumer       DubboConsumer            `json:"consumer,omitempty"`
	Metrics        map[string]DubboMetric   `json:"metrics,omitempty"`
	Tracing        map[string]DubboTracing  `json:"tracing,omitempty"`
	Shutdown       DubboShutdown            `json:"shutdown,omitempty"`
}

// --- profiles ---

// DubboProfiles controls loading and merging of configuration files based on
// the active profile suffix.
type DubboProfiles struct {
	Active string `json:"active,omitempty"` // the file suffix to be loaded
}

// --- application ---

// DubboApplication holds application metadata for the current process,
// whether it acts as a provider or a consumer.
type DubboApplication struct {
	Organization string `json:"organization,omitempty"` // default "dubbo-go"
	Name         string `json:"name,omitempty"`         // default "dubbo.io"
	Module       string `json:"module,omitempty"`       // default "sample"
	Group        string `json:"group,omitempty"`
	Version      string `json:"version,omitempty"`
	Owner        string `json:"owner,omitempty"` // default "dubbo-go"
	Environment  string `json:"environment,omitempty"`
	MetadataType string `json:"metadata-type,omitempty"` // "local" or "remote"; default "local"
}

// --- registry ---

// DubboRegistry is a single registry-center entry. Keys are free-form
// logical IDs validated against ^[_a-zA-Z][a-zA-Z\d_-]*$.
type DubboRegistry struct {
	Protocol     string         `json:"protocol,omitempty"` // nacos|etcdv3|polaris|xds|zookeeper|service-discovery-registry
	Timeout      string         `json:"timeout,omitempty"`  // default "5s"
	Group        string         `json:"group,omitempty"`
	Namespace    string         `json:"namespace,omitempty"`
	TTL          string         `json:"ttl,omitempty"`     // default "10s"
	Address      string         `json:"address,omitempty"` // format {protocol}://address
	Username     string         `json:"username,omitempty"`
	Password     string         `json:"password,omitempty"`
	Simplified   bool           `json:"simplified,omitempty"`
	Preferred    bool           `json:"preferred,omitempty"`     // always use this registry first
	Zone         string         `json:"zone,omitempty"`          // region for traffic isolation
	Weight       int            `json:"weight,omitempty"`        // default 100; traffic distribution among registries
	RegistryType string         `json:"registry-type,omitempty"` // "service" for application-level discovery
	Params       map[string]any `json:"params,omitempty"`        // extra params passed to the registry impl
}

// --- protocol ---

// DubboProtocol is a single protocol-listener entry. Keys are free-form
// logical IDs validated against ^[_a-zA-Z][a-zA-Z\d_-]*$.
type DubboProtocol struct {
	Name   string         `json:"name,omitempty"` // dubbo|rest|grpc|filter|jsonrpc|tri|registry; default "dubbo"
	Ip     string         `json:"ip,omitempty"`
	Port   float64        `json:"port,omitempty"` // default 20000; schema type is "number"
	Params map[string]any `json:"params,omitempty"`
}

// --- config-center ---

// DubboConfigCenter is the remote configuration-center entry.
type DubboConfigCenter struct {
	Protocol      string         `json:"protocol,omitempty"` // nacos|apollo|file|zookeeper
	Address       string         `json:"address,omitempty"`  // format {protocol}://address
	DataID        string         `json:"data-id,omitempty"`  // data id for nacos
	AppID         string         `json:"app-id,omitempty"`   // app id for apollo
	Cluster       string         `json:"cluster,omitempty"`
	Username      string         `json:"username,omitempty"`
	Password      string         `json:"password,omitempty"`
	Group         string         `json:"group,omitempty"`
	Namespace     string         `json:"namespace,omitempty"`
	Params        map[string]any `json:"params,omitempty"`         // extra params passed to the config-center impl
	Timeout       string         `json:"timeout,omitempty"`        // default "5s"
	FileExtension string         `json:"file-extension,omitempty"` // json|toml|yaml|yml|properties; suffix of config dataId
}

// --- metadata-report ---

// DubboMetadataReport is the metadata-report entry.
type DubboMetadataReport struct {
	Protocol  string `json:"protocol,omitempty"`
	Address   string `json:"address,omitempty"` // format {protocol}://address
	Username  string `json:"username,omitempty"`
	Password  string `json:"password,omitempty"`
	Group     string `json:"group,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Timeout   string `json:"timeout,omitempty"` // default "20s"
}

// --- provider ---

// DubboProvider is the provider-side configuration.
type DubboProvider struct {
	Filter                 string                  `json:"filter,omitempty"`
	Register               bool                    `json:"register,omitempty"`
	RegistryIDs            []string                `json:"registry-ids,omitempty"`
	TracingKey             string                  `json:"tracing-key,omitempty"`
	Proxy                  string                  `json:"proxy,omitempty"` // default "default"
	AdaptiveService        bool                    `json:"adaptive-service,omitempty"`
	AdaptiveServiceVerbose bool                    `json:"adaptive-service-verbose,omitempty"`
	Services               map[string]DubboService `json:"services,omitempty"`
}

// DubboService is a per-service entry under provider.services. Keys are
// free-form logical IDs validated against ^[_a-zA-Z][a-zA-Z\d_-]*$.
type DubboService struct {
	Filter                      string                 `json:"filter,omitempty"`
	ProtocolIDs                 []string               `json:"protocol-ids,omitempty"`
	Interface                   string                 `json:"interface,omitempty"`
	RegistryIDs                 []string               `json:"registry-ids,omitempty"`
	Cluster                     string                 `json:"cluster,omitempty"`     // default "failover"
	LoadBalance                 string                 `json:"loadbalance,omitempty"` // random|roundrobin|consistenthashing|leastactive|xdsringhash|p2c; default "random"
	Retries                     float64                `json:"retries,omitempty"`     // default 2; schema type is "number"
	Group                       string                 `json:"group,omitempty"`
	Version                     string                 `json:"version,omitempty"`
	Serialization               string                 `json:"serialization,omitempty"` // protobuf|hessian2|msgpack|jsonMapStruct
	Methods                     map[string]DubboMethod `json:"methods,omitempty"`
	Warmup                      float64                `json:"warmup,omitempty"` // default 600 (ms); schema type is "number"
	Params                      map[string]any         `json:"params,omitempty"`
	Token                       string                 `json:"token,omitempty"` // set "true" or "default" to use uuid
	AccessLog                   string                 `json:"accesslog,omitempty"`
	TPSLimiter                  string                 `json:"tps.limiter,omitempty"` // method-service|default
	TPSLimitInterval            int                    `json:"tps.limit.interval,omitempty"`
	TPSLimitStrategy            string                 `json:"tps.limit.strategy,omitempty"` // threadSafeFixedWindow|slidingWindow|fixedWindow|default
	TPSLimitRate                int                    `json:"tps.limit.rate,omitempty"`
	TPSLimitRejectedHandler     string                 `json:"tps.limit.rejected.handler,omitempty"`     // default "log"
	ExecuteLimit                float64                `json:"execute.limit,omitempty"`                  // schema type is "number"
	ExecuteLimitRejectedHandler string                 `json:"execute.limit.rejected.handler,omitempty"` // default "log"
	Auth                        bool                   `json:"auth,omitempty"`                           // default false
	ParamSign                   bool                   `json:"param.sign,omitempty"`                     // default false
	Tag                         string                 `json:"tag,omitempty"`
	MaxMessageSize              int                    `json:"max_message_size,omitempty"` // default 4 (MB)
	TracingKey                  string                 `json:"tracing-key,omitempty"`
}

// --- consumer ---

// DubboConsumer is the consumer-side configuration.
type DubboConsumer struct {
	Filter                         string                    `json:"filter,omitempty"`
	RegistryIDs                    []string                  `json:"registry-ids,omitempty"`
	RequestTimeout                 string                    `json:"request-timeout,omitempty"` // default "3s"
	Proxy                          string                    `json:"proxy,omitempty"`           // default "default"
	Check                          bool                      `json:"check,omitempty"`
	AdaptiveService                bool                      `json:"adaptive-service,omitempty"`
	TracingKey                     string                    `json:"tracing-key,omitempty"`
	MaxWaitTimeForServiceDiscovery string                    `json:"max-wait-time-for-service-discovery,omitempty"` // default "3s"
	References                     map[string]DubboReference `json:"references,omitempty"`
}

// DubboReference is a per-reference entry under consumer.references. Keys are
// free-form logical IDs validated against ^[_a-zA-Z][a-zA-Z\d_-]*$.
// Note: unlike DubboService, reference has no additionalProperties:false in
// the schema, so it is open to extra fields.
type DubboReference struct {
	Interface     string                 `json:"interface,omitempty"`
	Check         bool                   `json:"check,omitempty"`
	URL           string                 `json:"url,omitempty"`
	Filter        string                 `json:"filter,omitempty"`
	Protocol      string                 `json:"protocol,omitempty"` // default "tri"
	RegistryIDs   []string               `json:"registry-ids,omitempty"`
	Cluster       string                 `json:"cluster,omitempty"`     // default "failover"
	LoadBalance   string                 `json:"loadbalance,omitempty"` // random|roundrobin|consistenthashing|leastactive|xdsringhash|p2c; default "random"
	Group         string                 `json:"group,omitempty"`
	Version       string                 `json:"version,omitempty"`
	Serialization string                 `json:"serialization,omitempty"` // protobuf|hessian2|msgpack|jsonMapStruct
	Methods       map[string]DubboMethod `json:"methods,omitempty"`
	Async         bool                   `json:"async,omitempty"`
	Params        map[string]any         `json:"params,omitempty"`
	Generic       bool                   `json:"generic,omitempty"`
	Sticky        bool                   `json:"sticky,omitempty"`
	Timeout       string                 `json:"timeout,omitempty"`
	ForceTag      bool                   `json:"force.tag,omitempty"`
	TracingKey    string                 `json:"tracing-key,omitempty"`
}

// --- method (shared by services and references) ---

// DubboMethod is a per-method tuning entry. Keys are free-form method names
// validated against ^[_a-zA-Z][a-zA-Z\d_-]*$.
type DubboMethod struct {
	Name                        string  `json:"name,omitempty"`
	Retries                     float64 `json:"retries,omitempty"`     // default 2; schema type is "number"
	LoadBalance                 string  `json:"loadbalance,omitempty"` // random|roundrobin|consistenthashing|leastactive|xdsringhash|p2c; default "random"
	Weight                      int     `json:"weight,omitempty"`      // default 100
	TPSLimitInterval            int     `json:"tps.limit.interval,omitempty"`
	TPSLimitRate                int     `json:"tps.limit.rate,omitempty"`
	TPSLimitStrategy            string  `json:"tps.limit.strategy,omitempty"`             // threadSafeFixedWindow|slidingWindow|fixedWindow|default
	ExecuteLimit                float64 `json:"execute.limit,omitempty"`                  // schema type is "number"
	ExecuteLimitRejectedHandler string  `json:"execute.limit.rejected.handler,omitempty"` // default "log"
	Sticky                      bool    `json:"sticky,omitempty"`
	Timeout                     string  `json:"timeout,omitempty"`
}

// --- metrics ---

// DubboMetric is a single metrics entry. Keys are free-form logical IDs
// validated against ^[_a-zA-Z][a-zA-Z\d_-]*$.
type DubboMetric struct {
	Mode               string `json:"mode,omitempty"`      // default "pull"
	Namespace          string `json:"namespace,omitempty"` // default "dubbo"
	Enable             bool   `json:"enable,omitempty"`    // default true
	Port               int    `json:"port,omitempty"`      // default 9090
	Path               string `json:"path,omitempty"`      // default "/metrics"
	PushGatewayAddress string `json:"push-gateway-address,omitempty"`
}

// --- tracing ---

// DubboTracing is a single tracing entry. Keys are free-form logical IDs
// validated against ^[_a-zA-Z][a-zA-Z\d_-]*$.
type DubboTracing struct {
	Name        string `json:"name,omitempty"` // default "jaeger"
	ServiceName string `json:"serviceName,omitempty"`
	Address     string `json:"address,omitempty"`
	UseAgent    bool   `json:"use-agent,omitempty"` // default false
}

// --- shutdown ---

// DubboShutdown configures graceful shutdown behavior.
type DubboShutdown struct {
	Timeout                string `json:"timeout,omitempty"`                   // default "60s"
	StepTimeout            string `json:"step-timeout,omitempty"`              // default "3s"
	ConsumerUpdateWaitTime string `json:"consumer-update-wait-time,omitempty"` // default "3s"
	RejectHandler          string `json:"reject-handler,omitempty"`
	InternalSignal         bool   `json:"internal-signal,omitempty"` // default true
}
