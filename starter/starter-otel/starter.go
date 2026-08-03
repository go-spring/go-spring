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

// Package StarterOTel defines go-spring's unified, framework-level
// observability layer. It builds the shared OTel TracerProvider and
// MeterProvider from ${spring.observability} and installs them as the process
// globals so any instrumented component (starter-gorm-*, ...) that reads
// otel.GetTracerProvider()/GetMeterProvider() is wired up automatically -
// configure once, no per-component adaptation.
package StarterOTel

import (
	"context"
	"net/http"

	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/experimental/actuator/endpoint"
	"go-spring.org/spring/gs"
	"go-spring.org/starter-otel/metric"
	"go-spring.org/starter-otel/trace"
	"go-spring.org/stdlib/flatten"
	runtimemetrics "go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
)

var (
	// starterTag identifies logs emitted by the otel starter.
	starterTag = log.RegisterInfraTag("starter_otel", "")
)

func init() {
	// A nil condition means the module always runs when the starter is imported;
	// importing starter-otel activates the OTel SDK. The actual on/off is decided inside
	// setup from ${spring.observability.enable} (default true). This must be a
	// gs.Module, not a plain bean: its body executes during applyModules in the
	// RefreshPrepare phase, i.e. BEFORE any bean is instantiated. Setting the
	// OTel globals here therefore guarantees they are live before component beans
	// (e.g. a gorm client calling db.Use) are constructed. Building the providers
	// lazily inside a bean constructor would break that ordering.
	gs.Module(nil, setup)
}

// setup binds ${spring.observability} and builds the shared trace/metrics
// resource, then delegates each pillar to setupTrace / setupMetrics. Returning
// early on Enable=false leaves the globals as the SDK's no-op providers, so an
// imported-but-disabled starter has no effect.
func setup(r gs.BeanProvider, p flatten.Storage) error {
	var cfg Config
	if err := conf.Bind(p, &cfg, "${spring.observability}"); err != nil {
		return err
	}
	if !cfg.Enable {
		log.Infof(context.Background(), starterTag, "observability disabled; skipping OTel setup")
		return nil
	}

	log.Debugf(context.Background(), starterTag, "setting up OTel with service=%s trace_enable=%v metrics_enable=%v", cfg.ServiceName, cfg.Trace.Enable, cfg.Metrics.Enable)

	res, err := trace.NewResource(cfg.ServiceName)
	if err != nil {
		return err
	}

	if err := setupTrace(cfg.Trace, res); err != nil {
		return err
	}
	if err := setupMetrics(r, cfg.Metrics, res); err != nil {
		return err
	}
	return nil
}

// setupTrace builds the TracerProvider and propagator from the trace config,
// installs them as OTel globals, registers the provider as a process-global
// stopper (flushed at shutdown via gs.RegisterStopper), and wires outbound
// trace-context propagation through the discovery seam. It is a no-op when
// tracing is disabled or exporter is "none".
func setupTrace(cfg trace.TraceConfig, res *resource.Resource) error {
	if !cfg.Enable || cfg.Exporter == "none" {
		return nil
	}

	tp, err := trace.NewTracerProvider(cfg, res)
	if err != nil {
		return err
	}
	prop, err := trace.NewPropagator(cfg.Propagator)
	if err != nil {
		return err
	}
	otel.SetTracerProvider(tp)
	if prop != nil {
		otel.SetTextMapPropagator(prop)
	}
	// The provider is a process-global resource, so register it as a global
	// stopper (not a bean destroyer) to flush buffered spans at shutdown.
	gs.RegisterStopper("otel-trace", tp.Shutdown)
	// Propagate trace context on outbound requests via the discovery seam.
	// The injector is backed by the global OTel propagator just installed
	// from ${spring.observability.trace.propagator} (W3C traceparent and/or
	// B3), so outbound requests carry the active trace context and a
	// downstream service - or mesh sidecar (Istio/Envoy) on the path -
	// joins the same trace instead of starting a new one. starter-otel owns
	// the propagator, so installing the injector here lights up outbound
	// propagation (via TraceRoundTripper) everywhere with no per-component
	// wiring.
	SetTraceInjector(func(ctx context.Context, header http.Header) {
		otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(header))
	})

	log.Infof(context.Background(), starterTag, "trace provider initialized exporter=%s propagator=%s", cfg.Exporter, cfg.Propagator)
	return nil
}

// setupMetrics builds the MeterProvider from the metrics config, installs it as
// the OTel global, registers it (and, for the pull-based Prometheus exporter, its
// dedicated scrape server) as process-global stoppers via gs.RegisterStopper,
// contributes the scrape handler as an actuator endpoint, and feeds Go runtime
// metrics into the provider when enabled. It is a no-op when metrics is disabled
// or exporter is "none".
func setupMetrics(r gs.BeanProvider, cfg metric.MetricsConfig, res *resource.Resource) error {
	if !cfg.Enable || cfg.Exporter == "none" {
		return nil
	}

	mp, ps, err := metric.NewMeterProvider(cfg, res)
	if err != nil {
		return err
	}
	otel.SetMeterProvider(mp)
	gs.RegisterStopper("otel-metrics", mp.Shutdown)
	// Feed Go runtime metrics (GC, heap, goroutines, GOMAXPROCS, ...) into
	// the MeterProvider we just built. The instrumentation registers async
	// callbacks on this provider; they are torn down by mp.Shutdown above,
	// so there is no separate stop hook to manage.
	if cfg.Runtime.Enable {
		opts := []runtimemetrics.Option{runtimemetrics.WithMeterProvider(mp)}
		if cfg.Runtime.MinReadMemStatsInterval > 0 {
			opts = append(opts, runtimemetrics.WithMinimumReadMemStatsInterval(cfg.Runtime.MinReadMemStatsInterval))
		}
		if err := runtimemetrics.Start(opts...); err != nil {
			return err
		}
	}
	// Pull-based (prometheus) exporter: contribute the scrape handler as an
	// endpoint.Endpoint so starter-actuator, if present, serves /metrics on
	// the shared management port - no cross-starter import. The dedicated
	// server (ps.Server) is optional and only runs when metrics.port>0.
	if ps != nil {
		if ps.Server != nil {
			gs.RegisterStopper("otel-metrics-scrape-server", ps.Server.Shutdown)
		}
		if ps.Handler != nil {
			r.Provide(metric.NewEndpoint(cfg.Path, ps.Handler)).
				Export(gs.As[endpoint.Endpoint]())
		}
	}

	log.Infof(context.Background(), starterTag, "metrics provider initialized exporter=%s runtime_metrics=%v", cfg.Exporter, cfg.Runtime.Enable)
	return nil
}
