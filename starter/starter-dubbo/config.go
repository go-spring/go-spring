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
	"dubbo.apache.org/dubbo-go/v3/metrics"
	"dubbo.apache.org/dubbo-go/v3/otel/trace"
	"dubbo.apache.org/dubbo-go/v3/registry"
	"dubbo.apache.org/dubbo-go/v3/server"
	"go-spring.org/spring/gs"

	// Side-effect import: installs the dubbo-go -> go-spring log bridge (see
	// internal/logger). The bridge self-installs via init(), so no symbols are
	// referenced here - importing this package is what redirects dubbo-go's
	// own logs into the application's go-spring log pipeline.
	_ "go-spring.org/starter-dubbo/internal/logger"
)

func init() {
	// The single top-level bean for the whole starter, carrying the shared
	// dubbo.Instance (application metadata, metrics, tracing) and the global
	// registries. Gated on ${spring.dubbo.registries}: no registry means no
	// *Instance bean, hence no server and no clients, so importing the starter
	// never forces an Instance on a project without one. dubbo-go allows only one
	// Instance per process, so a second bean would be silently ignored.
	gs.Provide(
		NewInstance,
		gs.IndexArg(0, gs.TagArg("${spring.dubbo}")),
	).Condition(gs.OnProperty("spring.dubbo.registries"))
}

// AppCfg holds Instance application metadata under ${spring.dubbo.application}.
// Name is required (falls back to ${spring.application.name}); it is dubbo-go's
// application dimension on metrics and its registry identity. The rest are
// optional and fall back to dubbo-go defaults.
type AppCfg struct {
	Name         string `value:"${name:=${spring.application.name}}"`
	Organization string `value:"${organization:=}"`
	Module       string `value:"${module:=}"`
	Version      string `value:"${version:=}"`
	Owner        string `value:"${owner:=}"`
	Environment  string `value:"${environment:=}"`
}

// MetricsCfg configures the built-in Prometheus metrics under
// ${spring.dubbo.metrics}. On by default; when on it serves Path on Port.
type MetricsCfg struct {
	Enable bool   `value:"${enable:=true}"`
	Port   int    `value:"${port:=9090}"`
	Path   string `value:"${path:=/metrics}"`
}

// TracingCfg configures the built-in OTel tracing under ${spring.dubbo.tracing}.
// On by default with the stdout exporter; set Exporter+Endpoint for production.
// Set Insecure for a plaintext (non-TLS) otlp collector. Ratio only applies when
// Mode is "ratio".
type TracingCfg struct {
	Enable     bool    `value:"${enable:=true}"`
	Exporter   string  `value:"${exporter:=stdout}"`
	Endpoint   string  `value:"${endpoint:=}"`
	Propagator string  `value:"${propagator:=w3c}"`
	Mode       string  `value:"${mode:=}"` // always|never|ratio; empty keeps dubbo-go default
	Ratio      float64 `value:"${ratio:=1.0}"`
	Insecure   bool    `value:"${insecure:=false}"`
}

// InstanceConfig is the single top-level config bound to ${spring.dubbo}. Besides
// the observability sub-configs (each keeping its own application/metrics/
// tracing prefix), it owns the global registries: the one place registries are
// ever defined, referenced elsewhere only by ID via registry-ids.
type InstanceConfig struct {
	Application AppCfg                 `value:"${application}"`
	Metrics     MetricsCfg             `value:"${metrics}"`
	Tracing     TracingCfg             `value:"${tracing}"`
	Registries  map[string]RegistryCfg `value:"${registries}"`
}

// Instance is the shared top-level facade over the process-wide *dubbo.Instance
// (application metadata, metrics, tracing) and the global registries. It hands
// out server.Server / client.Client that inherit the observability config; both
// roles depend on it, so its existence enables the rest of the starter.
type Instance struct {
	ins        *dubbo.Instance
	registries map[string]RegistryCfg
}

// Registries returns the global registries defined under
// ${spring.dubbo.registries}, the single source of truth roles select from.
func (d *Instance) Registries() map[string]RegistryCfg {
	return d.registries
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
// registries. Empty/zero observability values are skipped so dubbo-go keeps its
// own defaults. Registries are mandatory (the starter is registry-based service
// discovery only); per-entry addresses are enforced at bind time via the
// required ${...address} tag, so NewInstance only validates what binding cannot.
func NewInstance(cfg InstanceConfig) (*Instance, error) {
	app, m, t := cfg.Application, cfg.Metrics, cfg.Tracing
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

	if m.Enable {
		// WithPrometheusExporterEnabled is what actually starts the /metrics HTTP
		// endpoint; WithPrometheus alone only selects the protocol.
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
		opts = append(opts, dubbo.WithMetrics(mopts...))
	}

	if t.Enable {
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

	ins, err := dubbo.NewInstance(opts...)
	if err != nil {
		return nil, err
	}
	return &Instance{ins: ins, registries: cfg.Registries}, nil
}

// RegistryCfg configures a single service registry. The map key is a free-form
// logical ID (becomes registry.WithID), letting multiple registries of the same
// type coexist and be selected via RegistryIDs. The type comes from Protocol, or
// the map key when Protocol is empty. Empty/zero fields are skipped.
type RegistryCfg struct {
	Address    string            `value:"${address}"`    // required; must be set explicitly
	Protocol   string            `value:"${protocol:=}"` // registry type (etcdv3|nacos|zookeeper|...); defaults to the map key
	Namespace  string            `value:"${namespace:=}"`
	Group      string            `value:"${group:=}"`
	Username   string            `value:"${username:=}"`
	Password   string            `value:"${password:=}"`
	Timeout    time.Duration     `value:"${timeout:=}"`  // e.g. "5s"
	TTL        time.Duration     `value:"${ttl:=}"`      // e.g. "15m"
	Weight     int64             `value:"${weight:=-1}"` // negative means unset; 0 is a valid weight
	Zone       string            `value:"${zone:=}"`
	Simplified bool              `value:"${simplified:=false}"`
	Preferred  bool              `value:"${preferred:=false}"` // try this registry first
	Params     map[string]string `value:"${params:=}"`
}

// options translates a RegistryCfg into dubbo-go registry.Options. Shared by
// both server and client, since dubbo-go takes the same registry.Option on both
// sides. Empty/zero fields are skipped so dubbo-go keeps its own defaults.
func (rc RegistryCfg) options(id string) []registry.Option {
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
	if rc.Timeout > 0 {
		opts = append(opts, registry.WithTimeout(rc.Timeout))
	}
	if rc.TTL > 0 {
		opts = append(opts, registry.WithTTL(rc.TTL))
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
func selectRegistries(registries map[string]RegistryCfg, ids []string) (map[string]RegistryCfg, error) {
	if len(ids) == 0 {
		return registries, nil
	}
	selected := make(map[string]RegistryCfg, len(ids))
	for _, id := range ids {
		rc, ok := registries[id]
		if !ok {
			return nil, fmt.Errorf("dubbo: registry id %q is not defined under ${spring.dubbo.registries}", id)
		}
		selected[id] = rc
	}
	return selected, nil
}
