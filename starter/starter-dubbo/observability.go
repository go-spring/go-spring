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

	"dubbo.apache.org/dubbo-go/v3"
	"dubbo.apache.org/dubbo-go/v3/client"
	"dubbo.apache.org/dubbo-go/v3/metrics"
	"dubbo.apache.org/dubbo-go/v3/otel/trace"
	"dubbo.apache.org/dubbo-go/v3/server"
	"go-spring.org/spring/gs"
)

func init() {
	// Observability is the single process-wide bean carrying application
	// metadata plus the built-in metrics + tracing. Building it bootstraps the
	// Prometheus exporter and the OTel tracer provider (both Instance-level in
	// dubbo-go), and both server and client derive their server.Server /
	// client.Client from it so they get observability for free. There is exactly
	// one such bean per process, matching dubbo-go's single-config-set model for
	// observability (metrics binds its HTTP port once; tracing sets a global
	// provider), so a second, divergent config would be silently ignored.
	gs.Provide(
		NewObservability,
		gs.IndexArg(0, gs.TagArg("${spring.dubbo.application}")),
		gs.IndexArg(1, gs.TagArg("${spring.dubbo.metrics}")),
		gs.IndexArg(2, gs.TagArg("${spring.dubbo.tracing}")),
		gs.IndexArg(3, gs.TagArg("?")), // optional: collect all InstanceOptioner beans
	)
}

// AppCfg holds the Dubbo application metadata under ${spring.dubbo.application}.
// Name is required: dubbo-go uses it as the application dimension on every
// metric and as the identity published to registries, so an unset name would
// silently produce meaningless observability data. The rest are optional and
// empty values fall back to dubbo-go's own defaults.
type AppCfg struct {
	Name         string `value:"${name:=}"`
	Organization string `value:"${organization:=}"`
	Module       string `value:"${module:=}"`
	Version      string `value:"${version:=}"`
	Owner        string `value:"${owner:=}"`
	Environment  string `value:"${environment:=}"`
}

// MetricsCfg configures the built-in Prometheus metrics under
// ${spring.dubbo.metrics}. Enabled by default (set enable=false to turn it
// off); when on it binds an HTTP endpoint on Port serving Path.
type MetricsCfg struct {
	Enable bool   `value:"${enable:=true}"`
	Port   int    `value:"${port:=9090}"`
	Path   string `value:"${path:=/metrics}"`
}

// TracingCfg configures the built-in OTel tracing under ${spring.dubbo.tracing}.
// Enabled by default with the stdout exporter so traces are visible with zero
// configuration; point Exporter at otlp-grpc/otlp-http/jaeger/zipkin plus an
// Endpoint for production. Ratio only matters when Mode is "ratio" (the
// dubbo-go default).
type TracingCfg struct {
	Enable     bool    `value:"${enable:=true}"`
	Exporter   string  `value:"${exporter:=stdout}"`
	Endpoint   string  `value:"${endpoint:=}"`
	Propagator string  `value:"${propagator:=w3c}"`
	Mode       string  `value:"${mode:=}"` // always|never|ratio; empty keeps dubbo-go default
	Ratio      float64 `value:"${ratio:=1.0}"`
}

// InstanceOptioner is the escape hatch for instance-level customization: provide
// one or more beans of this type and their options are appended last (highest
// priority) when building the shared instance, covering anything the typed
// config above does not expose.
type InstanceOptioner func() []dubbo.InstanceOption

// Observability is the process-wide dubbo observability component. It wraps the
// shared *dubbo.Instance that owns application metadata, metrics and tracing,
// and hands out server.Server / client.Client that inherit them. It is exposed
// as a semantic type (rather than a raw *dubbo.Instance) because its sole
// responsibility here is to be the single carrier of observability config.
type Observability struct {
	ins *dubbo.Instance
}

// NewServer builds a *server.Server from the shared instance so the server
// inherits the instance-level metrics and tracing. Caller options are layered
// on top and take priority.
func (o *Observability) NewServer(opts ...server.ServerOption) (*server.Server, error) {
	return o.ins.NewServer(opts...)
}

// NewClient builds a *client.Client from the shared instance so the client
// inherits the instance-level metrics and tracing. Caller options are layered
// on top and take priority.
func (o *Observability) NewClient(opts ...client.ClientOption) (*client.Client, error) {
	return o.ins.NewClient(opts...)
}

// NewObservability builds the shared *dubbo.Instance from application metadata
// and the observability config, wrapping it in an Observability. Empty/zero
// values are skipped so dubbo-go keeps its own defaults; user-provided
// customizers win over everything else.
func NewObservability(app AppCfg, m MetricsCfg, t TracingCfg, customizers []InstanceOptioner) (*Observability, error) {
	if app.Name == "" {
		return nil, errors.New("spring.dubbo.application.name is required")
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

	// Metrics: on unless explicitly disabled.
	if m.Enable {
		mopts := []metrics.Option{metrics.WithEnabled(), metrics.WithPrometheus()}
		if m.Port > 0 {
			mopts = append(mopts, metrics.WithPort(m.Port))
		}
		if m.Path != "" {
			mopts = append(mopts, metrics.WithPath(m.Path))
		}
		opts = append(opts, dubbo.WithMetrics(mopts...))
	}

	// Tracing: on unless explicitly disabled.
	if t.Enable {
		if t.Exporter != "stdout" && t.Endpoint == "" {
			return nil, errors.New("spring.dubbo.tracing.endpoint is required when exporter is not stdout")
		}
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
		topts = append(topts, trace.WithRatio(t.Ratio))
		opts = append(opts, dubbo.WithTracing(topts...))
	}

	for _, c := range customizers {
		opts = append(opts, c()...)
	}

	ins, err := dubbo.NewInstance(opts...)
	if err != nil {
		return nil, err
	}
	return &Observability{ins: ins}, nil
}
