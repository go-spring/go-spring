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
	"sync"

	"go-spring.org/cloud/actuator/endpoint"
	"go-spring.org/cloud/traffic"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/starter-otel/metric"
	metricotlp "go-spring.org/starter-otel/metric/otlp"
	metricprometheus "go-spring.org/starter-otel/metric/prometheus"
	metricstdout "go-spring.org/starter-otel/metric/stdout"
	"go-spring.org/starter-otel/trace"
	traceotlp "go-spring.org/starter-otel/trace/otlp"
	tracestdout "go-spring.org/starter-otel/trace/stdout"
	"go-spring.org/stdlib/flatten"
	runtimemetrics "go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
)

func init() {
	// Register the built-in trace and metric exporters with their driver
	// registries. Importing starter-otel is what makes otlp-grpc (the default),
	// otlp-http, prometheus and stdout available; the subpackages hold the
	// factories and expose a plain Register, so every registration the starter
	// performs is readable here rather than spread across their inits.
	//
	// An application that wants to slim its dependency footprint can register
	// its own exporter through the registry and drop the imports it does not
	// need.
	metricotlp.Register()
	metricprometheus.Register()
	metricstdout.Register()
	traceotlp.Register()
	tracestdout.Register()

	// The W3C propagator pair the trace registry resolves "w3c" to. A caller
	// that uses starter-otel/trace without linking this package (the luohua
	// umbrella, that package's own tests) calls it directly.
	trace.RegisterDefaults()

	// A nil condition means the module always runs when the starter is imported;
	// importing starter-otel activates the OTel SDK. The actual on/off is decided inside
	// setup from ${spring.observability.enable} (default true). This must be a
	// gs.Module, not a plain bean: its body executes during applyModules in the
	// RefreshPrepare phase, i.e. BEFORE any bean is instantiated. Setting the
	// OTel globals here therefore guarantees they are live before component beans
	// (e.g. a gorm client calling db.Use) are constructed. Building the providers
	// lazily inside a bean constructor would break that ordering.
	gs.Module(nil, setup)

	// The span processor that tags load-test spans is built together with the
	// tracer provider during the prepare phase, before any bean exists, so it
	// installs go-spring's default convention there; this hook swaps in the
	// container's bean before serving. Exported as a gs.Rooter so gs
	// instantiates it even though nothing autowires it.
	gs.Provide(newLoadTestHook, gs.IndexArg(0, gs.TagArg("?"))).
		Export(gs.As[gs.Rooter]()).Caller(1)
}

var (

	// runtimeOnce guards runtimemetrics.Start, which is not idempotent: the OTel
	// contrib runtime instrumentation registers fresh async callbacks on the
	// MeterProvider each call, so a second invocation (e.g. across gs.RunTest
	// re-runs against the same provider) would create duplicate instruments.
	// The first error is sticky so a later re-run does not silently lose the
	// original failure.
	runtimeOnce sync.Once
	runtimeErr  error
)

// loadTestHook is the marker bean whose construction installs the container's
// load-test convention into the span processor. A nil bean (no propagator
// provided) restores go-spring's default.
type loadTestHook struct{}

func newLoadTestHook(p traffic.Propagator) (*loadTestHook, error) {
	trace.SetLoadTestPropagator(p)
	return &loadTestHook{}, nil
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
		log.Info(context.Background(), log.TagAppDef, log.Msg("observability disabled; skipping OTel setup"))
		return nil
	}

	log.Debug(context.Background(), log.TagAppDef, func() []log.Field {
		return []log.Field{log.String("service_name", cfg.ServiceName), log.Bool("trace_enable", cfg.Trace.Enable), log.Bool("metrics_enable", cfg.Metrics.Enable), log.Msg("setting up OTel")}
	})

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

// setupTrace installs the global text-map propagator and, when tracing is
// enabled, the TracerProvider. The propagator is honored independently of
// tracing/exporting: context propagation (extract/inject of trace context and
// baggage on inbound/outbound requests) is useful on its own even when no
// spans are exported, so ${spring.observability.trace.propagator} is no
// longer ignored when trace.enable=false or exporter=none. The provider is a
// no-op when tracing is disabled or exporter is "none"; in that case the
// global TracerProvider stays the SDK no-op.
func setupTrace(cfg trace.TraceConfig, res *resource.Resource) error {
	// Propagator: always applied (independent of trace export).
	prop, err := trace.NewPropagator(cfg.Propagator)
	if err != nil {
		return err
	}
	if prop != nil {
		otel.SetTextMapPropagator(prop)
	}

	if !cfg.Enable || cfg.Exporter == "none" {
		if prop != nil {
			log.Info(context.Background(), log.TagAppDef,
				log.String("propagator", cfg.Propagator),
				log.Msg("trace export disabled; propagator still installed for context propagation"))
		}
		return nil
	}

	tp, err := trace.NewTracerProvider(cfg, res)
	if err != nil {
		return err
	}
	otel.SetTracerProvider(tp)
	// The provider is a process-global resource, so register it as a global
	// stopper (not a bean destroyer) to flush buffered spans at shutdown.
	gs.RegisterStopper("otel-trace", tp.Shutdown)

	log.Info(context.Background(), log.TagAppDef, log.String("exporter", cfg.Exporter), log.String("propagator", cfg.Propagator), log.Msg("init trace provider success"))
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
	// so there is no separate stop hook to manage. startRuntime is guarded so
	// a re-run of setup (e.g. across gs.RunTest) does not register duplicate
	// callbacks on an already-instrumented provider.
	if cfg.Runtime.Enable {
		opts := []runtimemetrics.Option{runtimemetrics.WithMeterProvider(mp)}
		if cfg.Runtime.MinReadMemStatsInterval > 0 {
			opts = append(opts, runtimemetrics.WithMinimumReadMemStatsInterval(cfg.Runtime.MinReadMemStatsInterval))
		}
		if err := startRuntime(opts); err != nil {
			return err
		}
		log.Info(context.Background(), log.TagAppDef, log.Msg("runtime metrics enabled"))
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
			r.Provide(&endpoint.Endpoint{Pattern: cfg.Path, Handler: ps.Handler})
		}
	}

	log.Info(context.Background(), log.TagAppDef, log.String("exporter", cfg.Exporter), log.Bool("runtime_metrics", cfg.Runtime.Enable), log.Msg("init metrics provider success"))
	return nil
}

// startRuntime starts the OTel Go-runtime metrics instrumentation exactly
// once per process. runtimemetrics.Start registers async callbacks on the
// MeterProvider and is not idempotent, so a second call (e.g. across gs.RunTest
// re-runs) would register duplicate instruments. The first call's outcome is
// sticky: a later re-run reuses it rather than silently masking the original
// error or re-registering callbacks.
func startRuntime(opts []runtimemetrics.Option) error {
	runtimeOnce.Do(func() {
		runtimeErr = runtimemetrics.Start(opts...)
	})
	return runtimeErr
}
