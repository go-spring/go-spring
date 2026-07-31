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

// Package prometheus registers the pull-based Prometheus metric exporter with
// the metric meter-exporter registry. It is blank-imported by the top-level
// starter-otel package. Unlike the push-based otlp/stdout exporters, this one
// serves a scrape endpoint: the handler is contributed to the actuator as an
// endpoint.Endpoint (when metrics.port=0) or a dedicated server is started
// (when metrics.port>0).
package prometheus

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go-spring.org/starter-otel/metric"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

func init() {
	metric.RegisterMeterExporter("prometheus", newPrometheus)
}

// newPrometheus builds a pull-based Prometheus reader plus the scrape
// artifacts. The otelprom exporter is itself a Reader; the scrape handler
// renders the dedicated registry. When a positive port is configured a
// dedicated scrape server is started, otherwise the handler is served solely
// through the actuator management port by the starter.
func newPrometheus(cfg metric.MetricsConfig) (sdkmetric.Reader, *metric.PromServe, error) {
	reg := prometheus.NewRegistry()
	exp, err := otelprom.New(otelprom.WithRegisterer(reg))
	if err != nil {
		return nil, nil, err
	}
	ps := &metric.PromServe{Handler: promhttp.HandlerFor(reg, promhttp.HandlerOpts{})}
	if cfg.Port > 0 {
		ps.Server = serveMetrics(fmt.Sprintf(":%d", cfg.Port), cfg.Path, ps.Handler)
	}
	return exp, ps, nil
}

// serveMetrics starts a standalone HTTP server rendering the Prometheus scrape
// handler on path. It runs on its own listener (decoupled from any component's
// transport), mirroring the dedicated :9090 the dubbo/kratos examples expose.
// The same handler is also contributed to the actuator (see NewEndpoint), so
// this dedicated server is optional and skipped when port<=0.
func serveMetrics(addr, path string, handler http.Handler) *http.Server {
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			_ = err
		}
	}()
	return srv
}
