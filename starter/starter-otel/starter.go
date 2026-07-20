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
// otel.GetTracerProvider()/GetMeterProvider() is wired up automatically —
// configure once, no per-component adaptation.
package StarterOTel

import (
	"context"

	"net/http"

	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/spring/actuator/endpoint"
	"go-spring.org/stdlib/flatten"
	runtimemetrics "go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func init() {
	// A nil condition means the module always runs when the starter is imported;
	// importing starter-otel is the opt-in. The actual on/off is decided inside
	// setup from ${spring.observability.enable} (default true). This must be a
	// gs.Module, not a plain bean: its body executes during applyModules in the
	// RefreshPrepare phase, i.e. BEFORE any bean is instantiated. Setting the
	// OTel globals here therefore guarantees they are live before component beans
	// (e.g. a gorm client calling db.Use) are constructed. Building the providers
	// lazily inside a bean constructor would break that ordering.
	gs.Module(nil, setup)
}

// setup binds ${spring.observability}, eagerly builds the trace/metrics
// providers, installs them as OTel globals and registers them as beans with
// shutdown hooks. Returning early on Enable=false leaves the globals as the
// SDK's no-op providers, so an imported-but-disabled starter has no effect.
func setup(r gs.BeanProvider, p flatten.Storage) error {
	var cfg Config
	if err := conf.Bind(p, &cfg, "${spring.observability}"); err != nil {
		return err
	}
	if !cfg.Enable {
		return nil
	}

	res, err := newResource(cfg.ServiceName)
	if err != nil {
		return err
	}

	if cfg.Trace.Enable && cfg.Trace.Exporter != "none" {
		tp, err := newTracerProvider(cfg.Trace, res)
		if err != nil {
			return err
		}
		prop, err := newPropagator(cfg.Trace.Propagator)
		if err != nil {
			return err
		}
		otel.SetTracerProvider(tp)
		if prop != nil {
			otel.SetTextMapPropagator(prop)
		}
		r.Provide(tp).Destroy(func(tp *sdktrace.TracerProvider) error {
			return tp.Shutdown(context.Background())
		})
		// Correlate logs with traces: install the log hook that stamps every
		// record with the active span's trace_id/span_id.
		installLogCorrelation()
		// Propagate trace context on outbound requests (via the discovery
		// seam) so downstream services and mesh sidecars share one trace.
		installTracePropagation()
	}

	if cfg.Metrics.Enable && cfg.Metrics.Exporter != "none" {
		mp, ps, err := newMeterProvider(cfg.Metrics, res)
		if err != nil {
			return err
		}
		otel.SetMeterProvider(mp)
		r.Provide(mp).Destroy(func(mp *sdkmetric.MeterProvider) error {
			return mp.Shutdown(context.Background())
		})
		// Feed Go runtime metrics (GC, heap, goroutines, GOMAXPROCS, ...) into
		// the MeterProvider we just built. The instrumentation registers async
		// callbacks on this provider; they are torn down by mp.Shutdown above,
		// so there is no separate stop hook to manage.
		if cfg.Metrics.Runtime.Enable {
			opts := []runtimemetrics.Option{runtimemetrics.WithMeterProvider(mp)}
			if cfg.Metrics.Runtime.MinReadMemStatsInterval > 0 {
				opts = append(opts, runtimemetrics.WithMinimumReadMemStatsInterval(cfg.Metrics.Runtime.MinReadMemStatsInterval))
			}
			if err := runtimemetrics.Start(opts...); err != nil {
				return err
			}
		}
		// Pull-based (prometheus) exporter: contribute the scrape handler as an
		// endpoint.Endpoint so starter-actuator, if present, serves /metrics on
		// the shared management port — no cross-starter import. The dedicated
		// server (ps.server) is optional and only runs when metrics.port>0.
		if ps != nil {
			if ps.server != nil {
				r.Provide(ps.server).Destroy(func(srv *http.Server) error {
					return srv.Shutdown(context.Background())
				})
			}
			if ps.handler != nil {
				r.Provide(newMetricsEndpoint(cfg.Metrics.Path, ps.handler)).
					Export(gs.As[endpoint.Endpoint]())
			}
		}
	}

	return nil
}
