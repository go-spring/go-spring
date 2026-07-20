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

package StarterGateway

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"go-spring.org/spring/actuator/health"
	"go-spring.org/spring/starter"
)

// Metrics accumulates per-route request counters exposed as Prometheus text on
// the actuator management port. It is intentionally dependency-free (no
// prometheus client) so the gateway keeps a minimal footprint; a heavier OTel
// meter can be layered on later without changing the seam.
type Metrics struct {
	mu       sync.RWMutex
	routes   map[string]*routeMetric
	inFlight int64
	reloadErr int64
}

type routeMetric struct {
	c2xx, c3xx, c4xx, c5xx int64
}

func newMetrics() *Metrics {
	return &Metrics{routes: map[string]*routeMetric{}}
}

func (m *Metrics) metric(route string) *routeMetric {
	m.mu.RLock()
	rm, ok := m.routes[route]
	m.mu.RUnlock()
	if ok {
		return rm
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if rm, ok = m.routes[route]; ok {
		return rm
	}
	rm = &routeMetric{}
	m.routes[route] = rm
	return rm
}

func (m *Metrics) record(route string, status int) {
	rm := m.metric(route)
	switch {
	case status >= 500:
		atomic.AddInt64(&rm.c5xx, 1)
	case status >= 400:
		atomic.AddInt64(&rm.c4xx, 1)
	case status >= 300:
		atomic.AddInt64(&rm.c3xx, 1)
	default:
		atomic.AddInt64(&rm.c2xx, 1)
	}
}

func (m *Metrics) recordReloadError() { atomic.AddInt64(&m.reloadErr, 1) }

// instrument wraps a route's handler to stamp the route id into the context
// (for rate-limit keys and downstream correlation) and to count response status
// codes and in-flight requests.
func (t *RouteTable) instrument(id string, next http.Handler) http.Handler {
	m := t.metrics
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&m.inFlight, 1)
		defer atomic.AddInt64(&m.inFlight, -1)
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		r = r.WithContext(withRouteID(r.Context(), id))
		next.ServeHTTP(sw, r)
		m.record(id, sw.status)
	})
}

// metricsEndpoint contributes GET /gateway/metrics to the actuator management
// server (endpoint.Endpoint seam) rendering the counters in Prometheus text
// format. The path is distinct from the actuator's built-ins and from a
// separate otel /metrics so both can coexist.
type metricsEndpoint struct {
	m *Metrics
}

func newMetricsEndpoint(m *Metrics) *metricsEndpoint { return &metricsEndpoint{m: m} }

func (e *metricsEndpoint) Path() string { return "/gateway/metrics" }

func (e *metricsEndpoint) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m := e.m
	m.mu.RLock()
	ids := make([]string, 0, len(m.routes))
	for id := range m.routes {
		ids = append(ids, id)
	}
	m.mu.RUnlock()
	sort.Strings(ids)

	var b strings.Builder
	b.WriteString("# HELP gateway_requests_total Total gateway requests by route and status class.\n")
	b.WriteString("# TYPE gateway_requests_total counter\n")
	for _, id := range ids {
		rm := m.metric(id)
		line(&b, id, "2xx", atomic.LoadInt64(&rm.c2xx))
		line(&b, id, "3xx", atomic.LoadInt64(&rm.c3xx))
		line(&b, id, "4xx", atomic.LoadInt64(&rm.c4xx))
		line(&b, id, "5xx", atomic.LoadInt64(&rm.c5xx))
	}
	b.WriteString("# HELP gateway_in_flight_requests Requests currently being proxied.\n")
	b.WriteString("# TYPE gateway_in_flight_requests gauge\n")
	fmt.Fprintf(&b, "gateway_in_flight_requests %d\n", atomic.LoadInt64(&m.inFlight))
	b.WriteString("# HELP gateway_route_reload_errors_total Failed route table reloads.\n")
	b.WriteString("# TYPE gateway_route_reload_errors_total counter\n")
	fmt.Fprintf(&b, "gateway_route_reload_errors_total %d\n", atomic.LoadInt64(&m.reloadErr))

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = w.Write([]byte(b.String()))
}

func line(b *strings.Builder, route, class string, v int64) {
	fmt.Fprintf(b, "gateway_requests_total{route=%q,status=%q} %d\n", route, class, v)
}

// newGatewayHealth reports the gateway as a health.Indicator. It stays UP as
// long as the route table is loaded; a route whose lb:// upstream currently
// has zero live instances is a per-route concern surfaced via metrics/logs,
// not a reason to fail the whole gateway's readiness (it may still serve
// other routes).
func newGatewayHealth(tbl *RouteTable) health.Indicator {
	return starter.NewIndicator("gateway", func(ctx context.Context) error {
		if tbl.compiled.Load() == nil {
			return fmt.Errorf("gateway: route table not loaded")
		}
		return nil
	})
}
